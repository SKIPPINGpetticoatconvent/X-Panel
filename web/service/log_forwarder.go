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
	batchSize       int           // 批量大小，达到此数量立即发送
	maxBatchDelay   time.Duration // 最大批量延迟，定时强制发送
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
		batchSize:       5, // 每5条日志批量发送一次
		maxBatchDelay:   10 * time.Second, // 最长等待10秒后强制发送
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

	// 检查 Telegram Bot 是否可用（自动接管）
	if !lf.telegramService.IsRunning() {
		logger.Warning("Telegram Bot 未运行，日志转发功能将被禁用")
		return nil
	}

	// 检查配置是否启用（保留用户控制）
	enabled, err := lf.settingService.GetTgLogForwardEnabled()
	if err != nil {
		logger.Warningf("获取日志转发配置失败: %v", err)
		// 如果获取配置失败，默认启用日志转发（自动接管）
		lf.isEnabled = true
	} else if !enabled {
		logger.Info("日志转发功能已手动禁用")
		return nil
	} else {
		lf.isEnabled = true
	}

	// 注册为日志监听器
	logger.AddLogListener(lf)

	// 启动工作协程
	for i := 0; i < lf.workerCount; i++ {
		lf.wg.Add(1)
		go lf.worker(i)
	}

	logger.Info("日志转发器已自动启动")
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
// 分类处理策略：只推送 Error/Warning，Info/Debug 记录在缓冲区供 /logs 查询
func (lf *LogForwarder) shouldSkipLog(message, formattedLog string) bool {
	// 始终只转发 ERROR 和 WARNING 级别（分类处理）
	if !strings.Contains(formattedLog, "ERROR") && !strings.Contains(formattedLog, "WARNING") {
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

// worker 工作协程，处理日志转发（批量模式）
func (lf *LogForwarder) worker(id int) {
	defer lf.wg.Done()

	logger.Infof("日志转发工作协程 %d 已启动", id)

	batch := make([]*LogMessage, 0, lf.batchSize)
	ticker := time.NewTicker(lf.maxBatchDelay)
	defer ticker.Stop()

	for {
		select {
		case <-lf.ctx.Done():
			logger.Infof("日志转发工作协程 %d 已停止", id)
			// 在退出前发送剩余的日志
			if len(batch) > 0 {
				lf.flushLogs(batch)
			}
			return

		case logMsg := <-lf.logBuffer:
			batch = append(batch, logMsg)
			if len(batch) >= lf.batchSize {
				lf.flushLogs(batch)
				batch = batch[:0] // 重置批次
				ticker.Reset(lf.maxBatchDelay) // 重置定时器
			}

		case <-ticker.C:
			if len(batch) > 0 {
				lf.flushLogs(batch)
				batch = batch[:0] // 重置批次
			}
			ticker.Reset(lf.maxBatchDelay) // 重置定时器
		}
	}
}

// flushLogs 批量发送日志消息
func (lf *LogForwarder) flushLogs(batch []*LogMessage) {
	if len(batch) == 0 {
		return
	}

	// 检查 Telegram Bot 状态
	if !lf.telegramService.IsRunning() {
		return
	}

	// 合并批量日志消息
	messages := make([]string, 0, len(batch))
	for _, logMsg := range batch {
		message := lf.formatLogMessage(logMsg)
		if message != "" {
			messages = append(messages, message)
		}
	}

	if len(messages) == 0 {
		return
	}

	// 如果只有一条消息，直接发送
	if len(messages) == 1 {
		err := lf.telegramService.SendMessage(messages[0])
		if err != nil {
			fmt.Printf("日志转发失败: %v\n", err)
		}
		return
	}

	// 多条消息，合并成一条发送
	combinedMessage := strings.Join(messages, "\n\n---\n\n")
	err := lf.telegramService.SendMessage(combinedMessage)
	if err != nil {
		fmt.Printf("批量日志转发失败: %v\n", err)
	}
}

// forwardLog 执行实际的日志转发（保留用于兼容性，但现在主要使用 flushLogs）
func (lf *LogForwarder) forwardLog(logMsg *LogMessage) {
	lf.flushLogs([]*LogMessage{logMsg})
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