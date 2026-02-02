package job

import (
	"testing"

	"x-ui/web/service"
)

func TestNewXrayTrafficJob(t *testing.T) {
	xrayService := &service.XrayService{}
	inboundService := &service.InboundService{}
	outboundService := &service.OutboundService{}

	job := NewXrayTrafficJob(xrayService, inboundService, outboundService)

	if job == nil {
		t.Fatal("NewXrayTrafficJob returned nil")
	}
	if job.xrayService != xrayService {
		t.Error("xrayService was not set correctly")
	}
	if job.inboundService != inboundService {
		t.Error("inboundService was not set correctly")
	}
	if job.outboundService != outboundService {
		t.Error("outboundService was not set correctly")
	}
}
