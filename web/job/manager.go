package job

import (
	"sync"

	"x-ui/logger"
)

type Manager struct {
	jobs []Job
	mu   sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		jobs: make([]Job, 0),
	}
}

func (m *Manager) Register(job Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
	logger.Infof("Registered job: %s", job.Name())
}

func (m *Manager) StartAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logger.Info("Starting all background jobs...")
	for _, job := range m.jobs {
		logger.Infof("Starting job: %s", job.Name())
		if err := job.Start(); err != nil {
			logger.Errorf("Failed to start job %s: %v", job.Name(), err)
		}
	}
}

func (m *Manager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logger.Info("Stopping all background jobs...")
	var wg sync.WaitGroup
	for _, j := range m.jobs {
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			logger.Infof("Stopping job: %s", job.Name())
			if err := job.Stop(); err != nil {
				logger.Errorf("Failed to stop job %s: %v", job.Name(), err)
			} else {
				logger.Infof("Job %s stopped", job.Name())
			}
		}(j)
	}
	wg.Wait()
	logger.Info("All background jobs stopped")
}
