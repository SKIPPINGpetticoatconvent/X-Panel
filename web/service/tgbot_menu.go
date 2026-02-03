package service

import (
	"fmt"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// ================== 主菜单 ==================

func (t *Tgbot) SendAnswer(chatId int64, msg string, isAdmin bool) {
	numericKeyboard := tu.InlineKeyboard(
		// ━━━━━━━━━━ 🏠 主菜单 (两级菜单) ━━━━━━━━━━
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📊 系统监控").WithCallbackData(t.encodeQuery("menu_monitor")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("👥 用户管理").WithCallbackData(t.encodeQuery("menu_users")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🛠 系统维护").WithCallbackData(t.encodeQuery("menu_maintenance")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚙️ 高级设置").WithCallbackData(t.encodeQuery("menu_advanced")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 关闭菜单").WithCallbackData(t.encodeQuery("close_keyboard")),
		),
	)
	numericKeyboardClient := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clientUsage")).WithCallbackData(t.encodeQuery("client_traffic")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("client_commands")),
		),
	)

	var ReplyMarkup telego.ReplyMarkup
	if isAdmin {
		ReplyMarkup = numericKeyboard
	} else {
		ReplyMarkup = numericKeyboardClient
	}
	t.SendMsgToTgbot(chatId, msg, ReplyMarkup)
}

// ================== 两级菜单 - 子菜单函数 ==================

// showMenuMonitor 显示系统监控子菜单
func (t *Tgbot) showMenuMonitor(chatId int64, messageId int) {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📈 系统状态").WithCallbackData(t.encodeQuery("get_usage")),
			tu.InlineKeyboardButton("📊 流量报告").WithCallbackData(t.encodeQuery("get_sorted_traffic_usage_report")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("menu_main")),
		),
	)
	t.editMessageCallbackTgBot(chatId, messageId, keyboard)
}

// showMenuUsers 显示用户管理子菜单
func (t *Tgbot) showMenuUsers(chatId int64, messageId int) {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("👥 所有客户").WithCallbackData(t.encodeQuery("get_inbounds")),
			tu.InlineKeyboardButton("➕ 添加客户").WithCallbackData(t.encodeQuery("add_client")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📶 在线用户").WithCallbackData(t.encodeQuery("onlines")),
			tu.InlineKeyboardButton("📋 入站列表").WithCallbackData(t.encodeQuery("inbounds")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📋 批量复制链接").WithCallbackData(t.encodeQuery("copy_all_links")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🚀 一键配置").WithCallbackData(t.encodeQuery("oneclick_options")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("menu_main")),
		),
	)
	t.editMessageCallbackTgBot(chatId, messageId, keyboard)
}

// showMenuMaintenance 显示系统维护子菜单
func (t *Tgbot) showMenuMaintenance(chatId int64, messageId int) {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("♻️ 重启面板").WithCallbackData(t.encodeQuery("restart_panel")),
			tu.InlineKeyboardButton("🔄 重置流量").WithCallbackData(t.encodeQuery("reset_all_traffics")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📥 备份数据").WithCallbackData(t.encodeQuery("get_backup")),
			tu.InlineKeyboardButton("🔥 防火墙").WithCallbackData(t.encodeQuery("firewall_menu")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("menu_main")),
		),
	)
	t.editMessageCallbackTgBot(chatId, messageId, keyboard)
}

