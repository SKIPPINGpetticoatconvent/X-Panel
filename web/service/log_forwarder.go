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
	forwardLevel    logging.Level // 日志转发级别 (ERROR, WARNING, INFO, DEBUG)
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

	// 获取配置的日志级别，默认 WARNING
	forwardLevel := logging.WARNING
	if levelStr, err := settingService.GetTgLogLevel(); err == nil {
		switch levelStr {
		case "error":
			forwardLevel = logging.ERROR
		case "warn":
			forwardLevel = logging.WARNING
		case "info":
			forwardLevel = logging.INFO
		case "debug":
			forwardLevel = logging.DEBUG
		default:
			forwardLevel = logging.WARNING
		}
	}

	// 针对低配机器（1CPU 1RAM）的优化配置
	// bufferSize: 200 (限制内存占用)
	// workerCount: 1 (限制 CPU 占用)
	// batchSize: 10 (减少网络 I/O 和上下文切换)
	// maxBatchDelay: 10s (减少定时器唤醒频率)
	return &LogForwarder{
		settingService:  settingService,
		telegramService: telegramService,
		isEnabled:       false,
		forwardLevel:    forwardLevel,
		logBuffer:       make(chan *LogMessage, 200), 
		bufferSize:      200,
		workerCount:     1, 
		batchSize:       10, 
		maxBatchDelay:   10 * time.Second, 
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
	if lf.shouldSkipLog(message, formattedLog, level) {
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
		// 注意：在高负载下，这可以防止内存无限增长
		logger.Warning("日志转发缓冲区已满，丢弃日志消息")
	}
}

// shouldSkipLog 判断是否应该跳过转发此日志
// 根据配置的级别转发日志
func (lf *LogForwarder) shouldSkipLog(message, formattedLog string, level logging.Level) bool {
	// 检查级别是否满足转发条件
	if level > lf.forwardLevel {
		return true
	}

	// 定义需要跳过的关键词列表
	// 包含 Telegram Bot 相关、日志转发器自身以及频繁的无意义日志
	skipKeywords := []string{
		"Telegram", "telegram", "bot", "Bot", "SendMsgToTgbot", "SendMessage",
		"LogForwarder", "日志转发",
		"checkpoint", "database", "DB",
	}

	// 遍历检查，避免过多的字符串操作和复杂的逻辑
	for _, keyword := range skipKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
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
	// 根据级别格式化消息，使用清晰的格式和 HTML 标记
	switch logMsg.Level {
	case logging.ERROR:
		return fmt.Sprintf("🚨 <b>ERROR</b>\n<code>%s</code>", logMsg.Message)
	case logging.WARNING:
		return fmt.Sprintf("⚠️ <b>WARNING</b>\n<code>%s</code>", logMsg.Message)
	case logging.INFO:
		// INFO 级别只转发重要的消息
		if lf.isImportantInfo(logMsg.Message) {
			return fmt.Sprintf("ℹ️ <b>INFO</b>\n<code>%s</code>", logMsg.Message)
		}
	case logging.DEBUG:
		// DEBUG 级别使用简洁格式
		return fmt.Sprintf("🐛 <b>DEBUG</b>\n<code>%s</code>", logMsg.Message)
	}

	return ""
}

// isImportantInfo 判断 INFO 级别消息是否重要
func (lf *LogForwarder) isImportantInfo(message string) bool {
	// 避免使用 strings.ToLower 以减少内存分配
	// 包含常见的大小写变体
	importantKeywords := []string{
		"started", "Started",
		"stopped", "Stopped",
		"running", "Running",
		"failed", "Failed",
		"error", "Error",
		"restart", "Restart",
		"shutdown", "Shutdown",
		"connected", "Connected",
		"disconnected", "Disconnected",
		"login", "Login",
		"logout", "Logout",
	}

	for _, keyword := range importantKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}

	return false
}

// SetForwardLevel 设置日志转发级别
func (lf *LogForwarder) SetForwardLevel(level logging.Level) {
	lf.mu.Lock()
	defer lf.mu.Unlock()
	lf.forwardLevel = level
	logger.Infof("日志转发级别已设置为: %v", level)
}

// UpdateConfig 更新配置（动态启用/禁用和级别）
func (lf *LogForwarder) UpdateConfig() {
	enabled, err := lf.settingService.GetTgLogForwardEnabled()
	if err != nil {
		logger.Warningf("获取日志转发配置失败: %v", err)
		return
	}

	// 获取新的级别
	var newLevel logging.Level = logging.WARNING
	if levelStr, err := lf.settingService.GetTgLogLevel(); err == nil {
		switch levelStr {
		case "error":
			newLevel = logging.ERROR
		case "warn":
			newLevel = logging.WARNING
		case "info":
			newLevel = logging.INFO
		case "debug":
			newLevel = logging.DEBUG
		default:
			newLevel = logging.WARNING
		}
	}

	lf.mu.Lock()
	currentEnabled := lf.isEnabled
	currentLevel := lf.forwardLevel
	lf.mu.Unlock()

	// 更新级别
	if currentLevel != newLevel {
		lf.SetForwardLevel(newLevel)
	}

	// 更新启用状态
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
