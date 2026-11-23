package mvc

import (
	"csp2d/util"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v4"
)

func PrintBanner() {
	customBanner := `
 ╔══════════════════════════════════════╗
 ║       CSP 2d Problem simulator       ║
 ║            Version 1.0               ║
 ╚══════════════════════════════════════╝
    `
	fmt.Println(customBanner)
}

func ConfigureRoutes(e *echo.Echo, webApp WebApp) {
	e.GET("/heartbeat", func(c echo.Context) error { return c.JSON(http.StatusOK, struct{}{}) })

	apiV1 := e.Group("/api/v1")
	apiV1.Use(AddJsonHeader(), AddNoCacheHeaders())
	service := apiV1.Group("/service")
	service.GET("/parameter-definitions", webApp.GetParameterDefinitions)
	simulation := apiV1.Group("/simulation")
	simulation.POST("/start", webApp.StartSimulation)
	simulation.GET("/pause/:id", webApp.PauseSimulation)
	simulation.GET("/continue/:id", webApp.ContinueSimulation)
	simulation.POST("/configure/:id", webApp.ConfigureSimulation)
	simulation.GET("/status/:id", webApp.GetSimulationStatus)
	simulation.GET("/result/:id", webApp.GetSimulationResult)
	simulation.DELETE("/:id", webApp.DeleteSimulation)

	pdfGroup := e.Group("/api/v1/pdf")
	pdfGroup.Use(AddNoCacheHeaders())
	pdfGroup.POST("/generate", webApp.GeneratePdf)

	if webApp.Configuration.Server.Pprof {
		mountPprof(e)
	}
}

// mountPprof serves the net/http/pprof handlers at /debug/pprof/ (BE-06):
// the index, the named profiles (/debug/pprof/heap, goroutine, allocs, ...;
// pprof.Index serves them by the last path element), and the CPU profile,
// trace, command line and symbol lookup.
func mountPprof(e *echo.Echo) {
	g := e.Group("/debug/pprof")
	g.GET("/", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	g.GET("/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
	g.GET("/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
	g.GET("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.POST("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
	g.GET("/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
	g.GET("/:name", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	util.Info.Println("pprof is served at /debug/pprof/")
}

func AddNoCacheHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Response().Header().Set("Pragma", "no-cache")
			c.Response().Header().Set("Expires", "0")

			return next(c)
		}
	}
}

func AddJsonHeader() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Content-Type", "application/json; charset=utf-8")
			return next(c)
		}
	}
}
