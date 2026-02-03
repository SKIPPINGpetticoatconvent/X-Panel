package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/xray"

	tu "github.com/mymmrac/telego/telegoutil"
)

// ================== 服务器状态 ==================

func (t *Tgbot) getServerUsage(chatId int64, messageID ...int) string {
	info := t.prepareServerUsageInfo()

	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("usage_refresh"))))

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], info, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, info, keyboard)
	}

	return info
}

func (t *Tgbot) prepareServerUsageInfo() string {
	ipv4, ipv6 := "", ""

	// get latest status of server
	t.lastStatus = t.serverService.GetStatus(t.lastStatus)
	onlines := t.xrayService.GetOnlineClients()

	// get ip address
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("net.Interfaces failed, err: ", err.Error())
		ipv4 = t.I18nBot("tgbot.unknown")
	} else {
		for i := 0; i < len(netInterfaces); i++ {
			if (netInterfaces[i].Flags & net.FlagUp) != 0 {
				addrs, _ := netInterfaces[i].Addrs()

				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ipv4 += ipnet.IP.String() + " "
						} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLinkLocalUnicast() {
							ipv6 += ipnet.IP.String() + " "
						}
					}
				}
			}
		}
	}

	var sb strings.Builder

	sb.WriteString(t.I18nBot("tgbot.messages.serverReportTitle") + "\n\n")

	// 基础信息
	sb.WriteString(t.I18nBot("tgbot.messages.headerBasic") + "\n")
	sb.WriteString("───────────────\n")
	sb.WriteString(t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname) + "\n")
	sb.WriteString(t.I18nBot("tgbot.messages.version", "Version=="+config.GetVersion()) + "\n")
	sb.WriteString(t.I18nBot("tgbot.messages.xrayVersion", "XrayVersion=="+fmt.Sprint(t.lastStatus.Xray.Version)) + "\n")
	sb.WriteString(t.I18nBot("tgbot.messages.serverUpTime", "UpTime=="+strconv.FormatUint(t.lastStatus.Uptime/86400, 10), "Unit=="+t.I18nBot("tgbot.days")) + "\n\n")

	// 资源监控
	sb.WriteString(t.I18nBot("tgbot.messages.headerResources") + "\n")
	sb.WriteString("───────────────\n")
	sb.WriteString(t.I18nBot("tgbot.messages.serverLoad", "Load1=="+strconv.FormatFloat(t.lastStatus.Loads[0], 'f', 2, 64), "Load2=="+strconv.FormatFloat(t.lastStatus.Loads[1], 'f', 2, 64), "Load3=="+strconv.FormatFloat(t.lastStatus.Loads[2], 'f', 2, 64)) + "\n")
	//nolint:gosec
	sb.WriteString(t.I18nBot("tgbot.messages.serverMemory", "Current=="+common.FormatTraffic(int64(t.lastStatus.Mem.Current)), "Total=="+common.FormatTraffic(int64(t.lastStatus.Mem.Total))) + "\n")
	sb.WriteString(t.I18nBot("tgbot.messages.xrayStatus", "State=="+fmt.Sprint(t.lastStatus.Xray.State)) + "\n\n")

	// 流量统计
	sb.WriteString(t.I18nBot("tgbot.messages.headerTraffic") + "\n")
	sb.WriteString("───────────────\n")
	//nolint:gosec
	sb.WriteString(t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), "Upload=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), "Download=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv))) + "\n\n")

	// 连接详情
	sb.WriteString(t.I18nBot("tgbot.messages.headerNetwork") + "\n")
	sb.WriteString("───────────────\n")
	if ipv4 != "" {
		sb.WriteString(t.I18nBot("tgbot.messages.ipv4", "IPv4=="+"<tg-spoiler>"+ipv4+"</tg-spoiler>") + "\n")
	}
	if ipv6 != "" {
		sb.WriteString(t.I18nBot("tgbot.messages.ipv6", "IPv6=="+"<tg-spoiler>"+ipv6+"</tg-spoiler>") + "\n")
	}
	sb.WriteString(t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(len(onlines))) + "\n")
	sb.WriteString(t.I18nBot("tgbot.messages.tcpCount", "Count=="+strconv.Itoa(t.lastStatus.TcpCount)) + "\n")
	sb.WriteString(t.I18nBot("tgbot.messages.udpCount", "Count=="+strconv.Itoa(t.lastStatus.UdpCount)) + "\n")

	return sb.String()
}

// ================== 登录通知 ==================

