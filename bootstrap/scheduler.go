package bootstrap

import (
	"x-ui/web/job"
	"x-ui/web/service"
)

// RegisterJobs 注册所有后台任务到 JobManager
func RegisterJobs(
	jobManager *job.Manager,
	app *App,
	tgBotService service.TelegramService,
) *job.CertMonitorJob {
	// 设备限制检查任务
	checkJob := job.NewCheckDeviceLimitJob(
		app.InboundService,
		app.XrayService,
		tgBotService,
		*app.SettingService,
	)
	jobManager.Register(checkJob)

	// 证书监控任务
	monitorJob := job.NewCertMonitorJob(*app.SettingService, tgBotService)
	jobManager.Register(monitorJob)

	// Xray 流量统计任务
	trafficJob := job.NewXrayTrafficJob(
		app.XrayService,
		app.InboundService,
		app.OutboundService,
	)
	jobManager.Register(trafficJob)

	// Xray 运行状态检查任务
	xrayRunningJob := job.NewCheckXrayRunningJob(app.XrayService)
	jobManager.Register(xrayRunningJob)

	// 日志清理任务
	clearLogsJob := job.NewClearLogsJob()
	jobManager.Register(clearLogsJob)

	// Xray 配置变更重启任务
	restartJob := job.NewXrayRestartJob(app.XrayService)
	jobManager.Register(restartJob)

	return monitorJob
}
