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
