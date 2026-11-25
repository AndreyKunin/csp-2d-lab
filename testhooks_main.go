//go:build testhooks

// Test hook for the server binary: compiled ONLY with `-tags testhooks` (the
// functional-test harness uses it for coverage builds). When
// CSP2D_EXIT_ON_STDIN_EOF=1, the process exits cleanly via os.Exit(0) once its
// stdin is closed, so a -cover build flushes its coverage counters to
// GOCOVERDIR (a killed process writes none). Works on every OS.

package main

import (
	"io"
	"os"
)

func init() {
	if os.Getenv("CSP2D_EXIT_ON_STDIN_EOF") != "1" {
		return
	}
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}()
}
