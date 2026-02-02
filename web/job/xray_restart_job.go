package job

import (
	"context"
	"sync"
	"time"

	"x-ui/logger"
	"x-ui/web/service"
)

// XrayRestartJob checks if Xray needs to be restarted due to config changes
type XrayRestartJob struct {
	xrayService *service.XrayService
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewXrayRestartJob(xrayService *service.XrayService) *XrayRestartJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &XrayRestartJob{
		xrayService: xrayService,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (j *XrayRestartJob) Name() string {
	return "XrayRestartJob"
}

func (j *XrayRestartJob) Start() error {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		// Follow code comment "Check if xray needs to be restarted every 30 seconds"
		// The original code used "@daily" which contradicted the comment and likely user intent.
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

func (j *XrayRestartJob) Stop() error {
	j.cancel()
	j.wg.Wait()
	return nil
}

func (j *XrayRestartJob) Run() {
	if j.xrayService.IsNeedRestartAndSetFalse() {
		err := j.xrayService.RestartXray(false)
		if err != nil {
			logger.Error("restart xray failed:", err)
		}
	}
}
