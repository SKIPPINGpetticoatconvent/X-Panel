package bootstrap

import (
	"context"
	"sync"

	"x-ui/logger"
)

type Status int

const (
	StatusStopped Status = iota
	StatusStarting
	StatusRunning
	StatusStopping
)

type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status() Status
}

type LifecycleManager struct {
	mu         sync.Mutex
	components []Component
}

func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		components: make([]Component, 0),
	}
}

func (m *LifecycleManager) Register(c Component) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components = append(m.components, c)
	logger.Infof("[Lifecycle] Registered component: %s", c.Name())
}

func (m *LifecycleManager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.components {
		logger.Infof("[Lifecycle] Starting component: %s", c.Name())
		if err := c.Start(ctx); err != nil {
			logger.Errorf("[Lifecycle] Failed to start component %s: %v", c.Name(), err)
			return err
		}
	}
	return nil
}

func (m *LifecycleManager) StopAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop in reverse order
	for i := len(m.components) - 1; i >= 0; i-- {
		c := m.components[i]
		logger.Infof("[Lifecycle] Stopping component: %s", c.Name())

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- c.Stop(ctx)
		}()

		select {
		case err := <-stopDone:
			if err != nil {
				logger.Errorf("[Lifecycle] Error stopping component %s: %v", c.Name(), err)
			}
		case <-ctx.Done():
			logger.Errorf("[Lifecycle] Timeout stopping component %s", c.Name())
		}
	}
}
