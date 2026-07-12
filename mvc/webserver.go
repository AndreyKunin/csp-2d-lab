//go:build !desktop

package mvc

import (
	"context"
	"csp2d/ga"
	"csp2d/util"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
)

type WebApp struct {
	Configuration     Configuration
	SimulationManager ga.SimulationManager
}

func StartWebApp(configuration *Configuration, ctx context.Context) {
	webApp := WebApp{
		Configuration:     *configuration,
		SimulationManager: ga.NewSimulationManager(),
	}

	e := echo.New()
	e.HideBanner = true
	PrintBanner()

	e.Use(AddNoCacheHeaders())
	currentDir, _ := os.Getwd()
	staticPath := filepath.Join(currentDir, configuration.Server.StaticDir)
	e.Static("/", staticPath)

	ConfigureRoutes(e, webApp)

	host := GetEnv("SERVER_HOST", configuration.Server.Host)
	port := GetEnv("SERVER_PORT", configuration.Server.Port)

	address := fmt.Sprintf("%s:%s", host, port)
	util.Info.Println(fmt.Sprintf("Web server started at %s", address))

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := e.Shutdown(shutdownCtx)
		if err != nil {
			util.Error.Println(err)
		}
	}()

	if err := e.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
		util.Error.Printf("Failed to start web server: %v", err)
		return
	}
	util.Info.Println("Web server stopped")
}
