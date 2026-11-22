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
	"strings"

	"github.com/labstack/echo/v4"
)

func (app *WebApp) GetDefaultSimulationParameters(c echo.Context) error {
	parameters := app.Configuration.Defaults.SimulationParameters

	return c.JSON(http.StatusOK, parameters)
}

func (app *WebApp) StartSimulation(c echo.Context) error {
	var request Request

	if err := c.Bind(&request); err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if n := request.PartsCount(); n < MinPartsCount {
		message := fmt.Sprintf("at least %d parts are required, got %d", MinPartsCount, n)
		util.Warning.Println(message)
		return echo.NewHTTPError(http.StatusBadRequest, message)
	}

	inputs, err := request.BuildInputs()
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	constraints := request.BuildConstraints()
	parameters := request.BuildParameters(app.Configuration.Defaults.SimulationParameters, len(inputs))
	simulation := ga.NewSimulation(&parameters)
	problem := csp2d.NewCsp2dProblem(inputs, constraints)

	job, err := app.SimulationManager.StartSimulation(simulation, problem)
	if errors.Is(err, ga.ErrTooManyJobs) {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	}
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	job.Message = "Simulation started"
	util.Info.Println(job.Message)

	return c.JSON(http.StatusCreated, job)

}

func (app *WebApp) PauseSimulation(c echo.Context) error {
	id := c.Param("id")

	err := app.SimulationManager.PauseSimulation(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	job, err := app.SimulationManager.GetSimulationStatus(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	job.Message = "Simulation paused"
	util.Info.Println(job.Message)

	return c.JSON(http.StatusOK, job)
}

func (app *WebApp) ContinueSimulation(c echo.Context) error {
	id := c.Param("id")

	err := app.SimulationManager.ResumeSimulation(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	job, err := app.SimulationManager.GetSimulationStatus(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	job.Message = "Simulation resumed"
	util.Info.Println(job.Message)

	return c.JSON(http.StatusOK, job)
}

func (app *WebApp) DeleteSimulation(c echo.Context) error {
	id := c.Param("id")

	job, err := app.SimulationManager.DeleteSimulation(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	job.Message = "Simulation stopped and deleted"
	util.Info.Println(job.Message)

	return c.JSON(http.StatusOK, job)
}

func (app *WebApp) ConfigureSimulation(c echo.Context) error {
	var request Request

	if err := c.Bind(&request); err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id := c.Param("id")

	isInitiallyPaused := false
	if err := app.SimulationManager.PauseSimulation(id); err != nil {
		if strings.Contains(err.Error(), "already paused") {
			isInitiallyPaused = true
		} else {
			util.Warning.Println(err)
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	job, err := app.SimulationManager.GetSimulationStatus(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	parameters := request.BuildParameters(app.Configuration.Defaults.SimulationParameters, job.InputsLength)
	err = app.SimulationManager.ConfigureSimulation(id, &parameters)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	if !isInitiallyPaused {
		if err := app.SimulationManager.ResumeSimulation(id); err != nil {
			if !strings.Contains(err.Error(), "already running") {
				util.Warning.Println(err)
				return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
			}
		}
	}

	job, err = app.SimulationManager.GetSimulationStatus(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	job.Message = "Simulation configured"
	util.Info.Println(job.Message)

	return c.JSON(http.StatusOK, job)
}

func (app *WebApp) GetSimulationStatus(c echo.Context) error {
	id := c.Param("id")

	statistics, err := app.SimulationManager.GetSimulationStatistics(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, statistics)
}

func (app *WebApp) GetSimulationResult(c echo.Context) error {
	id := c.Param("id")

	// The snapshot published by the job goroutine; the live problem is never read here.
	snapshot, err := app.SimulationManager.GetSimulationResult(id)
	if err != nil {
		util.Warning.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	response, ok := snapshot.(*csp2d.Response)
	if !ok { // nothing published yet
		response = &csp2d.Response{}
	}

	if response.Solutions != nil && len(response.Solutions) > 0 {
		if responseJSON, err := json.Marshal(response.Solutions[0]); err == nil {
			util.Info.Printf("Best solution: %s", string(responseJSON))
		}
	}

	return c.JSON(http.StatusOK, response)
}

func (app *WebApp) GeneratePdf(c echo.Context) error {
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
