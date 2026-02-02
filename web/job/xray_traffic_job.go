package job

import (
	"context"
	"sync"
	"time"

	"x-ui/logger"
	"x-ui/web/service"
)

type XrayTrafficJob struct {
	xrayService     *service.XrayService
	inboundService  *service.InboundService
	outboundService *service.OutboundService
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

func NewXrayTrafficJob(xrayService *service.XrayService, inboundService *service.InboundService, outboundService *service.OutboundService) *XrayTrafficJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &XrayTrafficJob{
		xrayService:     xrayService,
		inboundService:  inboundService,
		outboundService: outboundService,
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (j *XrayTrafficJob) Name() string {
	return "XrayTrafficJob"
}

func (j *XrayTrafficJob) Start() error {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		// 每30秒执行一次 (模拟原 cron: 30 * * * * * 其实是每分钟的第30秒，用30s间隔更频繁一点但也无妨)
		ticker := time.NewTicker(30 * time.Second)
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

func (j *XrayTrafficJob) Stop() error {
	j.cancel()
	j.wg.Wait()
	return nil
}

func (j *XrayTrafficJob) Run() {
	if j.xrayService == nil || j.inboundService == nil || j.outboundService == nil {
		return
	}

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
