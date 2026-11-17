package server

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// RSS returns the resident memory of the server process in bytes, observed
// from outside the process (Linux: /proc, macOS: ps, Windows: tasklist).
func (s *Server) RSS() (int64, error) {
	pid := s.Pid()
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			return 0, err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "VmRSS:") {
				return parseKB(strings.TrimPrefix(sc.Text(), "VmRSS:"))
			}
		}
		return 0, fmt.Errorf("VmRSS not found")
	case "windows":
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return 0, err
		}
		// "csp2d.exe","1234","Console","1","123,456 K"
		fields := strings.Split(strings.TrimSpace(string(out)), `","`)
		if len(fields) < 5 {
			return 0, fmt.Errorf("unexpected tasklist output %q", out)
		}
		return parseKB(strings.Trim(fields[4], `"`))
	default:
		out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
		if err != nil {
			return 0, err
		}
		return parseKB(string(out))
	}
}

// parseKB parses "123,456 K" / " 123456 kB" / "123456" (kilobytes) into bytes.
func parseKB(s string) (int64, error) {
	var digits strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return 0, fmt.Errorf("no number in %q", s)
	}
	kb, err := strconv.ParseInt(digits.String(), 10, 64)
	return kb * 1024, err
}
