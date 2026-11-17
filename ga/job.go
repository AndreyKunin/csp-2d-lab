package ga

import (
	"csp2d/problem"
	"fmt"
	"sync"
	"time"
)

type Simulation struct {
	Parameters *Parameters `json:"parameters,omitempty"`
	State      *State      `json:"state,omitempty"`
}

func NewSimulation(parameters *Parameters) *Simulation {
	return &Simulation{
		Parameters: parameters,
	}
}

type SimulationStatus string

const (
	StatusRunning   SimulationStatus = "running"
	StatusPaused    SimulationStatus = "paused"
	StatusCompleted SimulationStatus = "completed"
	StatusError     SimulationStatus = "error"
)

type SimulationJob struct {
	id         string
	simulation *Simulation
	status     SimulationStatus
	error      error
	createdAt  time.Time
	updatedAt  time.Time

	PauseChan     chan struct{}
	PauseAckChan  chan struct{}
	ResumeChan    chan struct{}
	ResumeAckChan chan struct{}

	// done is closed (once) when the job is deleted. The GA loop returns as soon
	// as it observes it, running or paused, so the goroutine and the problem's
	// memory are released.
	done       chan struct{}
	cancelOnce sync.Once

	// finished is closed when the job's goroutine has exited (completed,
	// failed, panicked or cancelled).
	finished chan struct{}

	// controlMu serialises pause/resume calls on this job.
	controlMu sync.Mutex

	problem problem.Problem

	mu sync.RWMutex
}

// cancel asks the job's goroutine to stop. Safe to call more than once and after
// the job has finished.
func (job *SimulationJob) cancel() {
	job.cancelOnce.Do(func() { close(job.done) })
}

type ProblemInsights struct {
	BestFitnessVector []float64   `json:"bestFitnessVector"`
	SelectedSpecies   [][]float64 `json:"selectedSpecies"`
}

type SimulationJobStatus struct {
	ID           string           `json:"id"`
	Status       SimulationStatus `json:"status"`
	InputsLength int              `json:"-"`
	Error        *string          `json:"error,omitempty"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
	Message      string           `json:"message"`
	Statistics   *Simulation      `json:"statistics,omitempty"`
	Problem      *ProblemInsights `json:"problem,omitempty"`
}

func NewSimulationJob(id string, simulation *Simulation, problem problem.Problem) *SimulationJob {
	return &SimulationJob{
		id:         id,
		simulation: simulation,
		status:     StatusRunning,
		createdAt:  time.Now(),
		updatedAt:  time.Now(),

		PauseChan:     make(chan struct{}, 1),
		PauseAckChan:  make(chan struct{}, 1),
		ResumeChan:    make(chan struct{}, 1),
		ResumeAckChan: make(chan struct{}, 1),

		done:     make(chan struct{}),
		finished: make(chan struct{}),

		problem: problem,
	}
}

func (job *SimulationJob) GetStatus() SimulationJobStatus {
	job.mu.RLock()
	defer job.mu.RUnlock()

	status := SimulationJobStatus{
		ID:           job.id,
		Status:       job.status,
		InputsLength: job.problem.GetInputsSize(),
		CreatedAt:    job.createdAt.Format(time.RFC3339),
		UpdatedAt:    job.updatedAt.Format(time.RFC3339),
	}

	if job.error != nil {
		errStr := job.error.Error()
		status.Error = &errStr
	}

	return status
}

func (job *SimulationJob) UpdateJobStatus(status SimulationStatus, err error) {
	job.mu.Lock()
	defer job.mu.Unlock()

	job.status = status
	job.error = err
	job.updatedAt = time.Now()
}

func (job *SimulationJob) UpdateSimulationParameters(parameters *Parameters) error {
	job.mu.Lock()
	defer job.mu.Unlock()

	if job.status == StatusRunning {
		return fmt.Errorf("cannot update simulation parameters while simulation is running")
	}

	job.simulation.Parameters = parameters
	job.updatedAt = time.Now()

	return nil
}

func (job *SimulationJob) getSimulation() *Simulation {
	job.mu.Lock()
	defer job.mu.Unlock()

	return job.simulation
}

func (job *SimulationJob) getProblemStats() *ProblemInsights {
	job.mu.Lock()
	defer job.mu.Unlock()

	result := ProblemInsights{
		BestFitnessVector: nil,
		SelectedSpecies:   nil,
	}

	population := job.problem.GetPopulation()
	if population == nil {
		return &result
	}
	selectedSpecies := population.Selected
	var selectedVectors = make([][]float64, 0, len(selectedSpecies))
	var bestVector []float64 = nil
	for i, individual := range selectedSpecies {
		selectedVectors = append(selectedVectors, individual.GetFitnessVector())
		if i == 0 {
			bestVector = individual.GetFitnessVector()
		}
	}
	result.BestFitnessVector = bestVector
	result.SelectedSpecies = selectedVectors

	return &result
}
