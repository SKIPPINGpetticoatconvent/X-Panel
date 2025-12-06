package handlers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"x-ui/config"
	"x-ui/util/common"
	"x-ui/web/service"
	"x-ui/web/service/tgbot/core"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// AdminHandlers 管理员命令处理器
type AdminHandlers struct {
	ctx             core.ContextInterface
	serverService   *service.ServerService
	xrayService     *service.XrayService
	inboundService  *service.InboundService
	settingService  *service.SettingService
}

// NewAdminHandlers 创建管理员处理器实例
func NewAdminHandlers(
	ctx core.ContextInterface,
	serverService *service.ServerService,
	xrayService *service.XrayService,
	inboundService *service.InboundService,
	settingService *service.SettingService,
) *AdminHandlers {
	return &AdminHandlers{
		ctx:            ctx,
		serverService:  serverService,
		xrayService:    xrayService,
		inboundService: inboundService,
		settingService: settingService,
	}
}

// HandleStatus 处理 /status 命令 - 显示系统状态
func (h *AdminHandlers) HandleStatus(message telego.Message) error {
	status := h.serverService.GetStatus(nil)
	
	msg := fmt.Sprintf(`🤖 <b>系统状态</b>

📊 <b>CPU 使用率:</b> %.2f%%
🖥️ <b>CPU 核心:</b> %d 核
⚡ <b>CPU 频率:</b> %.0f MHz

💾 <b>内存使用:</b> %s / %s
💿 <b>磁盘使用:</b> %s / %s

⏰ <b>运行时间:</b> %d 天
📈 <b>系统负载:</b> %.2f | %.2f | %.2f

🌐 <b>TCP 连接:</b> %d
🌐 <b>UDP 连接:</b> %d

📡 <b>网络流量:</b>
   ↑ 上传: %s
   ↓ 下载: %s

🔧 <b>Xray 状态:</b> %s
🔧 <b>Xray 版本:</b> %s

🏠 <b>主机名:</b> %s
📦 <b>面板版本:</b> %s`,
		status.Cpu,
		status.CpuCores,
		status.CpuSpeedMhz,
		common.FormatTraffic(int64(status.Mem.Current)),
		common.FormatTraffic(int64(status.Mem.Total)),
		common.FormatTraffic(int64(status.Disk.Current)),
		common.FormatTraffic(int64(status.Disk.Total)),
		status.Uptime/86400,
		status.Loads[0], status.Loads[1], status.Loads[2],
		status.TcpCount,
		status.UdpCount,
		common.FormatTraffic(int64(status.NetTraffic.Sent)),
		common.FormatTraffic(int64(status.NetTraffic.Recv)),
		status.Xray.State,
		status.Xray.Version,
		h.ctx.GetHostname(),
		config.GetVersion(),
	)

	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// HandleRestart 处理 /restart 命令 - 重启面板
func (h *AdminHandlers) HandleRestart(message telego.Message) error {
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 是，立即重启").WithCallbackData(h.ctx.I18n("admin.confirm_restart")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 否，我再想想").WithCallbackData(h.ctx.I18n("admin.cancel_restart")),
		),
	)

	msg := "🤔 您确定要重启 X-Panel 面板吗？\n\n这也会同时重启 Xray Core，会使面板在短时间内无法访问。"
	
	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg, confirmKeyboard)
}

// HandleRestartX 处理 /restartx 命令 - 重启 Xray 服务
func (h *AdminHandlers) HandleRestartX(message telego.Message) error {
	if !h.xrayService.IsXrayRunning() {
		msg := "❌ Xray 服务当前未运行，无法重启"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 发送确认消息
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 是，重启 Xray").WithCallbackData(h.ctx.I18n("admin.confirm_restartx")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 否，取消操作").WithCallbackData(h.ctx.I18n("admin.cancel_restartx")),
		),
	)

	msg := "🔄 您确定要重启 Xray 服务吗？\n\n这可能会短暂影响现有的连接。"
	
	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg, confirmKeyboard)
}

