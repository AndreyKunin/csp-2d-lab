package mvc

import (
	"fmt"
	"net/http"

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
