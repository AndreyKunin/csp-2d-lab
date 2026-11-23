//go:build testhooks

package mvc_test

import (
	"csp2d/mvc"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func get(t *testing.T, e *echo.Echo, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

// UT-WEB-01: the pprof handlers are served at /debug/pprof/ only when
// webserver.pprof is true (BE-06); by default they are not reachable.
func TestPprofBehindConfigFlag(t *testing.T) {
	routes := func(enabled bool) *echo.Echo {
		var app mvc.WebApp
		app.Configuration.Server.Pprof = enabled
		e := echo.New()
		mvc.ConfigureRoutes(e, app)
		return e
	}

	off := routes(false)
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/cmdline"} {
		if code, _ := get(t, off, path); code != http.StatusNotFound {
			t.Errorf("pprof off: GET %s = %d, want 404", path, code)
		}
	}

	on := routes(true)
	cases := []struct{ path, want string }{
		{"/debug/pprof/", "Types of profiles available"},
		{"/debug/pprof/heap?debug=1", "heap profile"},
		{"/debug/pprof/goroutine?debug=1", "goroutine profile"},
		{"/debug/pprof/cmdline", ""},
		{"/debug/pprof/symbol", "num_symbols"},
	}
	for _, c := range cases {
		code, body := get(t, on, c.path)
		if code != http.StatusOK || !strings.Contains(body, c.want) {
			t.Errorf("pprof on: GET %s = %d, body %.80q; want 200 containing %q", c.path, code, body, c.want)
		}
	}
	if code, _ := get(t, on, "/debug/pprof/nope"); code != http.StatusNotFound {
		t.Errorf("pprof on: unknown profile = %d, want 404", code)
	}
}

// UT-CFG-08: the production configuration keeps pprof off.
func TestProductionPprofIsOff(t *testing.T) {
	if productionConfig(t).Server.Pprof {
		t.Fatal("application.yaml enables pprof")
	}
}

func do(t *testing.T, e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

// UT-WEB-02: a wrong HTTP method on an /api route answers 405 with an Allow
// header listing the methods whose routes match the path (BE-09); an unknown
// /api path answers 404; other paths still reach the static catch-all (GET /*,
// as e.Static in the web build and e.GET("/*") in the desktop build).
func TestAPIMethodMismatchIs405(t *testing.T) {
	e := echo.New()
	e.GET("/*", func(c echo.Context) error { return c.String(http.StatusOK, "static") })
	mvc.ConfigureRoutes(e, mvc.WebApp{})

	cases := []struct {
		method, path string
		code         int
		allow        string
	}{
		// DELETE /api/v1/simulation/:id matches "start" as a job id.
		{http.MethodGet, "/api/v1/simulation/start", http.StatusMethodNotAllowed, "POST, DELETE"},
		{http.MethodPut, "/api/v1/simulation/start", http.StatusMethodNotAllowed, "POST, DELETE"},
		{http.MethodPost, "/api/v1/simulation/pause/abc", http.StatusMethodNotAllowed, "GET"},
		{http.MethodGet, "/api/v1/simulation/configure/abc", http.StatusMethodNotAllowed, "POST"},
		{http.MethodPost, "/api/v1/simulation/abc", http.StatusMethodNotAllowed, "DELETE"},
		{http.MethodGet, "/api/v1/pdf/generate", http.StatusMethodNotAllowed, "POST"},
		{http.MethodPost, "/api/v1/service/parameter-definitions", http.StatusMethodNotAllowed, "GET"},
		{http.MethodGet, "/api/v1/nope", http.StatusNotFound, ""},
		{http.MethodPost, "/api/v1/nope", http.StatusNotFound, ""},
		{http.MethodGet, "/api", http.StatusOK, ""}, // not under /api/: the catch-all
	}
	for _, c := range cases {
		rec := do(t, e, c.method, c.path)
		if rec.Code != c.code || rec.Header().Get("Allow") != c.allow {
			t.Errorf("%s %s = %d, Allow %q; want %d, Allow %q (body %s)", c.method, c.path, rec.Code, rec.Header().Get("Allow"), c.code, c.allow, rec.Body)
		}
	}
	if rec := do(t, e, http.MethodGet, "/index.html"); rec.Code != http.StatusOK || rec.Body.String() != "static" {
		t.Errorf("GET /index.html = %d %q, want the catch-all", rec.Code, rec.Body)
	}
	if rec := do(t, e, http.MethodGet, "/api/v1/service/parameter-definitions"); rec.Code != http.StatusOK {
		t.Errorf("GET parameter-definitions = %d, want 200", rec.Code)
	}
}
