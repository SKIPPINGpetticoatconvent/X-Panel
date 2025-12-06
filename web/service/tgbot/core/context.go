package core

import (
	"context"
	"fmt"
	"time"

	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/web/global"
	"x-ui/web/locale"
	"x-ui/web/service"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

// ContextInterface 定义了处理器需要的上下文接口
type ContextInterface interface {
	I18n(key string, params ...string) string
	SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) error
	Answer(chatId int64, msg string, isAdmin bool) error
	AnswerCallbackQuery(id string, message string) error
	GetHostname() string
	GetBot() *telego.Bot
}

// Context 封装了 Telegram bot 的上下文信息
// 包含 telego bot 实例和常用的消息发送、格式化等方法
type Context struct {
	bot         *telego.Bot
	botHandler  *th.BotHandler
	adminIds    []int64
	isRunning   bool
	hostname    string
	hashStorage *global.HashStorage
}

// NewContext 创建新的 Context 实例
func NewContext() *Context {
	return &Context{
		hashStorage: global.NewHashStorage(20 * time.Minute),
	}
}

// SetBot 设置 telego bot 实例
func (c *Context) SetBot(bot *telego.Bot) {
	c.bot = bot
}

// SetHandler 设置 bot handler
func (c *Context) SetHandler(handler *th.BotHandler) {
	c.botHandler = handler
}

// SetHostname 设置主机名
func (c *Context) SetHostname() {
	c.hostname = getHostname()
}

// GetHostname 获取主机名
func (c *Context) GetHostname() string {
	return c.hostname
}

// GetBot 获取 telego Bot 实例
func (c *Context) GetBot() *telego.Bot {
	return c.bot
}

// SetAdminIds 设置管理员 ID 列表
func (c *Context) SetAdminIds(adminIds []int64) {
	c.adminIds = adminIds
}

// GetAdminIds 获取管理员 ID 列表
func (c *Context) GetAdminIds() []int64 {
	return c.adminIds
}

// SetRunning 设置运行状态
func (c *Context) SetRunning(running bool) {
	c.isRunning = running
}

// IsRunning 检查是否正在运行
func (c *Context) IsRunning() bool {
	return c.isRunning
}

// Reply 发送回复消息
func (c *Context) Reply(chatID int64, message string, replyMarkup ...telego.ReplyMarkup) error {
	if !c.isRunning {
		return fmt.Errorf("bot is not running")
	}

	if message == "" {
		logger.Info("[tgbot] message is empty!")
		return nil
	}

	return c.SendMsgToTgbot(chatID, message, replyMarkup...)
}

