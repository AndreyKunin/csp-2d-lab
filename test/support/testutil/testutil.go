// Package testutil holds small helpers shared by the unit-test packages.
package testutil

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

// PendingEnv enables tests marked with Pending (the unit-test counterpart of
// the godog @pending tag): CSP2D_RUN_PENDING=1 go test -tags testhooks ./test/unit/...
const PendingEnv = "CSP2D_RUN_PENDING"

// Pending marks a test that describes required behaviour the implementation
// does not meet yet. It is skipped unless CSP2D_RUN_PENDING=1.
func Pending(t testing.TB, reason string) {
	t.Helper()
	if os.Getenv(PendingEnv) != "1" {
		t.Skipf("pending: %s (set %s=1 to run)", reason, PendingEnv)
	}
}

// Rand returns a deterministic generator and logs the seed so a failure can be
// reproduced.
func Rand(t testing.TB, seed int64) *rand.Rand {
	t.Helper()
	t.Logf("rand seed %d", seed)
	return rand.New(rand.NewSource(seed))
}

// Within runs fn and fails the test if it does not return within d (used to
// turn hangs/deadlocks into failures).
func Within(t testing.TB, d time.Duration, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s did not return within %s", what, d)
	}
}
