//go:build testhooks

package mvc_test

import (
	"csp2d/ga"
	"csp2d/mvc"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		{http.MethodPost, "/api/v1/simulation", http.StatusMethodNotAllowed, "PUT"},
		{http.MethodGet, "/api/v1/simulation", http.StatusMethodNotAllowed, "PUT"},
		{http.MethodPut, "/api/v1/simulation/abc", http.StatusMethodNotAllowed, "GET, POST, DELETE"},
		{http.MethodPatch, "/api/v1/simulation/abc", http.StatusMethodNotAllowed, "GET, POST, DELETE"},
		{http.MethodGet, "/api/v1/report", http.StatusMethodNotAllowed, "POST"},
		{http.MethodPost, "/api/service/v1/parameter-definitions", http.StatusMethodNotAllowed, "GET"},
		{http.MethodPost, "/api/service/v1/healthz", http.StatusMethodNotAllowed, "GET"},
		{http.MethodGet, "/api/service/healthz", http.StatusNotFound, ""}, // unversioned: no longer a route
		{http.MethodPost, "/api/service/healthz", http.StatusNotFound, ""},
		{http.MethodGet, "/api/v1/nope", http.StatusNotFound, ""},
		{http.MethodPost, "/api/v1/nope", http.StatusNotFound, ""},
		{http.MethodPost, "/api/service/nope", http.StatusNotFound, ""},
		{http.MethodPost, "/api/v2/simulation", http.StatusNotFound, ""},
		// the former routes are gone (POST .../start is an action on the id
		// "start": UT-WEB-03 checks it answers 404)
		{http.MethodPost, "/api/v1/pdf/generate", http.StatusNotFound, ""},
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
	for _, path := range []string{"/api/service/v1/parameter-definitions", "/api/service/v1/healthz"} {
		if rec := do(t, e, http.MethodGet, path); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
	}
}

// api is a test server with the production configuration and a real manager.
type api struct {
	t *testing.T
	e *echo.Echo
}

func newAPI(t *testing.T) *api {
	t.Helper()
	cfg := productionConfig(t)
	app := mvc.WebApp{Configuration: *cfg, SimulationManager: ga.NewSimulationManager()}
	e := echo.New()
	mvc.ConfigureRoutes(e, app)
	return &api{t, e}
}

func (a *api) call(method, path, body string) (int, map[string]any) {
	a.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	var decoded map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	return rec.Code, decoded
}

func (a *api) expect(method, path, body string, code int) map[string]any {
	a.t.Helper()
	got, decoded := a.call(method, path, body)
	if got != code {
		a.t.Fatalf("%s %s %s = %d (%v), want %d", method, path, body, got, decoded, code)
	}
	return decoded
}

func (a *api) start(body string) string {
	a.t.Helper()
	return a.expect(http.MethodPut, "/api/v1/simulation", body, http.StatusCreated)["id"].(string)
}

