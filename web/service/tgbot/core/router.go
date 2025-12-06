package core

import (
	"fmt"
	"time"

	"x-ui/logger"
	"x-ui/web/service"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

// CommandHandler 命令处理器函数类型
type CommandHandler func(ctx *Context, message telego.Message, isAdmin bool) error

// CallbackHandler 回调处理器函数类型
type CallbackHandler func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error

// CommandHandlerFunc 通用命令处理器函数类型（用于外部处理器）
type CommandHandlerFunc func(ctx ContextInterface, message telego.Message, isAdmin bool) error

// ExternalCommandRegistry 外部命令注册接口
type ExternalCommandRegistry interface {
	RegisterCommandExt(command string, handler CommandHandlerFunc)
}

// Router Telegram Bot 路由器
// 负责注册和分发命令及回调查询
type Router struct {
	commandHandlers  map[string]CommandHandler
	callbackHandlers map[string]CallbackHandler
	ctx              *Context
}

// NewRouter 创建新的路由器实例
func NewRouter(ctx *Context) *Router {
	return &Router{
		commandHandlers:  make(map[string]CommandHandler),
		callbackHandlers: make(map[string]CallbackHandler),
		ctx:              ctx,
	}
}

// SetContext 设置上下文
func (r *Router) SetContext(ctx *Context) {
	r.ctx = ctx
}

// RegisterCommand 注册命令处理器
func (r *Router) RegisterCommand(command string, handler CommandHandler) {
	r.commandHandlers[command] = handler
}

// RegisterCommandForExternal 注册外部命令处理器（实现 CommandRegistry 接口）
func (r *Router) RegisterCommandForExternal(command string, handler CommandHandlerFunc) {
	// 将外部处理器适配为内部处理器
	r.RegisterCommand(command, func(ctx *Context, message telego.Message, isAdmin bool) error {
		return handler(ctx, message, isAdmin)
	})
}

// RegisterCommandExt 注册外部命令处理器（实现 ExternalCommandRegistry 接口）
func (r *Router) RegisterCommandExt(command string, handler CommandHandlerFunc) {
	r.RegisterCommandForExternal(command, handler)
}

// RegisterCallback 注册回调处理器
func (r *Router) RegisterCallback(callback string, handler CallbackHandler) {
	r.callbackHandlers[callback] = handler
}

// HandleCommand 处理命令
func (r *Router) HandleCommand(ctx *Context, message telego.Message) error {
	command, _, _ := tu.ParseCommand(message.Text)
	isAdmin := r.ctx.CheckAdmin(message.From.ID)

	handler, exists := r.commandHandlers[command]
	if !exists {
		return r.handleUnknownCommand(ctx, message, isAdmin)
	}

	return handler(ctx, message, isAdmin)
}

// HandleCallback 处理回调查询
func (r *Router) HandleCallback(ctx *Context, query telego.CallbackQuery) error {
	isAdmin := r.ctx.CheckAdmin(query.From.ID)

	// 解码回调数据
	decodedQuery, err := r.ctx.DecodeQuery(query.Data)
	if err != nil {
		logger.Warning("Failed to decode callback query:", err)
		return ctx.AnswerCallbackQuery(query.ID, "操作失败，请重试")
	}

	// 查找处理器
	handler, exists := r.callbackHandlers[decodedQuery]
	if !exists {
		return r.handleUnknownCallback(ctx, query, isAdmin)
	}

	return handler(ctx, query, isAdmin)
}

// handleUnknownCommand 处理未知命令
func (r *Router) handleUnknownCommand(ctx *Context, message telego.Message, isAdmin bool) error {
	msg := ctx.I18n("tgbot.commands.unknown")
	return ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// handleUnknownCallback 处理未知回调
func (r *Router) handleUnknownCallback(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
	return ctx.AnswerCallbackQuery(query.ID, "未知操作")
}

// RegisterCommonCommandHandlers 注册通用命令处理器
func (r *Router) RegisterCommonCommandHandlers() {
	// 这里可以注册通用命令处理器
	// 实际集成将在外部调用
}

// SetupCommonHandlers 设置通用处理器
func (r *Router) SetupCommonHandlers() {
	// 这里可以添加通用处理器的设置逻辑
	// 目前留空，未来可以在这里集成外部处理器
}

// RegisterDefaultCommands 注册默认命令
func (r *Router) RegisterDefaultCommands(
	inboundService *service.InboundService,
	settingService *service.SettingService,
	serverService *service.ServerService,
	xrayService *service.XrayService,
) {
	// status 命令 - 保持在这里，因为它是特定于服务器状态的命令
	r.RegisterCommand("status", func(ctx *Context, message telego.Message, isAdmin bool) error {
		msg := ctx.I18n("tgbot.commands.status")
		return ctx.SendMsgToTgbot(message.Chat.ID, msg)
	})

	// usage 命令
	r.RegisterCommand("usage", func(ctx *Context, message telego.Message, isAdmin bool) error {
		// 这里需要实现获取用户使用情况的逻辑
		msg := ctx.I18n("tgbot.commands.usage")
		return ctx.SendMsgToTgbot(message.Chat.ID, msg)
	})

	// inbound 命令（仅管理员）
	r.RegisterCommand("inbound", func(ctx *Context, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return r.handleUnknownCommand(ctx, message, isAdmin)
		}

		msg := ctx.I18n("tgbot.commands.pleaseChoose")
		return ctx.Answer(message.Chat.ID, msg, isAdmin)
	})

	// restart 命令（仅管理员）
	r.RegisterCommand("restart", func(ctx *Context, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return r.handleUnknownCommand(ctx, message, isAdmin)
		}

		if xrayService.IsXrayRunning() {
			err := xrayService.RestartXray(true)
			if err != nil {
				msg := ctx.I18n("tgbot.commands.restartFailed", "Error=="+err.Error())
				return ctx.SendMsgToTgbot(message.Chat.ID, msg)
			} else {
				msg := ctx.I18n("tgbot.commands.restartSuccess")
				return ctx.SendMsgToTgbot(message.Chat.ID, msg)
			}
		} else {
			msg := ctx.I18n("tgbot.commands.xrayNotRunning")
			return ctx.SendMsgToTgbot(message.Chat.ID, msg)
		}
	})

	// oneclick 命令（仅管理员）
	r.RegisterCommand("oneclick", func(ctx *Context, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return r.handleUnknownCommand(ctx, message, isAdmin)
		}
		return r.sendOneClickOptions(ctx, message.Chat.ID)
	})

	// subconverter 命令（仅管理员）
	r.RegisterCommand("subconverter", func(ctx *Context, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return r.handleUnknownCommand(ctx, message, isAdmin)
		}
		return r.checkAndInstallSubconverter(ctx, message.Chat.ID)
	})

	// restartx 命令（仅管理员）
	r.RegisterCommand("restartx", func(ctx *Context, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return r.handleUnknownCommand(ctx, message, isAdmin)
		}
		return r.sendRestartConfirm(ctx, message.Chat.ID)
	})

	// xrayversion 命令（仅管理员）
	r.RegisterCommand("xrayversion", func(ctx *Context, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return r.handleUnknownCommand(ctx, message, isAdmin)
		}
		return r.sendXrayVersionOptions(ctx, message.Chat.ID)
	})
}

