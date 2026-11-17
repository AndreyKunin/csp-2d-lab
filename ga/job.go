package ga

import (
	"csp2d/problem"
	"csp2d/util"
	"fmt"
	"sync"
	"sync/atomic"
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

	// lastAccess is the time (UnixNano) of the last API lookup of this job. The
	// manager's sweeper removes jobs whose last access is too old.
	lastAccess atomic.Int64

	problem problem.Problem

	// The snapshot HTTP readers see, guarded by mu. The job goroutine publishes it
	// at the end of every epoch (publish). Readers never touch the live
	// simulation, population or problem, which only the job goroutine writes.
	published *Simulation      // copies of Parameters and State
	insights  *ProblemInsights // copies of the elites' fitness vectors
	result    any              // ResultSnapshotter output, or nil

	mu sync.RWMutex
}

// ResultSnapshotter is implemented by problems that can describe their current
// elites as an immutable value safe to serialise on another goroutine. Simulate
// calls it on the job goroutine at the end of every epoch.
type ResultSnapshotter interface {
	SnapshotResult() (any, error)
}

// Releaser is implemented by problems that hold memory worth freeing as soon
// as the job's goroutine has ended (arena buffers, the population). The manager
// calls Release exactly once, after Simulate returns, fails, panics or is
// cancelled; the published snapshot stays available.
type Releaser interface {
	Release()
}

func (job *SimulationJob) touch(now time.Time) { job.lastAccess.Store(now.UnixNano()) }

func (job *SimulationJob) lastAccessTime() time.Time { return time.Unix(0, job.lastAccess.Load()) }

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
	job := &SimulationJob{
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
	job.touch(job.createdAt)
	job.published = copySimulation(simulation)
	job.insights = &ProblemInsights{}
	return job
}

// copySimulation returns a copy of the parameters and state that shares no
// memory with simulation (the window's epoch list, which is not serialised, is
// left out).
func copySimulation(simulation *Simulation) *Simulation {
	if simulation == nil {
		return &Simulation{}
	}
	c := &Simulation{}
	if simulation.Parameters != nil {
		p := *simulation.Parameters
		c.Parameters = &p
	}
	if simulation.State != nil {
		st := *simulation.State
		st.CurrentWindow.Epochs = nil
		c.State = &st
	}
	return c
}

// publish replaces the snapshot. It must run on the job goroutine, between
// epochs, while nothing else writes the simulation or the population.
func (job *SimulationJob) publish(simulation *Simulation, population *problem.Population) {
	published := copySimulation(simulation)
	insights := &ProblemInsights{}
	if population != nil {
		insights.SelectedSpecies = make([][]float64, 0, len(population.Selected))
		for i, individual := range population.Selected {
			vector := append([]float64(nil), individual.GetFitnessVector()...)
			insights.SelectedSpecies = append(insights.SelectedSpecies, vector)
			if i == 0 {
				insights.BestFitnessVector = vector
			}
		}
	}
	var result any
	snapshotter, hasResult := job.problem.(ResultSnapshotter)
	hasResult = hasResult && population != nil // no elites before the first population
	if hasResult {
		r, err := snapshotter.SnapshotResult()
		if err != nil {
			util.Warning.Printf("[job=%s] result snapshot failed: %v", job.id, err)
			hasResult = false
		}
		result = r
	}

	job.mu.Lock()
	defer job.mu.Unlock()
	job.published = published
	job.insights = insights
	if hasResult {
		job.result = result
	}
}

// snapshot returns the last published statistics. The values are never
// modified after publication, so callers may read them without a lock.
func (job *SimulationJob) snapshot() (*Simulation, *ProblemInsights, any) {
	job.mu.RLock()
	defer job.mu.RUnlock()
	return job.published, job.insights, job.result
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
	// The job is paused, so no epoch will publish the new parameters until it is
	// resumed: publish them now, keeping the published state.
	p := *parameters
	job.published = &Simulation{Parameters: &p, State: job.published.State}

	return nil
}

func (job *SimulationJob) getSimulation() *Simulation {
	job.mu.Lock()
	defer job.mu.Unlock()

	return job.simulation
}
