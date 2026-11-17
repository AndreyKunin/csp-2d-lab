package ga

import (
	"context"
	"crypto/rand"
	"csp2d/problem"
	"csp2d/util"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ErrTooManyJobs is returned by StartSimulation when MaxActiveJobs unfinished
// jobs already exist.
var ErrTooManyJobs = errors.New("too many active simulations")

// Limits configures job housekeeping. Zero values select the defaults.
type Limits struct {
	// MaxActiveJobs caps the registered jobs whose goroutine has not finished
	// (running or paused). 0 means runtime.GOMAXPROCS(0); negative means no limit.
	MaxActiveJobs int
	// IdleTimeout: a job that no API call has looked up for this long is
	// cancelled and removed, whatever its status. 0 means DefaultIdleTimeout;
	// negative disables the sweeper.
	IdleTimeout time.Duration
}

// DefaultIdleTimeout is used when Limits.IdleTimeout is 0.
const DefaultIdleTimeout = 10 * time.Minute

type SimulationManager interface {
	StartSimulation(simulation *Simulation, problem problem.Problem) (SimulationJobStatus, error)
	PauseSimulation(id string) error
	ResumeSimulation(id string) error
	GetSimulationStatus(id string) (SimulationJobStatus, error)
	GetSimulationStatistics(id string) (SimulationJobStatus, error)
	// GetSimulationResult returns the problem's last published result snapshot
	// (see ResultSnapshotter), or nil if none has been published yet.
	GetSimulationResult(id string) (any, error)
	DeleteSimulation(id string) (SimulationJobStatus, error)
	ConfigureSimulation(id string, parameters *Parameters) error
}

type SimulationManagerImpl struct {
	jobs map[string]*SimulationJob
	mu   sync.RWMutex

	maxActiveJobs int           // <= 0: unlimited
	idleTimeout   time.Duration // <= 0: never swept
}

// NewSimulationManager returns a manager without job limits or sweeping.
func NewSimulationManager() SimulationManager {
	return &SimulationManagerImpl{
		jobs: make(map[string]*SimulationJob),
	}
}

// NewSimulationManagerWithLimits returns a manager that refuses new jobs beyond
// limits.MaxActiveJobs and, until ctx is done, periodically removes jobs idle
// for longer than limits.IdleTimeout.
func NewSimulationManagerWithLimits(ctx context.Context, limits Limits) *SimulationManagerImpl {
	sm := &SimulationManagerImpl{
		jobs:          make(map[string]*SimulationJob),
		maxActiveJobs: limits.MaxActiveJobs,
		idleTimeout:   limits.IdleTimeout,
	}
	if sm.maxActiveJobs == 0 {
		sm.maxActiveJobs = runtime.GOMAXPROCS(0)
	}
	if sm.idleTimeout == 0 {
		sm.idleTimeout = DefaultIdleTimeout
	}
	if sm.idleTimeout > 0 {
		go sm.sweepLoop(ctx)
	}
	return sm
}

// sweepLoop runs Sweep every tenth of the idle timeout, but at most once a second.
func (sm *SimulationManagerImpl) sweepLoop(ctx context.Context) {
	ticker := time.NewTicker(max(sm.idleTimeout/10, time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sm.Sweep(now)
		}
	}
}

// Sweep cancels and removes every job that no API call has looked up since
// now - IdleTimeout, and returns how many it removed.
func (sm *SimulationManagerImpl) Sweep(now time.Time) int {
	if sm.idleTimeout <= 0 {
		return 0
	}
	deadline := now.Add(-sm.idleTimeout)
	sm.mu.Lock()
	defer sm.mu.Unlock()
	removed := 0
	for id, job := range sm.jobs {
		if job.lastAccessTime().Before(deadline) {
			job.cancel()
			delete(sm.jobs, id)
			removed++
			util.Info.Printf("Simulation %s (%s) removed: no client access for %s",
				id, job.GetStatus().Status, sm.idleTimeout)
		}
	}
	return removed
}

// lookup returns the job and records the access, which keeps it from being swept.
func (sm *SimulationManagerImpl) lookup(id string) (*SimulationJob, bool) {
	sm.mu.RLock()
	job, exists := sm.jobs[id]
	sm.mu.RUnlock()
	if exists {
		job.touch(time.Now())
	}
	return job, exists
}

// activeJobs counts registered jobs whose goroutine has not finished. The caller holds sm.mu.
func (sm *SimulationManagerImpl) activeJobs() int {
	n := 0
	for _, job := range sm.jobs {
		select {
		case <-job.finished:
		default:
			n++
		}
	}
	return n
}

func (sm *SimulationManagerImpl) StartSimulation(simulation *Simulation, problem problem.Problem) (SimulationJobStatus, error) {
	id, err := sm.generateSimulationID()
	if err != nil {
		return SimulationJobStatus{}, err
	}

	job := NewSimulationJob(id, simulation, problem)

	sm.mu.Lock()
	if sm.maxActiveJobs > 0 && sm.activeJobs() >= sm.maxActiveJobs {
		sm.mu.Unlock()
		return SimulationJobStatus{}, fmt.Errorf("%w (limit %d)", ErrTooManyJobs, sm.maxActiveJobs)
	}
	sm.jobs[id] = job
	sm.mu.Unlock()

	go sm.runSimulation(job)

	return job.GetStatus(), nil
}

func (sm *SimulationManagerImpl) PauseSimulation(id string) error {
	return sm.control(id, StatusRunning, func(job *SimulationJob) (chan struct{}, chan struct{}) {
		return job.PauseChan, job.PauseAckChan
	})
}

func (sm *SimulationManagerImpl) ResumeSimulation(id string) error {
	return sm.control(id, StatusPaused, func(job *SimulationJob) (chan struct{}, chan struct{}) {
		return job.ResumeChan, job.ResumeAckChan
	})
}

// control sends one control signal (pause or resume) to a job's goroutine and
// waits for its acknowledgement.
//
// Control calls on a job are serialised (controlMu), so the status check and the
// signal cannot interleave with another control call. The job must be in the
// required status; a finished or failed job is refused instead of waiting for an
// acknowledgement that would never come. If the job finishes or is deleted while
// the call waits, the call returns an error instead of blocking.
func (sm *SimulationManagerImpl) control(id string, required SimulationStatus,
	channels func(job *SimulationJob) (signal chan struct{}, ack chan struct{})) error {
	job, exists := sm.lookup(id)
	if !exists {
		return fmt.Errorf("simulation %s not found", id)
	}

	job.controlMu.Lock()
	defer job.controlMu.Unlock()

	if status := job.GetStatus().Status; status != required {
		switch status {
		case StatusPaused:
			return fmt.Errorf("simulation %s is already paused", id)
		case StatusRunning:
			return fmt.Errorf("simulation %s is already running", id)
		default:
			return fmt.Errorf("simulation %s is %s", id, status)
		}
	}

	signal, ack := channels(job)
	select {
	case signal <- struct{}{}:
	case <-job.finished:
		return fmt.Errorf("simulation %s is %s", id, job.GetStatus().Status)
	case <-job.done:
		return fmt.Errorf("simulation %s was deleted", id)
	}
	select {
	case <-ack:
		return nil
	case <-job.finished:
		return fmt.Errorf("simulation %s is %s", id, job.GetStatus().Status)
	case <-job.done:
		return fmt.Errorf("simulation %s was deleted", id)
	}
}

func (sm *SimulationManagerImpl) GetSimulationStatus(id string) (SimulationJobStatus, error) {
	job, exists := sm.lookup(id)
	if !exists {
		return SimulationJobStatus{}, fmt.Errorf("simulation %s not found", id)
	}

	return job.GetStatus(), nil
}

func (sm *SimulationManagerImpl) GetSimulationStatistics(id string) (SimulationJobStatus, error) {
	job, exists := sm.lookup(id)
	if !exists {
		return SimulationJobStatus{}, fmt.Errorf("simulation %s not found", id)
	}
	stats := job.GetStatus()
	stats.Statistics, stats.Problem, _ = job.snapshot()

	return stats, nil
}

func (sm *SimulationManagerImpl) GetSimulationResult(id string) (any, error) {
	job, exists := sm.lookup(id)
	if !exists {
		return nil, fmt.Errorf("simulation %s not found", id)
	}
	_, _, result := job.snapshot()
	return result, nil
}

func (sm *SimulationManagerImpl) DeleteSimulation(id string) (SimulationJobStatus, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	job, exists := sm.jobs[id]
	if !exists {
		return SimulationJobStatus{}, fmt.Errorf("simulation %s not found", id)
	}

	// Stop the job's goroutine (running or paused) so that it and the problem's
	// memory are released; a finished job ignores this.
	job.cancel()

	delete(sm.jobs, id)

	return job.GetStatus(), nil
}

func (sm *SimulationManagerImpl) ConfigureSimulation(id string, parameters *Parameters) error {
	job, exists := sm.lookup(id)
	if !exists {
		return fmt.Errorf("simulation %s not found", id)
	}

	err := job.UpdateSimulationParameters(parameters)
	if err != nil {
		return err
	}

	return nil
}

func (sm *SimulationManagerImpl) generateSimulationID() (string, error) {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		err := fmt.Errorf("failed to generate random ID: %v", err)
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func (sm *SimulationManagerImpl) runSimulation(job *SimulationJob) {
	// Registered first, so it runs last: the final status is set before
	// waiting control calls are released.
	defer close(job.finished)
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("simulation panicked: %v", r)
			util.Warning.Println(err)
			job.UpdateJobStatus(StatusError, err)
		}
	}()

	// A finished job stays registered until it is deleted or swept (Limits.IdleTimeout).
	err := Simulate(job)

	if errors.Is(err, ErrSimulationCancelled) {
		util.Info.Printf("Simulation %s cancelled (deleted)", job.id)
		return
	}
	if err != nil {
		util.Warning.Println(err.Error())
		job.UpdateJobStatus(StatusError, err)
	} else {
		util.Info.Println("simulation completed")
		job.UpdateJobStatus(StatusCompleted, nil)
	}
}