func (a *api) waitStatus(id, status string) {
	a.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		_, st := a.call(http.MethodGet, "/api/v1/simulation/"+id, "")
		if st["status"] == status {
			return
		}
		if time.Now().After(deadline) {
			a.t.Fatalf("status %v, want %s", st["status"], status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

const (
	runningJob = `{"sheets":{"width":100,"length":100},"parts":[{"label":"1","width":10,"length":20,"count":6},{"label":"2","width":30,"length":15,"count":6}],
	               "parameters":{"maxEpochsCount":100000000,"cycleCount":1000000,"windowSize":250}}`
	shortJob = `{"sheets":{"width":100,"length":100},"parts":[{"label":"1","width":10,"length":20,"count":6}],"parameters":{"maxEpochsCount":3}}`
	// A part larger than the sheet: the job fails (status "error").
	failingJob = `{"sheets":{"width":100,"length":100},"parts":[{"label":"1","width":200,"length":20,"count":4}]}`
)

// UT-WEB-03: the simulation resource answers 404 for an unknown simulation,
// 400 for a parsing or validation error, 409 for a call the simulation's
// status does not allow, 500 when the simulation failed, and 2xx otherwise.
func TestSimulationResourceStatuses(t *testing.T) {
	a := newAPI(t)
	const unknown = "/api/v1/simulation/0123456789abcdef"

	// start
	a.expect(http.MethodPut, "/api/v1/simulation", `{"parts":`, http.StatusBadRequest)
	a.expect(http.MethodPut, "/api/v1/simulation", `{"parts":[{"label":"1","width":10,"length":20,"count":3}]}`, http.StatusBadRequest)
	id := a.start(runningJob)
	path := "/api/v1/simulation/" + id

	// unknown simulation
	a.expect(http.MethodGet, unknown, "", http.StatusNotFound)
	a.expect(http.MethodGet, unknown+"?format=result", "", http.StatusNotFound)
	a.expect(http.MethodPost, unknown, `{"action":"pause"}`, http.StatusNotFound)
	a.expect(http.MethodPost, unknown, `{"action":"configure","parameters":{"mutationRate":0.1}}`, http.StatusNotFound)
	a.expect(http.MethodPost, unknown, `{}`, http.StatusNotFound)         // the simulation first, then the body
	a.expect(http.MethodPost, unknown, `{"action":`, http.StatusNotFound) // the simulation first, then the body
	a.expect(http.MethodPost, "/api/v1/simulation/start", "", http.StatusNotFound)
	a.expect(http.MethodDelete, unknown, "", http.StatusNotFound)

	// parsing and validation
	a.expect(http.MethodPost, path, `{"action":`, http.StatusBadRequest)
	if m := a.expect(http.MethodPost, path, `{}`, http.StatusBadRequest); !strings.Contains(m["message"].(string), "missing action") {
		t.Errorf("missing action: %v", m)
	}
	if m := a.expect(http.MethodPost, path, `{"action":"stop"}`, http.StatusBadRequest); !strings.Contains(m["message"].(string), `unknown action "stop"`) {
		t.Errorf("unknown action: %v", m)
	}
	a.expect(http.MethodPost, path, `{"action":"configure","parameters":{"eliteCount":0}}`, http.StatusBadRequest)
	a.expect(http.MethodPost, path, `{"action":"configure","parameters":{"eliteCount":1.5}}`, http.StatusBadRequest)
	if m := a.expect(http.MethodGet, path+"?format=xml", "", http.StatusBadRequest); !strings.Contains(m["message"].(string), `unknown format "xml"`) {
		t.Errorf("unknown format: %v", m)
	}

	// status and result
	if st := a.expect(http.MethodGet, path, "", http.StatusOK); st["status"] != "running" || st["statistics"] == nil {
		t.Errorf("status (default format): %v", st)
	}
	a.expect(http.MethodGet, path+"?format=status", "", http.StatusOK)
	a.expect(http.MethodGet, path+"?format=result", "", http.StatusOK)

	// actions and their state conflicts
	if st := a.expect(http.MethodPost, path, `{"action":"pause"}`, http.StatusOK); st["status"] != "paused" || st["message"] != "Simulation paused" {
		t.Errorf("pause: %v", st)
	}
	a.expect(http.MethodPost, path, `{"action":"pause"}`, http.StatusConflict)
	if st := a.expect(http.MethodPost, path, `{"action":"configure","parameters":{"mutationRate":0.2}}`, http.StatusOK); st["status"] != "paused" || st["message"] != "Simulation configured" {
		t.Errorf("configure while paused: %v", st)
	}
	if st := a.expect(http.MethodPost, path, `{"action":"continue"}`, http.StatusOK); st["status"] != "running" || st["message"] != "Simulation resumed" {
		t.Errorf("continue: %v", st)
	}
	a.expect(http.MethodPost, path, `{"action":"continue"}`, http.StatusConflict)
	if st := a.expect(http.MethodPost, path, `{"action":"configure","parameters":{"mutationRate":0.1}}`, http.StatusOK); st["status"] != "running" {
		t.Errorf("configure while running: %v", st)
	}
	if st := a.expect(http.MethodDelete, path, "", http.StatusOK); st["message"] != "Simulation stopped and deleted" {
		t.Errorf("delete: %v", st)
	}
	a.expect(http.MethodGet, path, "", http.StatusNotFound)

	// a completed simulation: its status and result, no control
	done := a.start(shortJob)
	a.waitStatus(done, "completed")
	donePath := "/api/v1/simulation/" + done
	a.expect(http.MethodGet, donePath+"?format=result", "", http.StatusOK)
	a.expect(http.MethodPost, donePath, `{"action":"pause"}`, http.StatusConflict)
	a.expect(http.MethodPost, donePath, `{"action":"continue"}`, http.StatusConflict)
	a.expect(http.MethodPost, donePath, `{"action":"configure","parameters":{"mutationRate":0.1}}`, http.StatusConflict)
	a.expect(http.MethodDelete, donePath, "", http.StatusOK)

	// a failed simulation: its status reports the error; no result, no control
	failed := a.start(failingJob)
	a.waitStatus(failed, "error")
	failedPath := "/api/v1/simulation/" + failed
	if st := a.expect(http.MethodGet, failedPath, "", http.StatusOK); st["error"] == nil {
		t.Errorf("failed status without error: %v", st)
	}
	if m := a.expect(http.MethodGet, failedPath+"?format=result", "", http.StatusInternalServerError); !strings.Contains(m["message"].(string), "failed") {
		t.Errorf("failed result: %v", m)
	}
	a.expect(http.MethodPost, failedPath, `{"action":"pause"}`, http.StatusInternalServerError)
	a.expect(http.MethodPost, failedPath, `{"action":"configure","parameters":{"mutationRate":0.1}}`, http.StatusInternalServerError)
	a.expect(http.MethodDelete, failedPath, "", http.StatusOK)
}
