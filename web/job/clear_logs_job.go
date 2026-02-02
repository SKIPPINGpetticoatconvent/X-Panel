package job

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"x-ui/logger"
	"x-ui/xray"
)

type ClearLogsJob struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClearLogsJob() *ClearLogsJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &ClearLogsJob{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (j *ClearLogsJob) Name() string {
	return "ClearLogsJob"
}

// Start runs daily log clearing
func (j *ClearLogsJob) Start() error {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		// @daily -> 24h
		// 但是 cron 的 @daily 是每天午夜。Ticker 是随启动时间。
		// 为了简化，我们先用 24h。如果需要更精准的午夜执行，可以算一下 sleep 时间。
		// 这里暂用 Ticker 24h 作为一个简单的近似替代。
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				j.Run()
			case <-j.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (j *ClearLogsJob) Stop() error {
	j.cancel()
	j.wg.Wait()
	return nil
}

// ensureFileExists creates the necessary directories and file if they don't exist
func ensureFileExists(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	//nolint:gosec
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	_ = file.Close()
	return nil
}

// Here Run is an interface method of the Job interface
func (j *ClearLogsJob) Run() {
	logFiles := []string{xray.GetIPLimitLogPath(), xray.GetIPLimitBannedLogPath(), xray.GetAccessPersistentLogPath()}
	logFilesPrev := []string{xray.GetIPLimitBannedPrevLogPath(), xray.GetAccessPersistentPrevLogPath()}

	// Ensure all log files and their paths exist
	for _, path := range append(logFiles, logFilesPrev...) {
		if err := ensureFileExists(path); err != nil {
			logger.Warning("Failed to ensure log file exists:", path, "-", err)
		}
	}

	// Clear log files and copy to previous logs
	for i := 0; i < len(logFiles); i++ {
		if i > 0 {
			// Copy to previous logs
			logFilePrev, err := os.OpenFile(logFilesPrev[i-1], os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				logger.Warning("Failed to open previous log file for writing:", logFilesPrev[i-1], "-", err)
				continue
			}

			logFile, err := os.OpenFile(logFiles[i], os.O_RDONLY, 0o600)
			if err != nil {
				logger.Warning("Failed to open current log file for reading:", logFiles[i], "-", err)
				_ = logFilePrev.Close()
				continue
			}

			_, err = io.Copy(logFilePrev, logFile)
			if err != nil {
				logger.Warning("Failed to copy log file:", logFiles[i], "to", logFilesPrev[i-1], "-", err)
			}

			_ = logFile.Close()
			_ = logFilePrev.Close()
		}

		err := os.Truncate(logFiles[i], 0)
		if err != nil {
			logger.Warning("Failed to truncate log file:", logFiles[i], "-", err)
		}
	}
}
