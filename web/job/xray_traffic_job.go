package job

import (
	"x-ui/logger"
	"x-ui/web/service"
	"x-ui/xray"
)

type XrayTrafficJob struct {
	xrayService     *service.XrayService
	inboundService  *service.InboundService
	outboundService *service.OutboundService
	xrayAPI         xray.API
}

// NewXrayTrafficJob 创建流量统计任务（不推荐使用，建议使用 NewXrayTrafficJobWithDeps）
// Deprecated: Use NewXrayTrafficJobWithDeps for proper dependency injection
func NewXrayTrafficJob() *XrayTrafficJob {
	// 创建带默认依赖的实例以避免 nil 指针
	xrayService := &service.XrayService{}
	inboundService := &service.InboundService{}
	outboundService := &service.OutboundService{}
	xrayAPI := &xray.XrayAPI{}

	// 注入依赖
	xrayService.SetXrayAPI(xrayAPI)
	xrayService.SetInboundService(inboundService)
	inboundService.SetXrayAPI(xrayAPI)

	return &XrayTrafficJob{
		xrayService:     xrayService,
		inboundService:  inboundService,
		outboundService: outboundService,
		xrayAPI:         xrayAPI,
	}
}

// NewXrayTrafficJobWithDeps 使用依赖注入创建流量统计任务
func NewXrayTrafficJobWithDeps(xrayService *service.XrayService, inboundService *service.InboundService, outboundService *service.OutboundService, xrayAPI xray.API) *XrayTrafficJob {
	return &XrayTrafficJob{
		xrayService:     xrayService,
		inboundService:  inboundService,
		outboundService: outboundService,
		xrayAPI:         xrayAPI,
	}
}

func (j *XrayTrafficJob) Run() {
	// 防御性检查：确保 xrayService 不为 nil
	if j.xrayService == nil {
		logger.Warning("XrayTrafficJob: xrayService is nil, skipping traffic collection")
		return
	}

	if !j.xrayService.IsXrayRunning() {
		return
	}

	// 获取 API 端口并检查是否有效
	apiPort := j.xrayService.GetApiPort()
	if apiPort == 0 {
		logger.Debug("XrayTrafficJob: Xray API port is 0, skipping traffic collection")
		return
	}

	traffics, clientTraffics, err := j.xrayService.GetXrayTraffic()
	if err != nil {
		logger.Debug("XrayTrafficJob: GetXrayTraffic failed:", err)
		return
	}

	if j.inboundService != nil {
		err, needRestart0 := j.inboundService.AddTraffic(traffics, clientTraffics)
		if err != nil {
			logger.Warning("add inbound traffic failed:", err)
		}
		if needRestart0 {
			j.xrayService.SetToNeedRestart()
		}
	}

	if j.outboundService != nil {
		err, needRestart1 := j.outboundService.AddTraffic(traffics, clientTraffics)
		if err != nil {
			logger.Warning("add outbound traffic failed:", err)
		}
		if needRestart1 {
			j.xrayService.SetToNeedRestart()
		}
	}
}