func (t *Tgbot) UserLoginNotify(username string, password string, ip string, time string, status LoginStatus) {
	if !t.IsRunning() {
		return
	}

	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed, invalid info!")
		return
	}

	loginNotifyEnabled, err := t.settingService.GetTgBotLoginNotify()
	if err != nil || !loginNotifyEnabled {
		return
	}

	msg := ""
	switch status {
	case LoginSuccess:
		msg += t.I18nBot("tgbot.messages.loginSuccess")
		msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	case LoginFail:
		msg += t.I18nBot("tgbot.messages.loginFailed")
		msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
		msg += t.I18nBot("tgbot.messages.password", "Password=="+"<tg-spoiler>"+password+"</tg-spoiler>")
	}
	msg += t.I18nBot("tgbot.messages.username", "Username=="+username)
	msg += t.I18nBot("tgbot.messages.ip", "IP=="+"<tg-spoiler>"+ip+"</tg-spoiler>")
	msg += t.I18nBot("tgbot.messages.time", "Time=="+time)
	t.SendMsgToTgbotAdmins(msg)
}

// ================== 备份与日志 ==================

func (t *Tgbot) sendBackup(chatId int64) {
	output := t.I18nBot("tgbot.messages.backupTime", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	t.SendMsgToTgbot(chatId, output)

	// Update by manually trigger a checkpoint operation
	err := database.Checkpoint()
	if err != nil {
		logger.Error("Error in trigger a checkpoint operation: ", err)
	}

	file, err := os.Open(config.GetDBPath())
	if err == nil {
		document := tu.Document(
			tu.ID(chatId),
			tu.File(file),
		)
		_, err = bot.SendDocument(context.Background(), document)
		if err != nil {
			logger.Error("Error in uploading backup: ", err)
		}
	} else {
		logger.Error("Error in opening db file for backup: ", err)
	}

	file, err = os.Open(xray.GetConfigPath())
	if err == nil {
		document := tu.Document(
			tu.ID(chatId),
			tu.File(file),
		)
		_, err = bot.SendDocument(context.Background(), document)
		if err != nil {
			logger.Error("Error in uploading config.json: ", err)
		}
	} else {
		logger.Error("Error in opening config.json file for backup: ", err)
	}
}

func (t *Tgbot) sendBanLogs(chatId int64, dt bool) {
	if dt {
		output := t.I18nBot("tgbot.messages.datetime", "DateTime=="+time.Now().Format("2006-01-02 15:04:05"))
		t.SendMsgToTgbot(chatId, output)
	}

	file, err := os.Open(xray.GetIPLimitBannedPrevLogPath())
	if err == nil {
		// Check if the file is non-empty before attempting to upload
		fileInfo, _ := file.Stat()
		if fileInfo.Size() > 0 {
			document := tu.Document(
				tu.ID(chatId),
				tu.File(file),
			)
			_, err = bot.SendDocument(context.Background(), document)
			if err != nil {
				logger.Error("Error in uploading IPLimitBannedPrevLog: ", err)
			}
		} else {
			logger.Warning("IPLimitBannedPrevLog file is empty, not uploading.")
		}
		_ = file.Close()
	} else {
		logger.Error("Error in opening IPLimitBannedPrevLog file for backup: ", err)
	}

	file, err = os.Open(xray.GetIPLimitBannedLogPath())
	if err == nil {
		// Check if the file is non-empty before attempting to upload
		fileInfo, _ := file.Stat()
		if fileInfo.Size() > 0 {
			document := tu.Document(
				tu.ID(chatId),
				tu.File(file),
			)
			_, err = bot.SendDocument(context.Background(), document)
			if err != nil {
				logger.Error("Error in uploading IPLimitBannedLog: ", err)
			}
		} else {
			logger.Warning("IPLimitBannedLog file is empty, not uploading.")
		}
		_ = file.Close()
	} else {
		logger.Error("Error in opening IPLimitBannedLog file for backup: ", err)
	}
}

func (t *Tgbot) SendBackupToAdmins() {
	if !t.IsRunning() {
		return
	}
	for _, adminId := range adminIds {
		t.sendBackup(int64(adminId))
	}
}

// ================== 面板更新 ==================

func (t *Tgbot) checkPanelUpdate(chatId int64) {
	// 获取当前版本
	currentVersion := config.GetVersion()

	// 获取最新版本
	latestVersion, err := t.serverService.GetPanelLatestVersion()
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 检查更新失败: %v", err))
		return
	}

	// 比较版本
	if currentVersion == latestVersion {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("✅ 您的面板已经是最新版本！\n\n当前版本: <code>%s</code>", currentVersion))
		return
	}

	// 版本不同，显示更新提示
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 确认更新").WithCallbackData(t.encodeQuery("confirm_panel_update")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("cancel_panel_update")),
		),
	)

	message := fmt.Sprintf(
		"🔄 <b>发现新版本！</b>\n\n"+
			"当前版本: <code>%s</code>\n"+
			"最新版本: <code>%s</code>\n\n"+
			"⚠️ <b>注意：</b> 更新将：\n"+
			"• 自动从 GitHub 拉取最新代码\n"+
			"• 重启面板服务（期间无法访问）\n\n"+
			"是否确认更新？",
		currentVersion, latestVersion)

	t.SendMsgToTgbot(chatId, message, confirmKeyboard)
}
