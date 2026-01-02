package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"x-ui/logger"

	"github.com/op/go-logging"
)

// LogForwarder 日志转发器，将日志转发到 Telegram Bot
type LogForwarder struct {
	settingService  *SettingService
	telegramService TelegramService
	isEnabled       bool
	logBuffer       chan *LogMessage
	bufferSize      int
	workerCount     int
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex
}

// LogMessage 表示要转发的日志消息
type LogMessage struct {
	Level       logging.Level
	Message     string
	Formatted   string
	Timestamp   time.Time
}

// NewLogForwarder 创建新的日志转发器
func NewLogForwarder(settingService *SettingService, telegramService TelegramService) *LogForwarder {
	ctx, cancel := context.WithCancel(context.Background())

	return &LogForwarder{
		settingService:  settingService,
		telegramService: telegramService,
		isEnabled:       false,
		logBuffer:       make(chan *LogMessage, 500), // 缓冲区大小为500，节省内存
		bufferSize:      500,
		workerCount:     1, // 1个工作协程，减少CPU占用
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动日志转发器
func (lf *LogForwarder) Start() error {
	lf.mu.Lock()
	defer lf.mu.Unlock()

	if lf.isEnabled {
		return nil // 已经启动
	}

	// 检查配置是否启用
	enabled, err := lf.settingService.GetTgLogForwardEnabled()
	if err != nil {
		logger.Warningf("获取日志转发配置失败: %v", err)
		return err
	}

	// 检查 Telegram Bot 是否可用
	if !lf.telegramService.IsRunning() {
		logger.Warning("Telegram Bot 未运行，日志转发功能将被禁用")
		return nil
	}

	lf.isEnabled = enabled
	if !lf.isEnabled {
		logger.Info("日志转发功能已禁用")
		return nil
	}

	// 注册为日志监听器
	logger.AddLogListener(lf)

	// 启动工作协程
	for i := 0; i < lf.workerCount; i++ {
		lf.wg.Add(1)
		go lf.worker(i)
	}

	logger.Info("日志转发器已启动")
	return nil
}

// Stop 停止日志转发器
func (lf *LogForwarder) Stop() error {
	lf.mu.Lock()
	defer lf.mu.Unlock()

	if !lf.isEnabled {
		return nil
	}

	// 取消上下文
	lf.cancel()

	// 等待工作协程退出
	done := make(chan struct{})
	go func() {
		lf.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("日志转发器已停止")
	case <-time.After(5 * time.Second):
		logger.Warning("日志转发器停止超时")
	}

	// 移除监听器
	logger.RemoveLogListener(lf)

	lf.isEnabled = false
	return nil
}

// IsEnabled 检查转发器是否启用
func (lf *LogForwarder) IsEnabled() bool {
	lf.mu.RLock()
	defer lf.mu.RUnlock()
	return lf.isEnabled
}

// OnLog 实现 LogListener 接口，接收日志消息
func (lf *LogForwarder) OnLog(level logging.Level, message string, formattedLog string) {
	lf.mu.RLock()
	enabled := lf.isEnabled
	lf.mu.RUnlock()

	if !enabled {
		return
	}

	// 过滤不需要转发的日志
	if lf.shouldSkipLog(message, formattedLog) {
		return
	}

	// 创建日志消息
	logMsg := &LogMessage{
		Level:     level,
		Message:   message,
		Formatted: formattedLog,
		Timestamp: time.Now(),
	}

	// 非阻塞发送到缓冲区
	select {
	case lf.logBuffer <- logMsg:
		// 发送成功
	default:
		// 缓冲区满，丢弃消息
		logger.Warning("日志转发缓冲区已满，丢弃日志消息")
	}
}

// shouldSkipLog 判断是否应该跳过转发此日志
func (lf *LogForwarder) shouldSkipLog(message, formattedLog string) bool {
	// 获取配置的日志级别
	logLevel, err := lf.settingService.GetTgLogLevel()
	if err != nil {
		logger.Warningf("获取日志级别配置失败: %v", err)
		return true // 默认跳过以避免过多发送
	}

	// 根据配置的级别过滤
	switch strings.ToLower(logLevel) {
	case "error":
		// 只转发 ERROR
		if !strings.Contains(formattedLog, "ERROR") {
			return true
		}
	case "warn":
		// 转发 WARNING 和 ERROR
		if !strings.Contains(formattedLog, "WARNING") && !strings.Contains(formattedLog, "ERROR") {
			return true
		}
	case "info":
		// 转发 INFO, WARNING 和 ERROR，但 INFO 需要进一步检查重要性
		if !strings.Contains(formattedLog, "INFO") && !strings.Contains(formattedLog, "WARNING") && !strings.Contains(formattedLog, "ERROR") {
			return true
		}
	case "debug":
		// 转发所有级别（但代码中 DEBUG 被跳过）
		// 继续检查其他条件
	default:
		// 未知级别，默认跳过 INFO 和 DEBUG，只转发 WARNING 和 ERROR
		if !strings.Contains(formattedLog, "WARNING") && !strings.Contains(formattedLog, "ERROR") {
			return true
		}
	}

	// 跳过 DEBUG 级别日志（无论配置如何）
	if strings.Contains(formattedLog, "DEBUG") {
		return true
	}

	// 跳过与 Telegram Bot 相关的日志，避免死循环
	if strings.Contains(message, "Telegram") ||
		strings.Contains(message, "telegram") ||
		strings.Contains(message, "bot") ||
		strings.Contains(message, "Bot") ||
		strings.Contains(message, "SendMsgToTgbot") ||
		strings.Contains(message, "SendMessage") {
		return true
	}

	// 跳过与日志转发器本身相关的日志
	if strings.Contains(message, "LogForwarder") ||
		strings.Contains(message, "日志转发") {
		return true
	}

	// 跳过一些频繁的、无意义的日志
	if strings.Contains(message, "checkpoint") ||
		strings.Contains(message, "database") ||
		strings.Contains(message, "DB") {
		return true
	}

	return false
}

// worker 工作协程，处理日志转发
func (lf *LogForwarder) worker(id int) {
	defer lf.wg.Done()

	logger.Infof("日志转发工作协程 %d 已启动", id)

	for {
		select {
		case <-lf.ctx.Done():
			logger.Infof("日志转发工作协程 %d 已停止", id)
			return
		case logMsg := <-lf.logBuffer:
			lf.forwardLog(logMsg)
		}
	}
}

// forwardLog 执行实际的日志转发
func (lf *LogForwarder) forwardLog(logMsg *LogMessage) {
	// 检查 Telegram Bot 状态
	if !lf.telegramService.IsRunning() {
		return
	}

	// 格式化消息
	message := lf.formatLogMessage(logMsg)
	if message == "" {
		return
	}

	// 发送消息（TelegramService 应该内部处理超时）
	err := lf.telegramService.SendMessage(message)
	if err != nil {
		// 只记录错误，不再次触发日志转发，避免死循环
		// 使用 fmt.Println 而不是 logger 来避免递归
		fmt.Printf("日志转发失败: %v\n", err)
	}
}

// formatLogMessage 格式化日志消息
func (lf *LogForwarder) formatLogMessage(logMsg *LogMessage) string {
	// 只转发 ERROR、WARNING 和 INFO 级别
	switch logMsg.Level {
	case logging.ERROR:
		return fmt.Sprintf("🚨 <b>ERROR</b>\n%s", logMsg.Formatted)
	case logging.WARNING:
		return fmt.Sprintf("⚠️ <b>WARNING</b>\n%s", logMsg.Formatted)
	case logging.INFO:
		// INFO 级别只转发重要的消息
		if lf.isImportantInfo(logMsg.Message) {
			return fmt.Sprintf("ℹ️ <b>INFO</b>\n%s", logMsg.Formatted)
		}
	}

	return ""
}

// isImportantInfo 判断 INFO 级别消息是否重要
func (lf *LogForwarder) isImportantInfo(message string) bool {
	importantKeywords := []string{
		"started",
		"stopped",
		"running",
		"failed",
		"error",
		"restart",
		"shutdown",
		"connected",
		"disconnected",
		"login",
		"logout",
	}

	messageLower := strings.ToLower(message)
	for _, keyword := range importantKeywords {
		if strings.Contains(messageLower, keyword) {
			return true
		}
	}

	return false
}

// UpdateConfig 更新配置（动态启用/禁用）
func (lf *LogForwarder) UpdateConfig() {
	enabled, err := lf.settingService.GetTgLogForwardEnabled()
	if err != nil {
		logger.Warningf("获取日志转发配置失败: %v", err)
		return
	}

	lf.mu.Lock()
	currentEnabled := lf.isEnabled
	lf.mu.Unlock()

	if enabled != currentEnabled {
		if enabled {
			logger.Info("启用日志转发功能")
			lf.Start()
		} else {
			logger.Info("禁用日志转发功能")
			lf.Stop()
		}
	}
}