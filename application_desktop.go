//go:build desktop

package main

import (
	"context"
	"csp2d/mvc"
	"csp2d/util"
	"fmt"
	"net"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/getlantern/systray"
)

func run(port int, ln net.Listener, url string) {
	configuration := mvc.GetConfigurationFromBytes(embeddedConfig)
	ctx := context.Background()

	go mvc.StartWebAppEmbedded(configuration, embeddedStatic, ln, port, ctx)

	if waitForServer(port, 15*time.Second) {
		openBrowser(url)
	} else {
		fmt.Println("Server did not start in time")
	}
}

func main() {
	debug.SetGCPercent(-1)
	debug.SetMemoryLimit(134217728)

	port, ln, err := findFreePort(8080)
	if err != nil {
		fmt.Println("Could not find a free port:", err)
		return
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Starting server at %s\n", url)

	if IsLaunchedFromTerminal() {
		if runtime.GOOS == "windows" {
			AttachParentConsole()
			util.ReinitLoggers()
		}
		run(port, ln, url)
		select {}
	} else {
		systray.Run(func() {
			systray.SetIcon(generateIcon())
			systray.SetTitle("csp2d")
			systray.SetTooltip("csp2d is running at " + url)

			mOpen := systray.AddMenuItem("Open in Browser", "Open the app in browser")
			systray.AddSeparator()
			mQuit := systray.AddMenuItem("Quit", "Stop the server and exit")

			run(port, ln, url)

			go func() {
				for {
					select {
					case <-mOpen.ClickedCh:
						openBrowser(url)
					case <-mQuit.ClickedCh:
						systray.Quit()
					}
				}
			}()
		}, func() {
			os.Exit(0)
		})
	}
}
