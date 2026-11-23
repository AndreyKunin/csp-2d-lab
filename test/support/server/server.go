// Package server builds the csp2d binary and runs it as a separate process for
// black-box tests. Each Server gets its own temporary working directory holding
// application.yaml (copied from a profile) and a static/ directory, a free
// localhost port, and a captured log file.
package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// BuildOptions control how the binary under test is compiled.
type BuildOptions struct {
	Race   bool   // go build -race (requires cgo)
	Cover  bool   // go build -cover (write coverage to GOCOVERDIR at runtime)
	Tags   string // extra build tags
	OutDir string // where to place the binary; a temp dir when empty
}

// ModuleRoot walks up from the working directory to the directory holding go.mod.
func ModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above the working directory")
		}
		dir = parent
	}
}

// Build compiles the server (module root package) and returns the binary path.
func Build(opts BuildOptions) (string, error) {
	root, err := ModuleRoot()
	if err != nil {
		return "", err
	}
	outDir := opts.OutDir
	if outDir == "" {
		if outDir, err = os.MkdirTemp("", "csp2d-bin-"); err != nil {
			return "", err
		}
	}
	name := "csp2d"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(outDir, name)
	args := []string{"build", "-o", bin}
	if opts.Race {
		args = append(args, "-race")
	}
	if opts.Cover {
		args = append(args, "-cover", "-coverpkg=csp2d/...")
	}
	if opts.Tags != "" {
		args = append(args, "-tags", opts.Tags)
	}
	args = append(args, ".")
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	if opts.Race {
		cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return bin, nil
}

// Options describe one server instance.
type Options struct {
	Binary     string        // path returned by Build
	ConfigFile string        // profile YAML copied to <workdir>/application.yaml
	StaticDir  string        // optional directory to serve; a stub index.html is created when empty
	Env        []string      // extra environment (e.g. GOCOVERDIR=...)
	StartWait  time.Duration // heartbeat wait; 15 s when zero
	// GracefulStop: the binary was built with -tags testhooks; Stop closes its
	// stdin so it exits via os.Exit(0) (needed to flush -cover counters).
	GracefulStop bool
}

// Server is a running csp2d process.
type Server struct {
	URL     string
	Port    int
	WorkDir string
	LogPath string

	cmd      *exec.Cmd
	stdin    io.WriteCloser
	graceful bool
	logFile  *os.File
	done     chan struct{}
	exitErr  error
	mu       sync.Mutex
}

// Start launches the binary and waits until the health check answers 200.
func Start(opts Options) (*Server, error) {
	if opts.Binary == "" || opts.ConfigFile == "" {
		return nil, errors.New("server.Start: Binary and ConfigFile are required")
	}
	workDir, err := os.MkdirTemp("", "csp2d-srv-")
	if err != nil {
		return nil, err
	}
	if err := copyFile(opts.ConfigFile, filepath.Join(workDir, "application.yaml")); err != nil {
		return nil, err
	}
	if err := prepareStatic(workDir, opts.StaticDir); err != nil {
		return nil, err
	}
	port, err := FreePort()
	if err != nil {
		return nil, err
	}
	logPath := filepath.Join(workDir, "server.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(opts.Binary)
	cmd.Dir = workDir // the server reads application.yaml and static/ from its working directory
	cmd.Env = append(os.Environ(), "SERVER_HOST=127.0.0.1", fmt.Sprintf("SERVER_PORT=%d", port))
	cmd.Env = append(cmd.Env, opts.Env...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	var stdin io.WriteCloser
	if opts.GracefulStop {
		cmd.Env = append(cmd.Env, "CSP2D_EXIT_ON_STDIN_EOF=1")
		if stdin, err = cmd.StdinPipe(); err != nil {
			_ = logFile.Close()
			return nil, err
		}
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, err
	}

	s := &Server{
		URL:      fmt.Sprintf("http://127.0.0.1:%d", port),
		Port:     port,
		WorkDir:  workDir,
		LogPath:  logPath,
		cmd:      cmd,
		stdin:    stdin,
		graceful: opts.GracefulStop,
		logFile:  logFile,
		done:     make(chan struct{}),
	}
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		s.exitErr = err
		s.mu.Unlock()
		close(s.done)
	}()

	wait := opts.StartWait
	if wait == 0 {
		wait = 15 * time.Second
	}
	if err := s.waitReady(wait); err != nil {
		_ = s.Stop()
		return nil, fmt.Errorf("%w\n--- server log ---\n%s", err, s.LogTail(40))
	}
	return s, nil
}

func (s *Server) waitReady(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	for {
		select {
		case <-s.done:
			return fmt.Errorf("server exited during startup: %v", s.ExitError())
		case <-ctx.Done():
			return fmt.Errorf("server not ready within %s", timeout)
		default:
		}
		resp, err := client.Get(s.URL + "/api/service/v1/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Alive reports whether the process is still running.
func (s *Server) Alive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func (s *Server) ExitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitErr
}

// Pid returns the OS process id (for external resource observation).
func (s *Server) Pid() int { return s.cmd.Process.Pid }

// Stop ends the process (gracefully when built for it, otherwise by killing
// it) and waits for it to exit. The work directory (with the
// log) is kept so failures can be inspected; call Cleanup to remove it.
func (s *Server) Stop() error {
	if s.graceful && s.Alive() {
		_ = s.stdin.Close() // the testhooks build exits via os.Exit(0)
		select {
		case <-s.done:
		case <-time.After(5 * time.Second):
		}
	}
	if s.Alive() {
		_ = s.cmd.Process.Kill()
	}
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		return errors.New("server did not exit after kill")
	}
	return s.logFile.Close()
}

// Cleanup removes the work directory.
func (s *Server) Cleanup() error { return os.RemoveAll(s.WorkDir) }

// Log returns the whole captured log.
func (s *Server) Log() string {
	data, _ := os.ReadFile(s.LogPath)
	return string(data)
}

// LogTail returns the last n lines of the log.
func (s *Server) LogTail(n int) string {
	lines := strings.Split(strings.TrimRight(s.Log(), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// LogSize returns the current size of the log in bytes; use it with LogSince to
// examine only what a scenario produced.
func (s *Server) LogSize() int64 {
	fi, err := os.Stat(s.LogPath)
	if err != nil {
		return 0
	}
	return fi.Size()
}

// LogSince returns the log content written after offset.
func (s *Server) LogSince(offset int64) string {
	f, err := os.Open(s.LogPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return ""
	}
	data, _ := io.ReadAll(f)
	return string(data)
}

// FatalMarkers are log substrings that always indicate a defect.
var FatalMarkers = []string{"panic:", "simulation panicked", "WARNING: DATA RACE", "fatal error:"}

// FindFatal returns the log lines that contain any FatalMarkers.
func FindFatal(log string) []string {
	var hits []string
	sc := bufio.NewScanner(strings.NewReader(log))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		for _, m := range FatalMarkers {
			if strings.Contains(line, m) {
				hits = append(hits, line)
				break
			}
		}
	}
	return hits
}

// FreePort asks the OS for a free localhost TCP port.
func FreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func prepareStatic(workDir, staticDir string) error {
	target := filepath.Join(workDir, "static")
	if staticDir != "" {
		return copyDir(staticDir, target)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	stub := "<!doctype html><html><head><title>csp2d test stub</title></head><body>csp2d</body></html>"
	return os.WriteFile(filepath.Join(target, "index.html"), []byte(stub), 0o644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		return copyFile(path, out)
	})
}
