package job

// Job defines the standard interface for all background tasks
type Job interface {
	// Start starts the background task. It should be non-blocking (async).
	Start() error
	// Stop stops the background task gracefully. It should wait for the task to exit.
	Stop() error
	// Name returns the identifier of the job
	Name() string
}
