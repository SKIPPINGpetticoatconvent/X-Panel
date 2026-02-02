//go:build windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"x-ui/web/job"
)

// setupSignalHandler 注册信号监听（Windows版仅包含基础信号）
func setupSignalHandler(sigCh chan os.Signal) {
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
}

// handleCustomSignal 处理平台特定的信号
// Windows 不支持 SIGUSR2，直接返回 false
func handleCustomSignal(sig os.Signal, monitorJob *job.CertMonitorJob) bool {
	return false
}
