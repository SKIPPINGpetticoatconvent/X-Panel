package service

import (
	"fmt"
	"strconv"
	"time"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// ================== 入站列表获取 ==================

func (t *Tgbot) getInboundUsages() string {
	info := ""
	// get traffic
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		t.logError("getInboundUsages", err)
		info += t.I18nBot("tgbot.answers.getInboundsFailed")
	} else {
		// NOTE:If there no any sessions here,need to notify here
		// TODO:Sub-node push, automatic conversion format
		for _, inbound := range inbounds {
			info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
			info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
			info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

			if inbound.ExpiryTime == 0 {
				info += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
			} else {
				info += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
			info += "\r\n"
		}
	}
	return info
}

func (t *Tgbot) getInbounds() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		return nil, t.handleError("getInbounds", err, "tgbot.answers.getInboundsFailed")
	}

	if len(inbounds) == 0 {
		return nil, t.handleError("getInbounds", common.ErrInboundNotFound, "tgbot.answers.getInboundsFailed")
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", "get_clients", inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) getInboundsAddClient() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		return nil, t.handleError("getInboundsAddClient", err, "tgbot.answers.getInboundsFailed")
	}

	if len(inbounds) == 0 {
		return nil, t.handleError("getInboundsAddClient", common.ErrInboundNotFound, "tgbot.answers.getInboundsFailed")
	}

	excludedProtocols := map[model.Protocol]bool{
		model.Tunnel:    true,
		model.Socks:     true,
		model.WireGuard: true,
		model.HTTP:      true,
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		if excludedProtocols[inbound.Protocol] {
			continue
		}

		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", "add_client_to", inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) getInboundClients(chatId int64, id int) (*telego.InlineKeyboardMarkup, error) {
	inbound, err := t.inboundService.GetInbound(id)
	if err != nil {
		return nil, t.handleError("getInboundClients", err, "tgbot.answers.getInboundsFailed")
	}
	clients, err := t.inboundService.GetClients(inbound)
	var buttons []telego.InlineKeyboardButton

	if err != nil {
		return nil, t.handleError("getInboundClients", err, "tgbot.answers.getInboundsFailed")
	}
	if len(clients) > 0 {
		for _, client := range clients {
			buttons = append(buttons, tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery("client_get_usage "+client.Email)))
		}
	} else {
		return nil, t.handleError("getInboundClients", common.ErrClientNotFound, "tgbot.answers.getClientsFailed")
	}
	cols := 0
	if len(buttons) < 6 {
		cols = 3
	} else {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))

	// 添加批量复制按钮
	copyButton := tu.InlineKeyboardButton("📋 批量复制链接").WithCallbackData(t.encodeQuery(fmt.Sprintf("copy_inbound_clients %d", id)))
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []telego.InlineKeyboardButton{copyButton})

	return keyboard, nil
}

// ================== 入站搜索 ==================

func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbounds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		t.sendError(chatId, "searchInbound", err)
		return
	}
	if len(inbounds) == 0 {
		msg := t.I18nBot("tgbot.noInbounds")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, inbound := range inbounds {
		info := ""
		info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
		info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
		info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

		if inbound.ExpiryTime == 0 {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
		} else {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)

		if len(inbound.ClientStats) > 0 {
			output := ""
			for _, traffic := range inbound.ClientStats {
				output += t.clientInfoMsg(&traffic, true, true, true, true, true, true)
			}
			t.SendMsgToTgbot(chatId, output)
		}
	}
}

// ================== 即将耗尽预警 ==================

func (t *Tgbot) getExhausted(chatId int64) {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000
	var exhaustedInbounds []model.Inbound
	var exhaustedClients []xray.ClientTraffic
	var disabledInbounds []model.Inbound
	var disabledClients []xray.ClientTraffic

	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		//nolint:gosec
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		//nolint:gosec
		exDiff = int64(ExpireThreshold) * 86400000
	}
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load Inbounds", err)
	}

	for _, inbound := range inbounds {
		if inbound.Enable {
			if (inbound.ExpiryTime > 0 && (inbound.ExpiryTime-now < exDiff)) ||
				(inbound.Total > 0 && (inbound.Total-(inbound.Up+inbound.Down) < trDiff)) {
				exhaustedInbounds = append(exhaustedInbounds, *inbound)
			}
			if len(inbound.ClientStats) > 0 {
				for _, client := range inbound.ClientStats {
					if client.Enable {
						if (client.ExpiryTime > 0 && (client.ExpiryTime-now < exDiff)) ||
							(client.Total > 0 && (client.Total-(client.Up+client.Down) < trDiff)) {
							exhaustedClients = append(exhaustedClients, client)
						}
					} else {
						disabledClients = append(disabledClients, client)
					}
				}
			}
		} else {
			disabledInbounds = append(disabledInbounds, *inbound)
		}
	}

	// Inbounds
	output := ""
	output += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.inbounds"))
	output += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledInbounds)))
	output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(len(exhaustedInbounds)))

	if len(exhaustedInbounds) > 0 {
		output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+t.I18nBot("tgbot.inbounds"))

		for _, inbound := range exhaustedInbounds {
			output += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
			output += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
			output += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				output += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
			} else {
				output += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
			output += "\r\n"
		}
	}

	// Clients
	exhaustedCC := len(exhaustedClients)
	output += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.clients"))
	output += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledClients)))
	output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(exhaustedCC))

	if exhaustedCC > 0 {
		output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+t.I18nBot("tgbot.clients"))
		var buttons []telego.InlineKeyboardButton
		for _, traffic := range exhaustedClients {
			output += t.clientInfoMsg(&traffic, true, false, false, true, true, false)
			output += "\r\n"
			buttons = append(buttons, tu.InlineKeyboardButton(traffic.Email).WithCallbackData(t.encodeQuery("client_get_usage "+traffic.Email)))
		}
		cols := 0
		if exhaustedCC < 11 {
			cols = 1
		} else {
			cols = 2
		}
		keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
		t.SendMsgToTgbot(chatId, output, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, output)
	}
}

// ================== 在线用户 ==================

func (t *Tgbot) onlineClients(chatId int64, messageID ...int) {
	if t.xrayService == nil || !t.xrayService.IsXrayRunning() {
		return
	}

	onlines := t.xrayService.GetOnlineClients()
	onlinesCount := len(onlines)
	output := t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(onlinesCount))
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("onlines_refresh"))))

	if onlinesCount > 0 {
		var buttons []telego.InlineKeyboardButton
		for _, online := range onlines {
			buttons = append(buttons, tu.InlineKeyboardButton(online).WithCallbackData(t.encodeQuery("client_get_usage "+online)))
		}
		cols := 0
		if onlinesCount < 21 {
			cols = 2
		} else if onlinesCount < 61 {
			cols = 3
		} else {
			cols = 4
		}
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, tu.InlineKeyboardCols(cols, buttons...)...)
	}

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, keyboard)
	}
}
