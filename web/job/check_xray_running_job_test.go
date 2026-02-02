package job

import (
	"testing"

	"x-ui/web/service"
)

func TestNewCheckXrayRunningJob(t *testing.T) {
	xrayService := &service.XrayService{}
	job := NewCheckXrayRunningJob(xrayService)

	if job == nil {
		t.Fatal("NewCheckXrayRunningJob returned nil")
	}
	if job.xrayService != xrayService {
		t.Error("xrayService was not set correctly")
	}
}