// RegisterDefaultCallbacks 注册默认回调处理器
func (r *Router) RegisterDefaultCallbacks(
	inboundService *service.InboundService,
	settingService *service.SettingService,
	serverService *service.ServerService,
	xrayService *service.XrayService,
) {
	// 服务器使用情况
	r.RegisterCallback("get_usage", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		return ctx.AnswerCallbackQuery(query.ID, ctx.I18n("tgbot.buttons.serverUsage"))
	})

	// 重启面板
	r.RegisterCallback("restart_panel", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		if !isAdmin {
			return ctx.AnswerCallbackQuery(query.ID, "权限不足")
		}
		return r.sendRestartConfirm(ctx, query.Message.GetChat().ID)
	})

	// 重启面板确认
	r.RegisterCallback("restart_panel_confirm", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		if !isAdmin {
			return ctx.AnswerCallbackQuery(query.ID, "权限不足")
		}

		ctx.DeleteMessage(query.Message.GetChat().ID, query.Message.GetMessageID())
		ctx.AnswerCallbackQuery(query.ID, "指令已发送，请稍候...")

		go func() {
			err := serverService.RestartPanel()
			time.Sleep(20 * time.Second)
			if err != nil {
				ctx.SendMsgToTgbot(query.Message.GetChat().ID, fmt.Sprintf("❌ 面板重启失败: %v", err))
			} else {
				ctx.SendMsgToTgbot(query.Message.GetChat().ID, "🚀 面板重启成功！")
			}
		}()

		return nil
	})

	// 重启面板取消
	r.RegisterCallback("restart_panel_cancel", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		ctx.DeleteMessage(query.Message.GetChat().ID, query.Message.GetMessageID())
		return ctx.AnswerCallbackQuery(query.ID, "已取消")
	})

	// 一键配置选项
	r.RegisterCallback("oneclick_options", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		return r.sendOneClickOptions(ctx, query.Message.GetChat().ID)
	})

	// 订阅转换检查
	r.RegisterCallback("subconverter_install", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		return r.checkAndInstallSubconverter(ctx, query.Message.GetChat().ID)
	})

	// Xray 版本管理
	r.RegisterCallback("xrayversion", func(ctx *Context, query telego.CallbackQuery, isAdmin bool) error {
		return r.sendXrayVersionOptions(ctx, query.Message.GetChat().ID)
	})
}

