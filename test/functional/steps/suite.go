// Package steps holds the godog step definitions and the suite/scenario
// lifecycle for the csp2d black-box functional tests. Steps interact with the
// server only over HTTP (see test/support/httpclient).
package steps

import (
	"csp2d/test/support/server"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cucumber/godog"
)

// Config is the suite configuration (set from command-line flags).
type Config struct {
	FixturesDir    string // test/functional/fixtures
	DefaultProfile string // server profile used when a scenario does not choose one
	Race           bool   // build the server with -race
	CoverDir       string // when set: build with -cover and write coverage here (GOCOVERDIR)
	StaticDir      string // optional real frontend bundle to serve
	KeepWorkDirs   bool   // keep server work dirs (config + logs) after the run
	RunTimeout     int    // seconds to wait for a job in statistical steps
}

// Suite owns the binary and the servers shared between scenarios.
type Suite struct {
	cfg Config

	binary   string
	buildErr error

	mu      sync.Mutex
	shared  map[string]*server.Server // profile -> server
	started []*server.Server          // every server started (for cleanup)
}

func NewSuite(cfg Config) *Suite {
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = "fast"
	}
	if cfg.RunTimeout == 0 {
		cfg.RunTimeout = 300
	}
	return &Suite{cfg: cfg, shared: map[string]*server.Server{}}
}

// InitializeTestSuite builds the binary once and stops all servers at the end.
func (s *Suite) InitializeTestSuite(ctx *godog.TestSuiteContext) {
	ctx.BeforeSuite(func() {
		opts := server.BuildOptions{Race: s.cfg.Race}
		if s.cfg.CoverDir != "" {
			// Coverage needs a clean exit: build the stdin-EOF exit hook in.
			opts.Cover, opts.Tags = true, "testhooks"
		}
		s.binary, s.buildErr = server.Build(opts)
		if s.buildErr != nil {
			fmt.Fprintf(os.Stderr, "server build failed: %v\n", s.buildErr)
		}
	})
	ctx.AfterSuite(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		var problems []string
		for _, srv := range s.started {
			if fatal := server.FindFatal(srv.Log()); len(fatal) > 0 {
				problems = append(problems, fmt.Sprintf("%s: %d fatal log lines", srv.LogPath, len(fatal)))
			}
			_ = srv.Stop()
			if !s.cfg.KeepWorkDirs {
				_ = srv.Cleanup()
			}
		}
		if len(problems) > 0 {
			fmt.Fprintf(os.Stderr, "server logs contain fatal markers (see scenario failures):\n  %s\n", strings.Join(problems, "\n  "))
		}
	})
}

// InitializeScenario creates a fresh World per scenario and registers steps.
func (s *Suite) InitializeScenario(ctx *godog.ScenarioContext) {
	w := &World{suite: s}
	ctx.Before(w.before)
	ctx.After(w.after)
	registerServerSteps(ctx, w)
	registerRequestSteps(ctx, w)
	registerLifecycleSteps(ctx, w)
	registerResultSteps(ctx, w)
	registerStatisticsSteps(ctx, w)
	registerPdfSteps(ctx, w)
	registerCatalogueSteps(ctx, w)
	registerMultiSteps(ctx, w)
}

func (s *Suite) configPath(profile string) (string, error) {
	p := filepath.Join(s.cfg.FixturesDir, "config", profile+".yaml")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("unknown server profile %q (%s): %w", profile, p, err)
	}
	return p, nil
}

func (s *Suite) startServer(profile string) (*server.Server, error) {
	if s.buildErr != nil {
		return nil, fmt.Errorf("server binary was not built: %w", s.buildErr)
	}
	cfgPath, err := s.configPath(profile)
	if err != nil {
		return nil, err
	}
	var env []string
	if s.cfg.CoverDir != "" {
		if err := os.MkdirAll(s.cfg.CoverDir, 0o755); err != nil {
			return nil, err
		}
		abs, _ := filepath.Abs(s.cfg.CoverDir)
		env = append(env, "GOCOVERDIR="+abs)
	}
	srv, err := server.Start(server.Options{
		Binary: s.binary, ConfigFile: cfgPath, StaticDir: s.cfg.StaticDir, Env: env,
		GracefulStop: s.cfg.CoverDir != "",
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.started = append(s.started, srv)
	s.mu.Unlock()
	return srv, nil
}

// sharedServer returns the long-lived server for a profile, (re)starting it
// when it is missing or has died.
func (s *Suite) sharedServer(profile string) (*server.Server, error) {
	s.mu.Lock()
	srv := s.shared[profile]
	s.mu.Unlock()
	if srv != nil && srv.Alive() {
		return srv, nil
	}
	srv, err := s.startServer(profile)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.shared[profile] = srv
	s.mu.Unlock()
	return srv, nil
}

// stopServer stops a scenario-owned server (not shared ones).
func (s *Suite) stopServer(srv *server.Server) {
	_ = srv.Stop()
}

// sortedKeys is a small helper for deterministic messages.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
