// Package functional runs the godog black-box suite against a real csp2d
// server process. The harness builds the server binary once, starts it on a
// free localhost port in a temporary work directory (application.yaml copied
// from fixtures/config/<profile>.yaml, plus a static/ directory), talks to it
// only over HTTP with a timeout on every call, and fails a scenario when the
// server log lines it produced contain a panic, data race or fatal error.
//
// Tags: @pending (required behaviour not implemented yet; excluded from the
// gate), @slow, @quality (statistical solution quality), @resource (process
// memory/CPU), @expects-panic (suppresses the log check). Each scenario is also
// tagged with its ID (@FT-<AREA>-NN). The framework, tags and ID scheme are
// described in docs/dev/design.md §16.
//
//	go test ./test/functional/                                      # PR profile
//	go test ./test/functional/ -args --godog.tags='@pending'        # pending set
//	go test ./test/functional/ -args --server.race                  # race-built server (cgo)
//
// godog flags use pflag syntax (double dash): --godog.tags, --godog.format,
// --godog.paths, --godog.random, ... Scenarios run sequentially (the steps
// attribute server log lines to the running scenario).
package functional

import (
	"csp2d/test/functional/steps"
	"flag"
	"os"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/godog/colors"
	"github.com/spf13/pflag"
)

// DefaultTags is the PR-gate selection.
const DefaultTags = "~@pending && ~@slow && ~@quality && ~@resource"

var (
	opts = godog.Options{
		Output: colors.Colored(os.Stdout),
		Format: "pretty",
		Paths:  []string{"features"},
		Tags:   DefaultTags,
		Strict: true,
	}

	race         = flag.Bool("server.race", false, "build the server under test with -race (requires cgo)")
	coverDir     = flag.String("server.coverdir", "", "build the server with -cover and write coverage data here")
	staticDir    = flag.String("server.static", "", "serve this frontend bundle instead of a stub index.html")
	keepWorkDirs = flag.Bool("server.keep", false, "keep server work directories (config, logs) after the run")
	profile      = flag.String("server.profile", "fast", "default server config profile (fixtures/config/<name>.yaml)")
	runTimeout   = flag.Int("server.run-timeout", 300, "seconds to wait for a job in statistical steps")
)

func TestMain(m *testing.M) {
	// Bind here (not in init) so that pflag also sees the go test flags,
	// which are registered by testing.Init before TestMain runs.
	godog.BindCommandLineFlags("godog.", &opts)
	pflag.Parse()
	if args := pflag.Args(); len(args) > 0 {
		opts.Paths = args
	}
	os.Exit(m.Run())
}

func TestFeatures(t *testing.T) {
	suite := steps.NewSuite(steps.Config{
		FixturesDir:    "fixtures",
		DefaultProfile: *profile,
		Race:           *race,
		CoverDir:       *coverDir,
		StaticDir:      *staticDir,
		KeepWorkDirs:   *keepWorkDirs,
		RunTimeout:     *runTimeout,
	})
	o := opts
	o.TestingT = t
	status := godog.TestSuite{
		Name:                 "csp2d",
		TestSuiteInitializer: suite.InitializeTestSuite,
		ScenarioInitializer:  suite.InitializeScenario,
		Options:              &o,
	}.Run()
	if status != 0 {
		t.Fatalf("godog suite failed with status %d", status)
	}
}