// HandleStopX 处理 /stop 命令 - 停止 Xray 服务
func (h *AdminHandlers) HandleStopX(message telego.Message) error {
	if !h.xrayService.IsXrayRunning() {
		msg := "❌ Xray 服务当前未运行"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 发送确认消息
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 是，停止 Xray").WithCallbackData(h.ctx.I18n("admin.confirm_stopx")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 否，取消操作").WithCallbackData(h.ctx.I18n("admin.cancel_stopx")),
		),
	)

	msg := "🛑 您确定要停止 Xray 服务吗？\n\n这将中断所有代理连接。"
	
	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg, confirmKeyboard)
}

// HandleStartX 处理 /startx 命令 - 启动 Xray 服务
func (h *AdminHandlers) HandleStartX(message telego.Message) error {
	if h.xrayService.IsXrayRunning() {
		msg := "✅ Xray 服务已经在运行"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 发送确认消息
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 是，启动 Xray").WithCallbackData(h.ctx.I18n("admin.confirm_startx")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 否，取消操作").WithCallbackData(h.ctx.I18n("admin.cancel_startx")),
		),
	)

	msg := "🚀 您确定要启动 Xray 服务吗？\n\n这将恢复所有代理功能。"
	
	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg, confirmKeyboard)
}

// HandleLog 处理 /log 命令 - 获取日志
func (h *AdminHandlers) HandleLog(message telego.Message, args []string) error {
	count := "100" // 默认获取100行日志
	level := "info" // 默认日志级别
	
	// 解析参数
	if len(args) > 0 {
		if c, err := strconv.Atoi(args[0]); err == nil && c > 0 && c <= 1000 {
			count = args[0]
		}
	}
	if len(args) > 1 {
		level = args[1]
	}

	logs := h.serverService.GetLogs(count, level, "false")
	
	if len(logs) == 0 {
		msg := "📋 未找到日志记录"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 构建日志消息
	msg := fmt.Sprintf("📋 <b>系统日志</b> (最近 %s 行, 级别: %s)\n\n", count, level)
	
	for i, log := range logs {
		if i >= 50 { // 限制显示行数避免消息过长
			msg += "\n... (更多日志请查看面板)"
			break
		}
		msg += fmt.Sprintf("%s\n", log)
	}

	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// HandleBackup 处理 /backup 命令 - 备份数据库
func (h *AdminHandlers) HandleBackup(message telego.Message) error {
	msg := "💾 正在准备数据库备份..."
	h.ctx.SendMsgToTgbot(message.Chat.ID, msg)

	// 获取数据库文件
	dbData, err := h.serverService.GetDb()
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 备份失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	// 创建临时文件
	tempFileName := fmt.Sprintf("xui_backup_%s.db", time.Now().Format("2006-01-02_15-04-05"))
	tempFile, err := os.CreateTemp("", tempFileName)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 创建临时文件失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}
	defer os.Remove(tempFile.Name())

	// 写入数据库数据
	if _, err := tempFile.Write(dbData); err != nil {
		errorMsg := fmt.Sprintf("❌ 写入备份文件失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}
	tempFile.Close()

	// 发送备份文件
	file, err := os.Open(tempFile.Name())
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 打开备份文件失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}
	defer file.Close()

	document := tu.Document(
		tu.ID(message.Chat.ID),
		tu.File(file),
	).WithCaption(fmt.Sprintf("💾 X-Panel 数据库备份\n📅 备份时间: %s\n📦 面板版本: %s", 
		time.Now().Format("2006-01-02 15:04:05"), config.GetVersion()))

	// 获取 bot 实例发送文档
	bot := h.ctx.GetBot()
	_, err = bot.SendDocument(context.Background(), document)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 发送备份文件失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	successMsg := "✅ 数据库备份已发送"
	return h.ctx.SendMsgToTgbot(message.Chat.ID, successMsg)
}

// HandleCallback 处理管理员命令的回调
func (h *AdminHandlers) HandleCallback(query telego.CallbackQuery, decodedQuery string) error {
	chatId := query.Message.GetChat().ID
	
	switch decodedQuery {
	case "admin.confirm_restart":
		return h.executeRestartPanel(chatId, query.ID)
	case "admin.cancel_restart":
		return h.cancelOperation(chatId, query.ID, "已取消重启操作")
		
	case "admin.confirm_restartx":
		return h.executeRestartXray(chatId, query.ID)
	case "admin.cancel_restartx":
		return h.cancelOperation(chatId, query.ID, "已取消重启 Xray 操作")
		
	case "admin.confirm_stopx":
		return h.executeStopXray(chatId, query.ID)
	case "admin.cancel_stopx":
		return h.cancelOperation(chatId, query.ID, "已取消停止 Xray 操作")
		
	case "admin.confirm_startx":
		return h.executeStartXray(chatId, query.ID)
	case "admin.cancel_startx":
		return h.cancelOperation(chatId, query.ID, "已取消启动 Xray 操作")
	}
	
	return nil
}

// executeRestartPanel 执行重启面板
func (h *AdminHandlers) executeRestartPanel(chatId int64, queryID string) error {
	h.ctx.AnswerCallbackQuery(queryID, "正在重启面板...")
	
	// 在后台执行重启
	go func() {
		err := h.serverService.RestartPanel()
		if err != nil {
			h.ctx.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 面板重启失败: %v", err))
		} else {
			h.ctx.SendMsgToTgbot(chatId, "✅ 面板重启命令已发送，请稍候...")
		}
	}()
	
	return nil
}

// executeRestartXray 执行重启 Xray
func (h *AdminHandlers) executeRestartXray(chatId int64, queryID string) error {
	h.ctx.AnswerCallbackQuery(queryID, "正在重启 Xray...")
	
	go func() {
		err := h.serverService.RestartXrayService()
		if err != nil {
			h.ctx.SendMsgToTgbot(chatId, fmt.Sprintf("❌ Xray 重启失败: %v", err))
		} else {
			h.ctx.SendMsgToTgbot(chatId, "✅ Xray 重启成功！")
		}
	}()
	
	return nil
}

// executeStopXray 执行停止 Xray
func (h *AdminHandlers) executeStopXray(chatId int64, queryID string) error {
	h.ctx.AnswerCallbackQuery(queryID, "正在停止 Xray...")
	
	go func() {
		err := h.serverService.StopXrayService()
		if err != nil {
			h.ctx.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 停止 Xray 失败: %v", err))
		} else {
			h.ctx.SendMsgToTgbot(chatId, "✅ Xray 已停止")
		}
	}()
	
	return nil
}

// executeStartXray 执行启动 Xray
func (h *AdminHandlers) executeStartXray(chatId int64, queryID string) error {
	h.ctx.AnswerCallbackQuery(queryID, "正在启动 Xray...")
	
	go func() {
		err := h.serverService.RestartXrayService()
		if err != nil {
			h.ctx.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 启动 Xray 失败: %v", err))
		} else {
			h.ctx.SendMsgToTgbot(chatId, "✅ Xray 已启动")
		}
	}()
	
	return nil
}

// cancelOperation 取消操作
func (h *AdminHandlers) cancelOperation(chatId int64, queryID, message string) error {
	h.ctx.AnswerCallbackQuery(queryID, "已取消")
	h.ctx.SendMsgToTgbot(chatId, message)
	return nil
}

// RegisterAdminCommands 注册管理员命令到路由器
func (h *AdminHandlers) RegisterAdminCommands(router core.ExternalCommandRegistry) {
	// 注册 /status 命令
	router.RegisterCommandExt("status", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		return h.HandleStatus(message)
	})

	// 注册 /restart 命令
	router.RegisterCommandExt("restart", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		return h.HandleRestart(message)
	})

	// 注册 /restartx 命令
	router.RegisterCommandExt("restartx", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		return h.HandleRestartX(message)
	})

	// 注册 /stop 命令
	router.RegisterCommandExt("stop", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		return h.HandleStopX(message)
	})

	// 注册 /startx 命令
	router.RegisterCommandExt("startx", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		return h.HandleStartX(message)
	})

	// 注册 /log 命令
	router.RegisterCommandExt("log", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		// 解析命令参数
		_, _, args := tu.ParseCommand(message.Text)
		return h.HandleLog(message, args)
	})

	// 注册 /backup 命令
	router.RegisterCommandExt("backup", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		return h.HandleBackup(message)
	})
}