// showMenuAdvanced 显示高级设置子菜单
func (t *Tgbot) showMenuAdvanced(chatId int64, messageId int) {
	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚡ 机器优化").WithCallbackData(t.encodeQuery("machine_optimization")),
			tu.InlineKeyboardButton("🌍 更新Geo").WithCallbackData(t.encodeQuery("update_geodata_ask")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🆕 Xray版本").WithCallbackData(t.encodeQuery("xrayversion")),
			tu.InlineKeyboardButton("🔄 程序更新").WithCallbackData(t.encodeQuery("check_panel_update")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📝 日志设置").WithCallbackData(t.encodeQuery("log_settings")),
			tu.InlineKeyboardButton("📝 封禁日志").WithCallbackData(t.encodeQuery("get_banlogs")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❓ 命令帮助").WithCallbackData(t.encodeQuery("commands")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("menu_main")),
		),
	)
	t.editMessageCallbackTgBot(chatId, messageId, keyboard)
}

// ================== 日志设置菜单 ==================

// showLogSettings 显示日志设置菜单
func (t *Tgbot) showLogSettings(chatId int64) {
	tgForwardEnabled, err := t.settingService.GetTgLogForwardEnabled()
	if err != nil {
		t.SendMsgToTgbot(chatId, "❌ 获取 TG 转发状态失败")
		return
	}
	localLogEnabled, err := t.settingService.GetLocalLogEnabled()
	if err != nil {
		localLogEnabled = false
	}
	logLevel, err := t.settingService.GetTgLogLevel()
	if err != nil {
		logLevel = "warn"
	}

	tgForwardStatus := "❌"
	if tgForwardEnabled {
		tgForwardStatus = "✅"
	}
	localLogStatus := "❌"
	if localLogEnabled {
		localLogStatus = "✅"
	}

	message := fmt.Sprintf("📝 <b>日志设置</b>\n\n"+
		"📤 TG 转发: %s\n"+
		"💾 本地日志: %s\n"+
		"🔧 日志级别: %s\n\n"+
		"选择要切换的设置:",
		tgForwardStatus, localLogStatus, logLevel)

	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("📤 TG 转发: %s", tgForwardStatus)).WithCallbackData(t.encodeQuery("toggle_log_forward")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("💾 本地日志: %s", localLogStatus)).WithCallbackData(t.encodeQuery("toggle_local_log")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("🔧 日志级别: %s", logLevel)).WithCallbackData(t.encodeQuery("cycle_log_level")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔴 仅错误").WithCallbackData(t.encodeQuery("set_log_level error")),
			tu.InlineKeyboardButton("⚠️ 警告及以上").WithCallbackData(t.encodeQuery("set_log_level warn")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("ℹ️ 全部信息").WithCallbackData(t.encodeQuery("set_log_level info")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("back_to_main")),
		),
	)

	t.SendMsgToTgbot(chatId, message, keyboard)
}

// ================== 一键配置菜单 ==================

// sendOneClickOptions 发送【一键配置】的选项按钮给用户
func (t *Tgbot) sendOneClickOptions(chatId int64) {
	categoryKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔗 Direct Connection (直连)").WithCallbackData(t.encodeQuery("oneclick_category_direct")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 Relay (中转)").WithCallbackData(t.encodeQuery("oneclick_category_relay")),
		),
	)
	t.SendMsgToTgbot(chatId, "请选择【一键配置】类型：", categoryKeyboard)
}

// sendRelayOptions 显示中转类别的具体配置选项
func (t *Tgbot) sendRelayOptions(chatId int64) {
	relayKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🛡️ Vless Encryption + XHTTP + TLS").WithCallbackData(t.encodeQuery("oneclick_tls")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🌀 Switch + Vision Seed (开发中)").WithCallbackData(t.encodeQuery("oneclick_switch_vision")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("oneclick_options")),
		),
	)
	t.SendMsgToTgbot(chatId, "【中转】类别 - 适合需要中转的场景：\n\n🛡️ Vless Encryption + XHTTP + TLS: 加密传输，可配合CDN\n🌀 Switch + Vision Seed: 特殊配置（开发中）", relayKeyboard)
}

// sendDirectOptions 显示直连类别的具体配置选项
func (t *Tgbot) sendDirectOptions(chatId int64) {
	directKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🚀 Vless + TCP + Reality").WithCallbackData(t.encodeQuery("oneclick_reality")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚡ Vless + XHTTP + Reality").WithCallbackData(t.encodeQuery("oneclick_xhttp_reality")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("oneclick_options")),
		),
	)
	t.SendMsgToTgbot(chatId, "【直连】类别 - 适合优化线路直连：\n\n🚀 Vless + TCP + Reality: 高性能直连，优秀兼容性\n⚡ Vless + XHTTP + Reality: 新型传输，更佳隐蔽性", directKeyboard)
}
