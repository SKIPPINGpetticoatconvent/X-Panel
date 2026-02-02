//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"x-ui/logger"
	"x-ui/web/job"
)

// setupSignalHandler 注册信号监听（Unix版包含 SIGUSR2）
func setupSignalHandler(sigCh chan os.Signal) {
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGUSR2)
}

// handleCustomSignal 处理平台特定的信号（如 SIGUSR2）
// 返回 true 表示信号已被处理，无需进一步操作
func handleCustomSignal(sig os.Signal, monitorJob *job.CertMonitorJob) bool {
	if sig == syscall.SIGUSR2 {
		if monitorJob != nil {
			logger.Info("Received SIGUSR2 signal. Triggering CertMonitorJob manually...")
			monitorJob.Run()
		}
		return true
	}
	return false
}
