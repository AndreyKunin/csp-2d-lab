//go:build desktop

package main

import (
	"context"
	"csp2d/mvc"
	"csp2d/util"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/getlantern/systray"
)

func run(port int, ln net.Listener, url string) {
	configuration := mvc.GetConfigurationFromBytes(embeddedConfig)
	if _, err := mvc.ApplyMemorySettings(configuration.Memory); err != nil {
		util.Error.Fatal(err)
	}
	ctx := context.Background()

	go mvc.StartWebAppEmbedded(configuration, embeddedStatic, ln, port, ctx)

	if waitForServer(port, 15*time.Second) {
		openBrowser(url)
	} else {
		fmt.Println("Server did not start in time")
	}
}

func main() {

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
