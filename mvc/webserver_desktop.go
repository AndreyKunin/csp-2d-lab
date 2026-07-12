//go:build desktop

package mvc

import (
	"context"
	"csp2d/ga"
	"csp2d/util"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type WebApp struct {
	Configuration     Configuration
	SimulationManager ga.SimulationManager
}

func StartWebAppEmbedded(configuration *Configuration, staticFiles fs.FS, ln net.Listener, port int, ctx context.Context) {
	webApp := WebApp{
		Configuration:     *configuration,
		SimulationManager: ga.NewSimulationManager(),
	}

	e := echo.New()
	e.HideBanner = true
	PrintBanner()

	e.Use(AddNoCacheHeaders())

	staticSubFS, err := fs.Sub(staticFiles, "static/static")
	if err != nil {
		util.Error.Fatal(err)
	}
	e.GET("/*", echo.WrapHandler(http.FileServer(http.FS(staticSubFS))))

	ConfigureRoutes(e, webApp)

	address := fmt.Sprintf("127.0.0.1:%d", port)
	util.Info.Println(fmt.Sprintf("Web server started at %s", address))

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := e.Shutdown(shutdownCtx); err != nil {
			util.Error.Println(err)
		}
	}()

	e.Listener = ln
	if err := e.Start(""); err != nil && !errors.Is(err, http.ErrServerClosed) {
		util.Error.Printf("Failed to start web server: %v", err)
		return
	}
	util.Info.Println("Web server stopped")
}
