//go:build !desktop

package main

import (
	"context"
	"csp2d/mvc"
	"csp2d/util"
)

func main() {
	configuration := mvc.GetConfiguration("application.yaml")
	if _, err := mvc.ApplyMemorySettings(configuration.Memory); err != nil {
		util.Error.Fatal(err)
	}
	ctx := context.Background()
	mvc.StartWebApp(configuration, ctx)
}