// SendMsgToTgbot 发送消息到 Telegram
func (c *Context) SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) error {
	if !c.isRunning {
		return fmt.Errorf("bot is not running")
	}

	if msg == "" {
		logger.Info("[tgbot] message is empty!")
		return nil
	}

	var allMessages []string
	limit := 2000

	// 如果消息太长，进行分页
	if len(msg) > limit {
		messages := splitMessage(msg, limit)
		allMessages = messages
	} else {
		allMessages = append(allMessages, msg)
	}

	for n, message := range allMessages {
		params := telego.SendMessageParams{
			ChatID:    tu.ID(chatId),
			Text:      message,
			ParseMode: "HTML",
		}

		// 只在最后一条消息添加回复标记
		if len(replyMarkup) > 0 && n == (len(allMessages)-1) {
			params.ReplyMarkup = replyMarkup[0]
		}

		_, err := c.bot.SendMessage(context.Background(), &params)
		if err != nil {
			logger.Warning("Error sending telegram message:", err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// SendMsgToTgbotAdmins 发送消息给所有管理员
func (c *Context) SendMsgToTgbotAdmins(msg string, replyMarkup ...telego.ReplyMarkup) error {
	if len(replyMarkup) > 0 {
		for _, adminId := range c.adminIds {
			if err := c.SendMsgToTgbot(adminId, msg, replyMarkup[0]); err != nil {
				return err
			}
		}
	} else {
		for _, adminId := range c.adminIds {
			if err := c.SendMsgToTgbot(adminId, msg); err != nil {
				return err
			}
		}
	}
	return nil
}

// Answer 发送带菜单的回复
func (c *Context) Answer(chatId int64, msg string, isAdmin bool) error {
	replyMarkup := c.buildMainKeyboard(isAdmin)
	return c.SendMsgToTgbot(chatId, msg, replyMarkup)
}

// EditMessage 编辑消息
func (c *Context) EditMessage(chatId int64, messageID int, text string, inlineKeyboard ...*telego.InlineKeyboardMarkup) error {
	params := telego.EditMessageTextParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
	}

	if len(inlineKeyboard) > 0 {
		params.ReplyMarkup = inlineKeyboard[0]
	}

	_, err := c.bot.EditMessageText(context.Background(), &params)
	if err != nil {
		logger.Warning("Failed to edit message:", err)
	}
	return err
}

// DeleteMessage 删除消息
func (c *Context) DeleteMessage(chatId int64, messageID int) error {
	params := telego.DeleteMessageParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
	}

	err := c.bot.DeleteMessage(context.Background(), &params)
	if err != nil {
		logger.Warning("Failed to delete message:", err)
	} else {
		logger.Info("Message deleted successfully")
	}
	return err
}

// AnswerCallbackQuery 回答回调查询
func (c *Context) AnswerCallbackQuery(id string, message string) error {
	params := telego.AnswerCallbackQueryParams{
		CallbackQueryID: id,
		Text:            message,
	}

	err := c.bot.AnswerCallbackQuery(context.Background(), &params)
	if err != nil {
		logger.Warning("Failed to answer callback query:", err)
	}
	return err
}

// EditMessageReplyMarkup 编辑消息的回复标记
func (c *Context) EditMessageReplyMarkup(chatId int64, messageID int, inlineKeyboard *telego.InlineKeyboardMarkup) error {
	params := telego.EditMessageReplyMarkupParams{
		ChatID:      tu.ID(chatId),
		MessageID:   messageID,
		ReplyMarkup: inlineKeyboard,
	}

	_, err := c.bot.EditMessageReplyMarkup(context.Background(), &params)
	if err != nil {
		logger.Warning("Failed to edit message reply markup:", err)
	}
	return err
}

// SendMessageWithDelete 发送消息后自动删除
func (c *Context) SendMessageWithDelete(chatId int64, msg string, delayInSeconds int, replyMarkup ...telego.ReplyMarkup) error {
	var replyMarkupParam telego.ReplyMarkup
	if len(replyMarkup) > 0 {
		replyMarkupParam = replyMarkup[0]
	}

	sentMsg, err := c.bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      tu.ID(chatId),
		Text:        msg,
		ReplyMarkup: replyMarkupParam,
	})

	if err != nil {
		logger.Warning("Failed to send message:", err)
		return err
	}

	// 在后台协程中删除消息
	go func() {
		time.Sleep(time.Duration(delayInSeconds) * time.Second)
		c.DeleteMessage(chatId, sentMsg.MessageID)
	}()

	return nil
}

// CheckAdmin 检查用户是否为管理员
func (c *Context) CheckAdmin(tgId int64) bool {
	for _, adminId := range c.adminIds {
		if adminId == tgId {
			return true
		}
	}
	return false
}

// EncodeQuery 编码查询字符串
func (c *Context) EncodeQuery(query string) string {
	// 如果查询长度不超过64字符，直接返回
	if len(query) <= 64 {
		return query
	}

	// 否则使用哈希存储
	return c.hashStorage.SaveHash(query)
}

// DecodeQuery 解码查询字符串
func (c *Context) DecodeQuery(query string) (string, error) {
	if !c.hashStorage.IsMD5(query) {
		return query, nil
	}

	decoded, exists := c.hashStorage.GetValue(query)
	if !exists {
		return "", common.NewError("hash not found in storage!")
	}

	return decoded, nil
}

// I18n 国际化文本
func (c *Context) I18n(name string, params ...string) string {
	return locale.I18n(locale.Bot, name, params...)
}

// FormatTraffic 格式化流量显示
func (c *Context) FormatTraffic(bytes int64) string {
	return common.FormatTraffic(bytes)
}

