package handlers

import (
	"fmt"

	"x-ui/config"
	"x-ui/web/service/tgbot/core"

	"github.com/mymmrac/telego"
)

// CommonHandlers 通用命令处理器
type CommonHandlers struct {
	ctx core.ContextInterface
}

// NewCommonHandlers 创建通用处理器实例
func NewCommonHandlers(ctx core.ContextInterface) *CommonHandlers {
	return &CommonHandlers{
		ctx: ctx,
	}
}

// HandleStart 处理 /start 命令
func (h *CommonHandlers) HandleStart(message telego.Message, isAdmin bool) error {
	// 获取欢迎消息
	msg := h.ctx.I18n("tgbot.commands.start", "Firstname=="+message.From.FirstName)
	
	// 如果是管理员，添加欢迎信息
	if isAdmin {
		msg += h.ctx.I18n("tgbot.commands.welcome", "Hostname=="+h.ctx.GetHostname())
	}
	
	// 添加选择提示
	msg += "\n\n" + h.ctx.I18n("tgbot.commands.pleaseChoose")
	
	return h.ctx.Answer(message.Chat.ID, msg, isAdmin)
}

// HandleHelp 处理 /help 命令
func (h *CommonHandlers) HandleHelp(message telego.Message, isAdmin bool) error {
	var msg string
	
	if isAdmin {
		// 管理员帮助信息
		msg = h.ctx.I18n("tgbot.commands.helpAdminCommands")
	} else {
		// 普通用户帮助信息
		msg = h.ctx.I18n("tgbot.commands.helpClientCommands")
	}
	
	// 添加选择提示
	msg += "\n\n" + h.ctx.I18n("tgbot.commands.pleaseChoose")
	
	return h.ctx.Answer(message.Chat.ID, msg, isAdmin)
}

// HandleID 处理 /id 命令
func (h *CommonHandlers) HandleID(message telego.Message, isAdmin bool) error {
	msg := h.ctx.I18n("tgbot.commands.getID", "ID=="+fmt.Sprintf("%d", message.From.ID))
	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// HandleVersion 处理 /version 命令
func (h *CommonHandlers) HandleVersion(message telego.Message, isAdmin bool) error {
	version := config.GetVersion()
	
	var msg string
	if isAdmin {
		msg = fmt.Sprintf("🤖 <b>X-Panel 版本信息</b>\n\n📦 当前版本：<code>%s</code>\n\n🔧 这是一个管理员命令", version)
	} else {
		msg = fmt.Sprintf("🤖 <b>X-Panel 版本信息</b>\n\n📦 当前版本：<code>%s</code>", version)
	}
	
	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// RegisterCommonCommands 注册通用命令到路由器
func (h *CommonHandlers) RegisterCommonCommands(router core.ExternalCommandRegistry) {
	// 注册 /start 命令
	router.RegisterCommandExt("start", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		return h.HandleStart(message, isAdmin)
	})

	// 注册 /help 命令
	router.RegisterCommandExt("help", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		return h.HandleHelp(message, isAdmin)
	})

	// 注册 /id 命令
	router.RegisterCommandExt("id", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		return h.HandleID(message, isAdmin)
	})

	// 注册 /version 命令
	router.RegisterCommandExt("version", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		return h.HandleVersion(message, isAdmin)
	})
}