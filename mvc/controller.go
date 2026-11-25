package mvc

import (
	"csp2d/ga"
	"csp2d/problem/csp2d"
	"csp2d/util"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Health handles GET /api/service/v1/healthz: the server is up.
func (app *WebApp) Health(c echo.Context) error {
	return c.JSON(http.StatusOK, struct{}{})
}

// GetParameterDefinitions returns the default value and the validation rule of
// every parameter by key (application.yaml): the UI shows the defaults and
// validates with the server's rules.
func (app *WebApp) GetParameterDefinitions(c echo.Context) error {
	return c.JSON(http.StatusOK, app.Configuration.Defaults.Parameters.Definitions())
}

// StartSimulation handles PUT /api/v1/simulation: validates the request (400),
// creates the problem and starts a job (201; 429 when too many jobs run).
func (app *WebApp) StartSimulation(c echo.Context) error {
	var request Request

	if err := c.Bind(&request); err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := request.Validate(app.Configuration.Defaults.Parameters, app.Configuration.Inputs.MaxPartsCount); err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	inputs, err := request.BuildInputs()
	if err != nil { // Validate checked the labels: not expected
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	constraints := request.BuildConstraints()
	parameters := request.BuildParameters(app.Configuration.Defaults.SimulationParameters, len(inputs))
	if err := ValidateBuiltParameters(parameters); err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	simulation := ga.NewSimulation(&parameters)
	problem := csp2d.NewCsp2dProblem(inputs, constraints)

	job, err := app.SimulationManager.StartSimulation(simulation, problem)
	if err != nil {
		return httpError(err)
	}
	job.Message = "Simulation started"
	util.Info.Println(job.Message)

	return c.JSON(http.StatusCreated, job)

}

// Simulation actions (POST /api/v1/simulation/:id, field "action").
const (
	ActionPause     = "pause"
	ActionContinue  = "continue"
	ActionConfigure = "configure"
)

// ActionRequest is the body of POST /api/v1/simulation/:id. Parameters is used
// by the configure action only.
type ActionRequest struct {
	Action     string                `json:"action"`
	Parameters *SimulationParameters `json:"parameters"`
}

// Formats of GET /api/v1/simulation/:id (query parameter "format").
const (
	FormatStatus = "status" // the default
	FormatResult = "result"
)

// httpError maps an error of the simulation manager to its HTTP status:
// 404 unknown (or deleted) simulation, 409 a call the simulation's status does
// not allow, 429 too many jobs, 500 anything else.
func httpError(err error) *echo.HTTPError {
	util.Warning.Println(err)
	switch {
	case errors.Is(err, ga.ErrSimulationNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, ga.ErrInvalidState):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, ga.ErrTooManyJobs):
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}

func badRequest(err error) *echo.HTTPError {
	util.Warning.Println(err)
	return echo.NewHTTPError(http.StatusBadRequest, err.Error())
}

// failedSimulation returns a 500 error when the job failed (status "error"):
// the simulation can neither be controlled nor give a result. nil otherwise.
func failedSimulation(job ga.SimulationJobStatus) *echo.HTTPError {
	if job.Status != ga.StatusError {
		return nil
	}
	message := fmt.Sprintf("simulation %s failed", job.ID)
	if job.Error != nil {
		message += ": " + *job.Error
	}
	util.Warning.Println(message)
	return echo.NewHTTPError(http.StatusInternalServerError, message)
}

// SimulationAction handles POST /api/v1/simulation/:id: pause, continue or
// configure, by the "action" field of the body.
//
// Checks in order: the simulation exists (404), the body parses and names a
// known action (400), the simulation has not failed (500), its status allows
// the action (409).
func (app *WebApp) SimulationAction(c echo.Context) error {
	id := c.Param("id")
	job, err := app.SimulationManager.GetSimulationStatistics(id)
	if err != nil {
		return httpError(err)
	}
	var request ActionRequest
	if err := c.Bind(&request); err != nil {
		return badRequest(err)
	}
	switch request.Action {
	case ActionPause, ActionContinue, ActionConfigure:
	case "":
		return badRequest(errors.New(`missing action: "pause", "continue" or "configure"`))
	default:
		return badRequest(fmt.Errorf(`unknown action %q: "pause", "continue" or "configure"`, request.Action))
	}
	if failed := failedSimulation(job); failed != nil {
		return failed
	}

	var message string
	switch request.Action {
	case ActionPause:
		err, message = app.SimulationManager.PauseSimulation(id), "Simulation paused"
	case ActionContinue:
		err, message = app.SimulationManager.ResumeSimulation(id), "Simulation resumed"
	case ActionConfigure:
		err, message = app.configure(id, job, request.Parameters), "Simulation configured"
	}
	if err != nil {
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			return httpErr
		}
		return httpError(err)
	}

	status, err := app.SimulationManager.GetSimulationStatus(id)
	if err != nil {
		return httpError(err)
	}
	status.Message = message
	util.Info.Println(message)
	return c.JSON(http.StatusOK, status)
}

// configure validates the parameters (400 before the job is touched), pauses
// the job if it is running, replaces its parameters and resumes it if it was
// running.
func (app *WebApp) configure(id string, job ga.SimulationJobStatus, requested *SimulationParameters) error {
	if err := app.Configuration.Defaults.Parameters.Validate(requested); err != nil {
		return badRequest(err)
	}
	request := Request{Parameters: requested}
	parameters := request.BuildParameters(app.Configuration.Defaults.SimulationParameters, job.InputsLength)
	if job.Statistics != nil && job.Statistics.Parameters != nil {
		// The job keeps its population size (BE-01; UpdateSimulationParameters).
		parameters.PopulationSize = job.Statistics.Parameters.PopulationSize
	}
	if err := ValidateBuiltParameters(parameters); err != nil {
		return badRequest(err)
	}

	isInitiallyPaused := job.Status == ga.StatusPaused
	if !isInitiallyPaused {
		if err := app.SimulationManager.PauseSimulation(id); err != nil {
			return err
		}
	}
	if err := app.SimulationManager.ConfigureSimulation(id, &parameters); err != nil {
		return err
	}
	if !isInitiallyPaused {
		if err := app.SimulationManager.ResumeSimulation(id); err != nil {
			return err
		}
	}
	return nil
}

// GetSimulation handles GET /api/v1/simulation/:id?format=status|result: the
// status with the statistics (the default), or the result. A failed simulation
// reports its status (status "error"), but has no result (500).
func (app *WebApp) GetSimulation(c echo.Context) error {
	id := c.Param("id")
	format := c.QueryParam("format")
	if format == "" {
		format = FormatStatus
	}
	if format != FormatStatus && format != FormatResult {
		return badRequest(fmt.Errorf(`unknown format %q: "status" or "result"`, format))
	}
	statistics, err := app.SimulationManager.GetSimulationStatistics(id)
	if err != nil {
		return httpError(err)
	}
	if format == FormatStatus {
		return c.JSON(http.StatusOK, statistics)
	}
	if failed := failedSimulation(statistics); failed != nil {
		return failed
	}

	// The snapshot published by the job goroutine; the live problem is never read here.
	snapshot, err := app.SimulationManager.GetSimulationResult(id)
	if err != nil {
		return httpError(err)
	}
	response, ok := snapshot.(*csp2d.Response)
	if !ok { // nothing published yet
		response = &csp2d.Response{}
	}
	if len(response.Solutions) > 0 {
		if responseJSON, err := json.Marshal(response.Solutions[0]); err == nil {
			util.Info.Printf("Best solution: %s", string(responseJSON))
		}
	}
	return c.JSON(http.StatusOK, response)
}

// DeleteSimulation handles DELETE /api/v1/simulation/:id: stops the job and
// removes it, whatever its status.
func (app *WebApp) DeleteSimulation(c echo.Context) error {
	job, err := app.SimulationManager.DeleteSimulation(c.Param("id"))
	if err != nil {
		return httpError(err)
	}
	job.Message = "Simulation stopped and deleted"
	util.Info.Println(job.Message)
	return c.JSON(http.StatusOK, job)
}

// GenerateReport handles POST /api/v1/report: a PDF report of the layout
// (multipart form: the start request as "request", the sheet images as "images").
func (app *WebApp) GenerateReport(c echo.Context) error {
	// Limit total request size to ~32 MB to protect the server.
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, 32<<20)

	form, err := c.MultipartForm()
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
	}

	// --- Parse JSON request payload --------------------------------------------------
	var request Request
	if values, ok := form.Value["request"]; ok && len(values) > 0 {
		if err := json.Unmarshal([]byte(values[0]), &request); err != nil {
			util.Warning.Println(err)
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request JSON: "+err.Error())
		}
	} else {
		return echo.NewHTTPError(http.StatusBadRequest, "missing 'request' form field")
	}

	// --- Read images -----------------------------------------------------------------
	fileHeaders := form.File["images"]
	images := make([][]byte, 0, len(fileHeaders))
	for _, fh := range fileHeaders {
		f, err := fh.Open()
		if err != nil {
			util.Warning.Println(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "cannot open uploaded image: "+err.Error())
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			util.Warning.Println(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "cannot read uploaded image: "+err.Error())
		}
		images = append(images, data)
	}

	// --- Build PDF -------------------------------------------------------------------
	pdfBytes, err := util.BuildPdf(request.ToPdfInput(), images)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate PDF: "+err.Error())
	}

	// Allow inline viewing in browsers that have a PDF viewer; otherwise the user
	// will be offered to download it. The filename is just a hint.
	c.Response().Header().Set("Content-Disposition", `inline; filename="csp2d-report.pdf"`)
	return c.Blob(http.StatusOK, "application/pdf", pdfBytes)
}
