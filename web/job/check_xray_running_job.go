package job

import (
	"context"
	"sync"
	"time"

	"x-ui/logger"
	"x-ui/web/service"
)

type CheckXrayRunningJob struct {
	xrayService *service.XrayService

	checkTime int
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewCheckXrayRunningJob(xrayService *service.XrayService) *CheckXrayRunningJob {
	ctx, cancel := context.WithCancel(context.Background())
	return &CheckXrayRunningJob{
		xrayService: xrayService,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (j *CheckXrayRunningJob) Name() string {
	return "CheckXrayRunningJob"
}

func (j *CheckXrayRunningJob) Start() error {
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		// @every 1s
		ticker := time.NewTicker(1 * time.Second)
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

func (j *CheckXrayRunningJob) Stop() error {
	j.cancel()
	j.wg.Wait()
	return nil
}

func (j *CheckXrayRunningJob) Run() {
	if j.xrayService == nil {
		return
	}
	if !j.xrayService.DidXrayCrash() {
		j.checkTime = 0
	} else {
		j.checkTime++
		// only restart if it's down 2 times in a row
		if j.checkTime > 1 {
			err := j.xrayService.RestartXray(false)
			j.checkTime = 0
			if err != nil {
				logger.Error("Restart xray failed:", err)
			}
		}
	}
}
