package bootstrap

import (
	"log"
	"sync"

	"x-ui/sub"
	"x-ui/web"
	"x-ui/web/global"
	"x-ui/web/job"
	"x-ui/web/service"
)

// Runtime 封装应用运行时状态
type Runtime struct {
	App          *App
	WebServer    *web.Server
	SubServer    *sub.Server
	JobManager   *job.Manager
	TgBotService service.TelegramService
	LogForwarder *service.LogForwarder
	MonitorJob   *job.CertMonitorJob

	tgMu sync.RWMutex
}

// NewRuntime 创建运行时实例
func NewRuntime(app *App) *Runtime {
	return &Runtime{
		App:        app,
		JobManager: job.NewManager(),
	}
}

// InitTelegramBot 初始化 Telegram Bot 服务
func (r *Runtime) InitTelegramBot() error {
	tgEnable, err := r.App.SettingService.GetTgbotEnabled()
	if err != nil {
		return err
	}

	r.tgMu.Lock()
	defer r.tgMu.Unlock()

	if tgEnable {
		tgBot := service.NewTgBot(
			r.App.InboundService,
			r.App.SettingService,
			r.App.ServerService,
			r.App.XrayService,
			r.App.LastStatus,
		)
		r.TgBotService = tgBot
		r.LogForwarder = service.NewLogForwarder(r.App.SettingService, r.TgBotService)
	}

	return nil
}

// GetTelegramService 线程安全地获取 Telegram 服务
func (r *Runtime) GetTelegramService() service.TelegramService {
	r.tgMu.RLock()
	defer r.tgMu.RUnlock()
	return r.TgBotService
}

// SetTelegramService 线程安全地设置 Telegram 服务
func (r *Runtime) SetTelegramService(svc service.TelegramService) {
	r.tgMu.Lock()
	defer r.tgMu.Unlock()
	r.TgBotService = svc
}

// StartWebServer 启动 Web 服务器
func (r *Runtime) StartWebServer() error {
	r.WebServer = web.NewServer(
		r.App.ServerService,
		r.App.SettingService,
		r.App.XrayService,
		r.App.InboundService,
		r.App.OutboundService,
	)

	if r.TgBotService != nil {
		r.WebServer.SetTelegramService(r.TgBotService)
	}

	global.SetWebServer(r.WebServer)
	return r.WebServer.Start()
}

// StartSubServer 启动订阅服务器
func (r *Runtime) StartSubServer() error {
	r.SubServer = sub.NewServer(r.App.InboundService, r.App.SettingService)
	global.SetSubServer(r.SubServer)
	return r.SubServer.Start()
}

// StartLogForwarder 启动日志转发器
func (r *Runtime) StartLogForwarder() {
	if r.LogForwarder != nil {
		if err := r.LogForwarder.Start(); err != nil {
			log.Printf("启动日志转发器失败: %v", err)
		}
	}
}

// StartJobs 注册并启动所有后台任务
func (r *Runtime) StartJobs() {
	r.MonitorJob = RegisterJobs(r.JobManager, r.App, r.TgBotService)
	r.JobManager.StartAll()
}

// StopAll 停止所有服务
func (r *Runtime) StopAll() {
	r.JobManager.StopAll()

	if r.LogForwarder != nil {
		_ = r.LogForwarder.Stop()
	}

	if r.WebServer != nil {
		_ = r.WebServer.Stop()
	}

	if r.SubServer != nil {
		_ = r.SubServer.Stop()
	}
}

// Restart 重启所有服务（用于 SIGHUP 信号处理）
func (r *Runtime) Restart() error {
	// 停止所有服务
	r.JobManager.StopAll()

	if r.LogForwarder != nil {
		_ = r.LogForwarder.Stop()
	}

	if r.WebServer != nil {
		_ = r.WebServer.Stop()
	}

	if r.SubServer != nil {
		_ = r.SubServer.Stop()
	}

	// 重新检查 Telegram Bot 设置
	if err := r.refreshTelegramBot(); err != nil {
		log.Printf("刷新 Telegram Bot 设置失败: %v", err)
	}

	// 重新注入依赖
	r.App.WireServices(r.GetTelegramService())

	// 重启 Web 服务器
	if err := r.StartWebServer(); err != nil {
		return err
	}
	log.Println("Web server restarted successfully.")

	// 重启日志转发器
	r.StartLogForwarder()

	// 重启订阅服务器
	if err := r.StartSubServer(); err != nil {
		return err
	}
	log.Println("Sub server restarted successfully.")

	// 重启后台任务
	r.JobManager.StartAll()

	return nil
}

// refreshTelegramBot 刷新 Telegram Bot 服务状态
func (r *Runtime) refreshTelegramBot() error {
	tgEnable, err := r.App.SettingService.GetTgbotEnabled()
	if err != nil {
		return err
	}

	r.tgMu.Lock()
	defer r.tgMu.Unlock()

	if tgEnable {
		if r.TgBotService == nil {
			tgBot := service.NewTgBot(
				r.App.InboundService,
				r.App.SettingService,
				r.App.ServerService,
				r.App.XrayService,
				r.App.LastStatus,
			)
			r.TgBotService = tgBot
		}

		if r.LogForwarder == nil {
			r.LogForwarder = service.NewLogForwarder(r.App.SettingService, r.TgBotService)
		}
	} else {
		r.TgBotService = nil
		r.LogForwarder = nil
		r.App.ServerService.SetTelegramService(nil)
		r.App.InboundService.SetTelegramService(nil)
	}

	return nil
}
