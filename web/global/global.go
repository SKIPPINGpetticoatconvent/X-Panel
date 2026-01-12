package global

import (
	"context"
	_ "unsafe"

	"github.com/robfig/cron/v3"
)

var (
	webServer WebServer
	subServer SubServer
)

type WebServer interface {
	GetCron() *cron.Cron
	GetCtx() context.Context
	PauseHTTPListener() error
	ResumeHTTPListener() error
	IsListeningOnPort80() bool
}

type SubServer interface {
	GetCtx() context.Context
}

func SetWebServer(s WebServer) {
	webServer = s
}

func GetWebServer() WebServer {
	return webServer
}

func SetSubServer(s SubServer) {
	subServer = s
}

func GetSubServer() SubServer {
	return subServer
}