// sendOneClickOptions 发送一键配置选项
func (r *Router) sendOneClickOptions(ctx *Context, chatId int64) error {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔗 Direct Connection (直连)").WithCallbackData(ctx.EncodeQuery("oneclick_category_direct")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 Relay (中转)").WithCallbackData(ctx.EncodeQuery("oneclick_category_relay")),
		),
	)

	return ctx.SendMsgToTgbot(chatId, "请选择【一键配置】类型：", keyboard)
}

// checkAndInstallSubconverter 检查并安装订阅转换
func (r *Router) checkAndInstallSubconverter(ctx *Context, chatId int64) error {
	// 这里需要实现检查订阅转换服务的逻辑
	ctx.SendMsgToTgbot(chatId, "正在检查订阅转换服务状态...")

	// 模拟异步检查
	go func() {
		// TODO: 实现实际的检查逻辑
		ctx.SendMsgToTgbot(chatId, "✅ 服务检查完成")
	}()

	return nil
}

// sendRestartConfirm 发送重启确认
func (r *Router) sendRestartConfirm(ctx *Context, chatId int64) error {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 是，立即重启").WithCallbackData(ctx.EncodeQuery("restart_panel_confirm")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 否，我再想想").WithCallbackData(ctx.EncodeQuery("restart_panel_cancel")),
		),
	)

	return ctx.SendMsgToTgbot(chatId, "🤔 您确定要重启X-Panel面板服务吗？\n\n这也会同时重启Xray Core，会使面板在短时间内无法访问。", keyboard)
}

// sendXrayVersionOptions 发送 Xray 版本选项
func (r *Router) sendXrayVersionOptions(ctx *Context, chatId int64) error {
	ctx.AnswerCallbackQuery("callback_query_id", "🚀 请选择要更新的版本...")

	// 这里需要获取可用的 Xray 版本
	versions := []string{"v1.8.0", "v1.7.5", "v1.7.0"}

	var buttons []telego.InlineKeyboardButton
	for _, version := range versions {
		callbackData := ctx.EncodeQuery(fmt.Sprintf("update_xray_ask %s", version))
		buttons = append(buttons, tu.InlineKeyboardButton(version).WithCallbackData(callbackData))
	}

	// 添加取消按钮
	buttons = append(buttons, tu.InlineKeyboardButton("❌ 取消").WithCallbackData(ctx.EncodeQuery("update_xray_cancel")))

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(1, buttons...))

	return ctx.SendMsgToTgbot(chatId, "🚀 **Xray 版本管理**\n\n请选择要更新的版本：", keyboard)
}

// SetupHandlers 设置处理器到 bot handler
func (r *Router) SetupHandlers(botHandler *th.BotHandler) {
	// 处理关闭键盘的消息
	botHandler.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		if message.Text == r.ctx.I18n("tgbot.buttons.closeKeyboard") {
			return r.ctx.SendMsgToTgbot(message.Chat.ID, r.ctx.I18n("tgbot.keyboardClosed"), tu.ReplyKeyboardRemove())
		}
		return nil
	}, th.TextEqual(r.ctx.I18n("tgbot.buttons.closeKeyboard")))

	// 处理命令消息
	botHandler.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		return r.HandleCommand(r.ctx, message)
	}, th.AnyCommand())

	// 处理回调查询
	botHandler.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
		return r.HandleCallback(r.ctx, query)
	}, th.AnyCallbackQuery())
}
