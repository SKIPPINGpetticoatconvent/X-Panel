package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

type XrayTrafficJob struct {
	xrayService     *service.XrayService
	inboundService  *service.InboundService
	outboundService *service.OutboundService
}

func NewXrayTrafficJob(xrayService *service.XrayService, inboundService *service.InboundService, outboundService *service.OutboundService) *XrayTrafficJob {
	return &XrayTrafficJob{
		xrayService:     xrayService,
		inboundService:  inboundService,
		outboundService: outboundService,
	}
}

func (j *XrayTrafficJob) Run() {
	if !j.xrayService.IsXrayRunning() {
		return
	}
	traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		return
	}
	err, needRestart0 := j.inboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add inbound traffic failed:", err)
	}
	err, needRestart1 := j.outboundService.AddTraffic(traffics, clientTraffics)
	if err != nil {
		logger.Warning("add outbound traffic failed:", err)
	}
	if needRestart0 || needRestart1 {
		j.xrayService.SetToNeedRestart()
	}
}
