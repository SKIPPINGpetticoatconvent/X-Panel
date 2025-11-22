package job

import (
	"x-ui/web/service"
)

type LoginStatus byte

const (
	LoginSuccess LoginStatus = 1
	LoginFail    LoginStatus = 0
)

type StatsNotifyJob struct {
	xrayService    *service.XrayService
	tgbotService   service.TelegramService
}

func NewStatsNotifyJob(xrayService *service.XrayService, tgbotService service.TelegramService) *StatsNotifyJob {
	job := &StatsNotifyJob{
		xrayService:  xrayService,
		tgbotService: tgbotService,
	}
	return job
}

func (j *StatsNotifyJob) Run() {
	// TODO: 实现统计通知逻辑
	// 这个方法需要在后续开发中实现具体的统计通知功能
}