// FormatServerUsageInfo 格式化服务器使用信息
func (c *Context) FormatServerUsageInfo(lastStatus *service.Status, onlines []string) string {
	info := ""
	info += c.I18n("tgbot.messages.hostname", "Hostname=="+c.hostname)
	info += c.I18n("tgbot.messages.serverUpTime", "UpTime=="+fmt.Sprintf("%d", lastStatus.Uptime/86400), "Unit=="+c.I18n("tgbot.days"))
	info += c.I18n("tgbot.messages.serverLoad", "Load1=="+fmt.Sprintf("%.2f", lastStatus.Loads[0]), "Load2=="+fmt.Sprintf("%.2f", lastStatus.Loads[1]), "Load3=="+fmt.Sprintf("%.2f", lastStatus.Loads[2]))
	info += c.I18n("tgbot.messages.serverMemory", "Current=="+c.FormatTraffic(int64(lastStatus.Mem.Current)), "Total=="+c.FormatTraffic(int64(lastStatus.Mem.Total)))
	info += c.I18n("tgbot.messages.onlinesCount", "Count=="+fmt.Sprintf("%d", len(onlines)))
	info += c.I18n("tgbot.messages.tcpCount", "Count=="+fmt.Sprintf("%d", lastStatus.TcpCount))
	info += c.I18n("tgbot.messages.udpCount", "Count=="+fmt.Sprintf("%d", lastStatus.UdpCount))
	info += c.I18n("tgbot.messages.traffic", "Total=="+c.FormatTraffic(int64(lastStatus.NetTraffic.Sent+lastStatus.NetTraffic.Recv)), "Upload=="+c.FormatTraffic(int64(lastStatus.NetTraffic.Sent)), "Download=="+c.FormatTraffic(int64(lastStatus.NetTraffic.Recv)))
	info += c.I18n("tgbot.messages.xrayStatus", "State=="+fmt.Sprintf("%v", lastStatus.Xray.State))

	return info
}

// buildMainKeyboard 构建主菜单键盘
func (c *Context) buildMainKeyboard(isAdmin bool) *telego.InlineKeyboardMarkup {
	if isAdmin {
		return tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.serverUsage")).WithCallbackData(c.EncodeQuery("get_usage")),
				tu.InlineKeyboardButton("♻️ 重启面板").WithCallbackData(c.EncodeQuery("restart_panel")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.SortedTrafficUsageReport")).WithCallbackData(c.EncodeQuery("get_sorted_traffic_usage_report")),
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.ResetAllTraffics")).WithCallbackData(c.EncodeQuery("reset_all_traffics")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.dbBackup")).WithCallbackData(c.EncodeQuery("get_backup")),
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.getBanLogs")).WithCallbackData(c.EncodeQuery("get_banlogs")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.getInbounds")).WithCallbackData(c.EncodeQuery("inbounds")),
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.depleteSoon")).WithCallbackData(c.EncodeQuery("deplete_soon")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.commands")).WithCallbackData(c.EncodeQuery("commands")),
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.onlines")).WithCallbackData(c.EncodeQuery("onlines")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.allClients")).WithCallbackData(c.EncodeQuery("get_inbounds")),
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.addClient")).WithCallbackData(c.EncodeQuery("add_client")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.oneClick")).WithCallbackData(c.EncodeQuery("oneclick_options")),
				tu.InlineKeyboardButton(c.I18n("tgbot.buttons.subconverter")).WithCallbackData(c.EncodeQuery("subconverter_install")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("🆕 Xray 版本管理").WithCallbackData(c.EncodeQuery("xrayversion")),
			),
		)
	}

	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(c.I18n("tgbot.buttons.clientUsage")).WithCallbackData(c.EncodeQuery("client_traffic")),
			tu.InlineKeyboardButton(c.I18n("tgbot.buttons.commands")).WithCallbackData(c.EncodeQuery("client_commands")),
		),
	)
}

// splitMessage 将长消息分割为多个部分
func splitMessage(msg string, limit int) []string {
	messages := make([]string, 0)
	parts := splitByDoubleNewline(msg)

	var currentMessage string
	for _, part := range parts {
		if len(currentMessage)+len(part) <= limit {
			if currentMessage == "" {
				currentMessage = part
			} else {
				currentMessage += "\r\n\r\n" + part
			}
		} else {
			if currentMessage != "" {
				messages = append(messages, currentMessage)
			}
			currentMessage = part
		}
	}

	if currentMessage != "" {
		messages = append(messages, currentMessage)
	}

	return messages
}

// splitByDoubleNewline 按双换行符分割文本
func splitByDoubleNewline(text string) []string {
	return []string{text} // 简化实现，直接返回原文本
}

// getHostname 获取主机名
func getHostname() string {
	// 这里需要根据实际环境获取主机名
	// 暂时返回一个默认值
	return "X-Panel"
}
