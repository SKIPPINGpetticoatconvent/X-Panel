package service

import (
	"x-ui/xray"

	"github.com/google/wire"
)

// ServiceSet 包含所有服务及其相关的 Provider
var ServiceSet = wire.NewSet(
	NewSettingService,
	NewUserService,
	NewOutboundService,
	NewInboundService,
	NewXrayService,
	NewServerService,
	NewTgBot,
	// 接口绑定：将 *Tgbot 实例绑定到 TelegramService 接口
	wire.Bind(new(TelegramService), new(*Tgbot)),
	// 提供基础结构体
	NewXrayAPI,
	NewStatus,
)

func NewXrayAPI() *xray.XrayAPI {
	return &xray.XrayAPI{}
}

func NewStatus() *Status {
	return &Status{}
}
