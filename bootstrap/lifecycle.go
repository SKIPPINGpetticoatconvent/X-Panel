package bootstrap

import (
	"context"
	"sync"
	"time"

	"x-ui/logger"
)

type State int

const (
	StateStopped State = iota
	StateStarting
	StateRunning
	StateStopping
	StateError
)

type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	State() State
}

type LifecycleManager struct {
	mu         sync.RWMutex
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
		start := time.Now()
		if err := c.Start(ctx); err != nil {
			logger.Errorf("[Lifecycle] Failed to start component %s (took %v): %v", c.Name(), time.Since(start), err)
			return err
		}
		logger.Infof("[Lifecycle] Component %s started successfully (took %v)", c.Name(), time.Since(start))
	}
	return nil
}

func (m *LifecycleManager) StopAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop in reverse order (LIFO Principle)
	for i := len(m.components) - 1; i >= 0; i-- {
		c := m.components[i]
		logger.Infof("[Lifecycle] Stopping component: %s", c.Name())
		start := time.Now()

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- c.Stop(ctx)
		}()

		select {
		case err := <-stopDone:
			duration := time.Since(start)
			if err != nil {
				logger.Errorf("[Lifecycle] Error stopping component %s (took %v): %v", c.Name(), duration, err)
			} else {
				logger.Infof("[Lifecycle] Component %s stopped (took %v)", c.Name(), duration)
			}
		case <-ctx.Done():
			logger.Errorf("[Lifecycle] Timeout stopping component %s after %v", c.Name(), time.Since(start))
		}
	}
}
