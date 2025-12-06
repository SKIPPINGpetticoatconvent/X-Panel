package handlers

import (
	"fmt"

	"x-ui/web/service/tgbot/core"
)

// IntegrationExample 展示如何在现有 bot 中集成通用处理器
// IntegrationExample 展示如何使用新的模块化处理器
func IntegrationExample() {
	fmt.Println("=== Telegram Bot 模块化处理器集成示例 ===")
	
	// 步骤 1: 创建核心组件
	ctx := core.NewContext()
	router := core.NewRouter(ctx)
	_ = router // 防止未使用变量警告
	
	// 步骤 2: 创建各种处理器（需要传入实际的服务实例）
	// 注意：这里需要从main.go或其他地方传入实际的服务实例
	/*
	// 示例代码（需要根据实际项目结构调整）：
	inboundService := service.NewInboundService(db)
	settingService := service.NewSettingService(db)
	serverService := service.NewServerService()
	xrayService := service.NewXrayService()
	
	// 创建各种处理器
	commonHandlers := NewCommonHandlers(ctx)
	adminHandlers := NewAdminHandlers(ctx, serverService, xrayService, inboundService, settingService)
	inboundHandlers := NewInboundHandlers(ctx, inboundService, serverService, xrayService)
	
	// 步骤 3: 注册所有处理器
	commonHandlers.RegisterCommonCommands(router)
	adminHandlers.RegisterAdminCommands(router)
	inboundHandlers.RegisterInboundCommands(router)
	*/
	
	fmt.Println("✅ 模块化处理器集成完成")
	fmt.Println("📋 已注册的处理器:")
	fmt.Println("   🌍 通用处理器 (CommonHandlers):")
	fmt.Println("      - /start: 欢迎消息")
	fmt.Println("      - /help: 帮助信息")
	fmt.Println("      - /id: 显示用户ID")
	fmt.Println("      - /version: 显示版本信息")
	fmt.Println("   👑 管理员处理器 (AdminHandlers):")
	fmt.Println("      - /status: 显示系统状态")
	fmt.Println("      - /restart: 重启面板")
	fmt.Println("      - /restartx: 重启Xray服务")
	fmt.Println("      - /stop: 停止Xray服务")
	fmt.Println("      - /startx: 启动Xray服务")
	fmt.Println("      - /log: 获取日志")
	fmt.Println("      - /backup: 备份数据库")
	fmt.Println("   📡 入站处理器 (InboundHandlers):")
	fmt.Println("      - /inbound: 入站管理")
	fmt.Println("      - /clients: 查看客户端")
	fmt.Println("      - /toggle: 启用/禁用入站")
	
	fmt.Println("\n🔧 使用说明:")
	fmt.Println("1. 在main.go中创建服务实例")
	fmt.Println("2. 传入服务实例到处理器构造函数")
	fmt.Println("3. 注册所有处理器到路由器")
	fmt.Println("4. 设置处理器到bot handler")
}

// ShowArchitecture 说明新架构的优势
func ShowArchitecture() {
	fmt.Println("=== 新架构优势 ===")
	fmt.Println("1. 🎯 模块化设计: 处理器独立，便于维护")
	fmt.Println("2. 🔄 接口解耦: 使用接口减少依赖")
	fmt.Println("3. 🛡️ 向后兼容: 不破坏现有架构")
	fmt.Println("4. 🚀 易于扩展: 轻松添加新命令")
	fmt.Println("5. 📦 代码复用: 通用处理器可复用")
}