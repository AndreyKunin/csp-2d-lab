package ga

import (
	"crypto/rand"
	"csp2d/problem"
	"csp2d/util"
	"encoding/hex"
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
	sm.mu.Lock()
	job, exists := sm.jobs[id]
	sm.mu.Unlock()

	if !exists {
		return fmt.Errorf("simulation %s not found", id)
	}
	status := job.GetStatus()
	if status.Status == StatusPaused {
		return fmt.Errorf("simulation %s is already paused", id)
	}

	select {
	case job.PauseChan <- struct{}{}:
		<-job.PauseAckChan
	default:
		return fmt.Errorf("pause signal already sent for simulation %s", id)
	}

	return nil
}

func (sm *SimulationManagerImpl) ResumeSimulation(id string) error {
	sm.mu.Lock()
	job, exists := sm.jobs[id]
	sm.mu.Unlock()

	if !exists {
		return fmt.Errorf("simulation %s not found", id)
	}
	status := job.GetStatus()
	if status.Status == StatusRunning {
		return fmt.Errorf("simulation %s is already running", id)
	}

	select {
	case job.ResumeChan <- struct{}{}:
		<-job.ResumeAckChan
	default:
		return fmt.Errorf("resume signal already sent for simulation %s", id)
	}

	return nil
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

	select {
	case job.PauseChan <- struct{}{}:
	default:
	}

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
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("simulation panicked: %v", r)
			util.Warning.Println(err)
			job.UpdateJobStatus(StatusError, err)
		}
	}()

	err := Simulate(job)
	//TODO: these finished jobs need to be deleted eventually

	if err != nil {
		util.Warning.Println(err.Error())
		job.UpdateJobStatus(StatusError, err)
	} else {
		util.Info.Println("simulation completed")
		job.UpdateJobStatus(StatusCompleted, nil)
	}
}
