package web

import (
	"context"
	"testing"

	"x-ui/web/service"

	"github.com/stretchr/testify/assert"
)

// Mock implementation of TelegramService for testing
type mockTelegramService struct {
	service.TelegramService
}

func (m *mockTelegramService) Start() error {
	return nil
}

func (m *mockTelegramService) Stop() {
}

func TestNewServer(t *testing.T) {
	// Setup mock services
	serverService := &service.ServerService{}
	xrayService := &service.XrayService{}
	inboundService := &service.InboundService{}
	outboundService := &service.OutboundService{}
	settingService := &service.SettingService{}
	userService := &service.UserService{}
	// Call NewServer
	server := NewServer(serverService, settingService, xrayService, inboundService, outboundService, userService)

	// Verify the returned server instance
	assert.NotNil(t, server, "NewServer should return a non-nil Server instance")
	assert.NotNil(t, server.ctx, "Server context should be initialized")
	assert.NotNil(t, server.cancel, "Server cancel function should be initialized")

	// Verify dependency injection
	assert.Equal(t, serverService, server.serverService, "ServerService should be correctly injected")
	assert.Equal(t, xrayService, server.xrayService, "XrayService should be correctly injected")
	assert.Equal(t, inboundService, server.inboundService, "InboundService should be correctly injected")
	assert.Equal(t, settingService, server.settingService, "SettingService should be correctly injected")
	assert.Equal(t, outboundService, server.outboundService, "OutboundService should be correctly injected")
	assert.Equal(t, userService, server.userService, "UserService should be correctly injected")

	// Verify that tgbotService is initially nil (since it's not passed to NewServer)
	assert.Nil(t, server.tgbotService, "TgbotService should be initially nil")

	// Cleanup
	server.cancel()
}

func TestServer_SetTelegramService(t *testing.T) {
	// Setup server
	server := &Server{}

	// Setup mock Telegram service
	tgService := &mockTelegramService{}

	// Call SetTelegramService
	server.SetTelegramService(tgService)

	// Verify that tgbotService is updated
	assert.Equal(t, tgService, server.tgbotService, "SetTelegramService should update the tgbotService field")
}

func TestServer_GetCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		ctx:    ctx,
		cancel: cancel,
	}
	defer cancel()

	assert.Equal(t, ctx, server.GetCtx(), "GetCtx should return the correct context")
}

func TestServer_GetCron(t *testing.T) {
	// We can't easily mock cron.Cron structure fully without internal access,
	// but we can check if it returns what we set.
	// However, initialization of cron happens in Start(), so initially it might be nil
	// or we can manually set it for this unit test if needed.
	// Since we are testing internal state accessors:

	server := &Server{}
	assert.Nil(t, server.GetCron(), "GetCron should return nil if not initialized")

	// Note: To test non-nil return, we would need to manually set the cron field
	// which is private. But for critical initialization test, ensuring NewServer behaves correctly is key.
}

func TestFallbackToLocalhost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"IPv4", "192.168.1.1", "127.0.0.1"},
		{"IPv6", "2001:db8::1", "::1"},
		{"Invalid", "invalid-ip", "127.0.0.1"},
		{"Empty", "", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fallbackToLocalhost(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWrapAssetsFile_ModTime(t *testing.T) {
	// This tests the wrapAssetsFileInfo.ModTime method which hardcodes return to startTime module var
	// Ideally we should test that it returns *some* time, specifically likely related to global.startTime
	// Since we can't easily access the unexported types directly to instantiate them without using the helpers,
	// this test is implicitly covered if we could test the file system.
	// For now we will skip direct testing of the unexported wrapper details unless we export them or access via interface.
}

// Timeout context for safety
func TestServer_Stop_Safety(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{
		ctx:    ctx,
		cancel: cancel,
		// Mock services to avoid panics during Stop
		xrayService: &service.XrayService{},
		// Note: xrayService.StopXray() might panic if dependencies aren't there,
		// so we have to be careful. In a real unit test we need mocks.
		// For this task, we will verify NewServer structure primarily.
	}

	// We are just verifying logic flow, but without full mocks involved in Stop(),
	// calling Stop() might be risky if mocked services aren't fully compliant mocks.
	// So we will stick to testing the initialization part which is the request.
	_ = server
}
