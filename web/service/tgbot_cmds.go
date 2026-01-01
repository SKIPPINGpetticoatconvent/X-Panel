package service

import (
	"context" // 新增：用于 tls.Config
	// 新增：用于 json.Marshal / Unmarshal
	// 新增：用于 http.Client / Transport
	// 新增：用于 exec.Command（getDomain 等）
	// 新增：用于 filepath.Base / Dir（getDomain 用到）
	"strconv"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
	// 新增 qrcode 包，用于生成二维码
)

func (t *Tgbot) OnReceive() {
	params := telego.GetUpdatesParams{
		Timeout: 10,
	}

	updates, _ := bot.UpdatesViaLongPolling(context.Background(), &params)

	botHandler, _ = th.NewBotHandler(bot, updates)

	botHandler.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		delete(userStates, message.Chat.ID)
		t.SendMsgToTgbot(message.Chat.ID, t.I18nBot("tgbot.keyboardClosed"), tu.ReplyKeyboardRemove())
		return nil
	}, th.TextEqual(t.I18nBot("tgbot.buttons.closeKeyboard")))

	botHandler.HandleMessage(func(ctx *th.Context, message telego.Message) error {
		delete(userStates, message.Chat.ID)
		t.answerCommand(&message, message.Chat.ID, checkAdmin(message.From.ID))
		return nil
	}, th.AnyCommand())

	// 【修复】: 注册 CallbackQuery Handler，确保按钮回调被正确处理
	botHandler.HandleCallbackQuery(func(ctx *th.Context, query telego.CallbackQuery) error {
		t.answerCallback(&query, checkAdmin(query.From.ID))
		return nil
	}, th.AnyCallbackQuery())

	botHandler.Start()
}

func (t *Tgbot) answerCommand(message *telego.Message, chatId int64, isAdmin bool) {
	msg, onlyMessage := "", false

	command, _, commandArgs := tu.ParseCommand(message.Text)

	// Helper function to handle unknown commands.
	handleUnknownCommand := func() {
		msg += t.I18nBot("tgbot.commands.unknown")
	}

	// Handle the command.
	switch command {
	case "help":
		msg += t.I18nBot("tgbot.commands.help")
		msg += t.I18nBot("tgbot.commands.pleaseChoose")
	case "start":
		msg += t.I18nBot("tgbot.commands.start", "Firstname=="+message.From.FirstName)
		if isAdmin {
			msg += t.I18nBot("tgbot.commands.welcome", "Hostname=="+hostname)
		}
		msg += "\n\n" + t.I18nBot("tgbot.commands.pleaseChoose")
	case "status":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.status")
	case "id":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.getID", "ID=="+strconv.FormatInt(message.From.ID, 10))
	case "usage":
		onlyMessage = true
		if len(commandArgs) > 0 {
			if isAdmin {
				t.searchClient(chatId, commandArgs[0])
			} else {
				t.getClientUsage(chatId, int64(message.From.ID), commandArgs[0])
			}
		} else {
			msg += t.I18nBot("tgbot.commands.usage")
		}
	case "inbound":
		onlyMessage = true
		if isAdmin && len(commandArgs) > 0 {
			t.searchInbound(chatId, commandArgs[0])
		} else {
			handleUnknownCommand()
		}
	case "restart":
		onlyMessage = true
		if isAdmin {
			if len(commandArgs) == 0 {
				if t.xrayService.IsXrayRunning() {
					err := t.xrayService.RestartXray(true)
					if err != nil {
						msg += t.I18nBot("tgbot.commands.restartFailed", "Error=="+err.Error())
					} else {
						msg += t.I18nBot("tgbot.commands.restartSuccess")
					}
				} else {
					msg += t.I18nBot("tgbot.commands.xrayNotRunning")
				}
			} else {
				handleUnknownCommand()
				msg += t.I18nBot("tgbot.commands.restartUsage")
			}
		} else {
			handleUnknownCommand()
		}
	// 【新增代码】: 处理 /oneclick 指令
	case "oneclick":
		onlyMessage = true
		if isAdmin {
			t.sendOneClickOptions(chatId)
		} else {
			handleUnknownCommand()
		}

	// 【新增代码】: 处理 /subconverter 指令
	case "subconverter":
		onlyMessage = true
		if isAdmin {
			t.checkAndInstallSubconverter(chatId)
		} else {
			handleUnknownCommand()
		}

	// 〔中文注释〕: 【新增代码】: 处理 /restartx 指令，用于重启面板
	case "restartx":
		onlyMessage = true
		if isAdmin {
			// 〔中文注释〕: 发送重启确认消息
			confirmKeyboard := tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("✅ 是，立即重启").WithCallbackData(t.encodeQuery("restart_panel_confirm")),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("❌ 否，我再想想").WithCallbackData(t.encodeQuery("restart_panel_cancel")),
				),
			)
			// 〔中文注释〕: 从您提供的需求中引用提示文本
			t.SendMsgToTgbot(chatId, "🤔 您“现在的操作”是要确定进行，\n\n重启〔X-Panel 面板〕服务吗？\n\n这也会同时重启 Xray Core，\n\n会使面板在短时间内无法访问。", confirmKeyboard)
		} else {
			handleUnknownCommand()
		}
	case "xrayversion":
		onlyMessage = true
		t.sendXrayVersionOptions(chatId)
	default:
		handleUnknownCommand()
	}

	if msg != "" {
		t.sendResponse(chatId, msg, onlyMessage, isAdmin)
	}
}

// Helper function to send the message based on onlyMessage flag.
func (t *Tgbot) sendResponse(chatId int64, msg string, onlyMessage, isAdmin bool) {
	if onlyMessage {
		t.SendMsgToTgbot(chatId, msg)
	} else {
		t.SendAnswer(chatId, msg, isAdmin)
	}
}
