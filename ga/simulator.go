package ga

import (
	"crypto/rand"
	"csp2d/problem"
	"csp2d/util"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

type SimulationManager interface {
	StartSimulation(simulation *Simulation, problem problem.Problem) (SimulationJobStatus, error)
	PauseSimulation(id string) error
	ResumeSimulation(id string) error
	GetSimulationStatus(id string) (SimulationJobStatus, error)
	GetSimulationStatistics(id string) (SimulationJobStatus, error)
	GetSimulationProblem(id string) (problem.Problem, error)
	DeleteSimulation(id string) (SimulationJobStatus, error)
	ConfigureSimulation(id string, parameters *Parameters) error
}

type SimulationManagerImpl struct {
	jobs map[string]*SimulationJob
	mu   sync.RWMutex
}

func NewSimulationManager() SimulationManager {
	return &SimulationManagerImpl{
		jobs: make(map[string]*SimulationJob),
	}
}

func (sm *SimulationManagerImpl) StartSimulation(simulation *Simulation, problem problem.Problem) (SimulationJobStatus, error) {
	id, err := sm.generateSimulationID()
	if err != nil {
		return SimulationJobStatus{}, err
	}

	job := NewSimulationJob(id, simulation, problem)

	sm.mu.Lock()
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
	sm.mu.Lock()
	job, exists := sm.jobs[id]
	sm.mu.Unlock()

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
	sm.mu.Lock()
	defer sm.mu.Unlock()

	job, exists := sm.jobs[id]
	if !exists {
		return SimulationJobStatus{}, fmt.Errorf("simulation %s not found", id)
	}

	return job.GetStatus(), nil
}

func (sm *SimulationManagerImpl) GetSimulationStatistics(id string) (SimulationJobStatus, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	job, exists := sm.jobs[id]
	if !exists {
		return SimulationJobStatus{}, fmt.Errorf("simulation %s not found", id)
	}
	stats := job.GetStatus()
	stats.Statistics = job.getSimulation()
	stats.Problem = job.getProblemStats()

	return stats, nil
}

func (sm *SimulationManagerImpl) GetSimulationProblem(id string) (problem.Problem, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	job, exists := sm.jobs[id]
	if !exists {
		return nil, fmt.Errorf("simulation %s not found", id)
	}

	return job.problem, nil
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
	sm.mu.Lock()
	job, exists := sm.jobs[id]
	sm.mu.Unlock()

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

	err := Simulate(job)
	//TODO: these finished jobs need to be deleted eventually

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
