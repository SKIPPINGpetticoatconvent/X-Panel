package web

import (
	"x-ui/logger"
)

type CronLogger struct{}

func (l CronLogger) Info(msg string, keysAndValues ...interface{}) {
	// Cron info logs
	logger.Infof("[Cron] %s %v", msg, keysAndValues)
}

func (l CronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	// Cron recovery logs with [PANIC RECOVER] prefix
	logger.Errorf("[PANIC RECOVER] [Cron] %s: %v %v", msg, err, keysAndValues)
}
