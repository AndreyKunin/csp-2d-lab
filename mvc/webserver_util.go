package mvc

import (
	"csp2d/util"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"

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

// ConfigureRoutes registers the API. Every route is under /api and versioned:
//
//	GET    /api/service/v1/healthz                  the server is up
//	GET    /api/service/v1/parameter-definitions    defaults and rules
//	PUT    /api/v1/simulation                       start a simulation (201)
//	POST   /api/v1/simulation/:id                   {"action": "pause"|"continue"|"configure", "parameters": {...}}
//	GET    /api/v1/simulation/:id?format=status|result
//	DELETE /api/v1/simulation/:id                   stop and remove
//	POST   /api/v1/report                           the PDF report
func ConfigureRoutes(e *echo.Echo, webApp WebApp) {
	service := e.Group("/api/service", AddJsonHeader(), AddNoCacheHeaders())
	serviceV1 := service.Group("/v1")
	serviceV1.GET("/healthz", webApp.Health)
	serviceV1.GET("/parameter-definitions", webApp.GetParameterDefinitions)

	apiV1 := e.Group("/api/v1")
	// The report is a PDF: no JSON content type.
	apiV1.POST("/report", webApp.GenerateReport, AddNoCacheHeaders())
	simulation := apiV1.Group("/simulation", AddJsonHeader(), AddNoCacheHeaders())
	simulation.PUT("", webApp.StartSimulation)
	simulation.POST("/:id", webApp.SimulationAction)
	simulation.GET("/:id", webApp.GetSimulation)
	simulation.DELETE("/:id", webApp.DeleteSimulation)

	// A group with middleware registers catch-all "route not found" routes
	// (<prefix> and <prefix>/*) that answer 404, also for a path that exists
	// with another method. Replace them, and catch the rest of /api, so a wrong
	// method answers 405 and an unknown path 404 (BE-09).
	for _, g := range []*echo.Group{service, serviceV1, apiV1, simulation} {
		g.RouteNotFound("", apiFallback(e))
		g.RouteNotFound("/*", apiFallback(e))
	}
	e.RouteNotFound("/api/*", apiFallback(e))

	if webApp.Configuration.Server.Pprof {
		mountPprof(e)
	}
}

// apiFallback answers an /api request that no route of its method matches:
// 405 with an Allow header listing the methods of the /api routes whose
// pattern matches the path, or 404 when none does.
func apiFallback(e *echo.Echo) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Request().URL.Path
		var allowed []string
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			for _, r := range e.Routes() {
				if r.Method == method && strings.HasPrefix(r.Path, "/api/") && routeMatches(r.Path, path) {
					allowed = append(allowed, method)
					break
				}
			}
		}
		if len(allowed) == 0 {
			return echo.ErrNotFound
		}
		c.Response().Header().Set(echo.HeaderAllow, strings.Join(allowed, ", "))
		return echo.ErrMethodNotAllowed
	}
}

// routeMatches reports whether path matches an Echo route pattern: a :param
// matches one non-empty segment, * matches the rest, other segments match
// literally.
func routeMatches(pattern, path string) bool {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range patternParts {
		if p == "*" {
			return true
		}
		if i >= len(pathParts) {
			return false
		}
		if strings.HasPrefix(p, ":") {
			if pathParts[i] == "" {
				return false
			}
			continue
		}
		if p != pathParts[i] {
			return false
		}
	}
	return len(patternParts) == len(pathParts)
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
