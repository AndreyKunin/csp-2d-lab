//go:build !desktop

package main

import (
	"context"
	"csp2d/mvc"
	_ "net/http/pprof" // registers /debug/pprof/* on DefaultServeMux
	"runtime/debug"
)

func main() {
	debug.SetGCPercent(-1)          // GOGC=off
	debug.SetMemoryLimit(134217728) // 128 MB

	configuration := mvc.GetConfiguration("application.yaml")
	ctx := context.Background()
	mvc.StartWebApp(configuration, ctx)
}
