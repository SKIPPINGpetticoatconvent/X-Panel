package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

// LogListener 定义日志监听器接口
type LogListener interface {
	OnLog(level Level, message string, formattedLog string)
}

// LogEntry 表示单个日志条目
type LogEntry struct {
	Time    time.Time
	Level   Level
	Message string
}

// ListenerBackend 管理日志监听器
type ListenerBackend struct {
	listeners []LogListener
	mu        sync.RWMutex
}

// NewListenerBackend 创建新的监听后端
func NewListenerBackend() *ListenerBackend {
	return &ListenerBackend{
		listeners: make([]LogListener, 0),
	}
}

// AddListener 添加日志监听器
func (b *ListenerBackend) AddListener(listener LogListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners = append(b.listeners, listener)
}

// RemoveListener 移除日志监听器
func (b *ListenerBackend) RemoveListener(listener LogListener) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, l := range b.listeners {
		if l == listener {
			b.listeners = append(b.listeners[:i], b.listeners[i+1:]...)
			break
		}
	}
}

// notifyListeners 通知所有监听器
func (b *ListenerBackend) notifyListeners(level Level, message, formatted string) {
	b.mu.RLock()
	listeners := make([]LogListener, len(b.listeners))
	copy(listeners, b.listeners)
	b.mu.RUnlock()

	for _, listener := range listeners {
		go listener.OnLog(level, message, formatted)
	}
}

// listenerHandler 自定义 slog.Handler，支持日志监听
type listenerHandler struct {
	next    slog.Handler
	backend *ListenerBackend
	level   Level
}

func (h *listenerHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := h.level.ToSlogLevel()
	return level >= minLevel
}

func (h *listenerHandler) Handle(ctx context.Context, r slog.Record) error {
	// 先调用下一个 handler
	if h.next != nil {
		if err := h.next.Handle(ctx, r); err != nil {
			return err
		}
	}

	// 通知监听器
	if h.backend != nil && len(h.backend.listeners) > 0 {
		level := LevelFromSlog(r.Level)
		formatted := fmt.Sprintf("%s %s - %s", r.Time.Format("2006/01/02 15:04:05"), level.String(), r.Message)
		h.backend.notifyListeners(level, r.Message, formatted)
	}

	return nil
}

func (h *listenerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &listenerHandler{
		next:    h.next.WithAttrs(attrs),
		backend: h.backend,
		level:   h.level,
	}
}

func (h *listenerHandler) WithGroup(name string) slog.Handler {
	return &listenerHandler{
		next:    h.next.WithGroup(name),
		backend: h.backend,
		level:   h.level,
	}
}

var (
	slogLogger      *slog.Logger
	logBuffer       []LogEntry
	logBufferMu     sync.RWMutex
	listenerBackend *ListenerBackend
	localLogEnabled bool
	mu              sync.RWMutex
)

func init() {
	InitLogger(INFO, false)
}

// InitLogger 初始化日志系统
func InitLogger(level Level, enabled bool) {
	mu.Lock()
	defer mu.Unlock()

	localLogEnabled = enabled
	listenerBackend = NewListenerBackend()

	var writer io.Writer
	if enabled {
		fmt.Fprintln(os.Stderr, "[Security] Local file logging is enabled. Log file: x-ui.log")
		file, err := os.OpenFile("x-ui.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "无法创建日志文件: %v\n", err)
			writer = os.Stderr
		} else {
			writer = file
		}
	} else {
		writer = os.Stderr
	}

	// 创建 slog handler
	opts := &slog.HandlerOptions{
		Level: level.ToSlogLevel(),
	}
	baseHandler := slog.NewTextHandler(writer, opts)

	// 包装为 listenerHandler
	handler := &listenerHandler{
		next:    baseHandler,
		backend: listenerBackend,
		level:   level,
	}

	slogLogger = slog.New(handler)

	if localLogEnabled {
		Info("本地文件日志已启用")
	} else {
		Info("本地文件日志已禁用，仅使用控制台输出")
	}
}

// Debug logs at DEBUG level
func Debug(args ...any) {
	msg := fmt.Sprint(args...)
	slogLogger.Debug(msg)
	addToBuffer(DEBUG, msg)
}

// Debugf logs at DEBUG level with format
func Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slogLogger.Debug(msg)
	addToBuffer(DEBUG, msg)
}

// Info logs at INFO level
func Info(args ...any) {
	msg := fmt.Sprint(args...)
	slogLogger.Info(msg)
	addToBuffer(INFO, msg)
}

// Infof logs at INFO level with format
func Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slogLogger.Info(msg)
	addToBuffer(INFO, msg)
}

// Notice logs at NOTICE level
func Notice(args ...any) {
	msg := fmt.Sprint(args...)
	slogLogger.Info(msg) // slog 没有 NOTICE，用 INFO
	addToBuffer(NOTICE, msg)
}

// Noticef logs at NOTICE level with format
func Noticef(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slogLogger.Info(msg)
	addToBuffer(NOTICE, msg)
}

// Warning logs at WARNING level
func Warning(args ...any) {
	msg := fmt.Sprint(args...)
	slogLogger.Warn(msg)
	addToBuffer(WARNING, msg)
}

// Warningf logs at WARNING level with format
func Warningf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slogLogger.Warn(msg)
	addToBuffer(WARNING, msg)
}

// Error logs at ERROR level
func Error(args ...any) {
	msg := fmt.Sprint(args...)
	slogLogger.Error(msg)
	addToBuffer(ERROR, msg)
}

// Errorf logs at ERROR level with format
func Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slogLogger.Error(msg)
	addToBuffer(ERROR, msg)
}

func addToBuffer(level Level, newLog string) {
	logBufferMu.Lock()
	defer logBufferMu.Unlock()

	maxSize := 200
	if len(logBuffer) >= maxSize {
		logBuffer = logBuffer[1:]
	}

	logBuffer = append(logBuffer, LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: newLog,
	})
}

// GetLogs 获取日志，按级别过滤
func GetLogs(c int, level string) []string {
	logBufferMu.RLock()
	defer logBufferMu.RUnlock()

	var output []string
	if logBuffer == nil {
		return output
	}

	logLevel := ParseLevel(level)

	for i := len(logBuffer) - 1; i >= 0 && len(output) < c; i-- {
		if logBuffer[i].Level >= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s",
				logBuffer[i].Time.Format("2006/01/02 15:04:05"),
				logBuffer[i].Level.String(),
				logBuffer[i].Message))
		}
	}
	return output
}

// AddLogListener 添加日志监听器
func AddLogListener(listener LogListener) {
	if listenerBackend != nil {
		listenerBackend.AddListener(listener)
	}
}

// RemoveLogListener 移除日志监听器
func RemoveLogListener(listener LogListener) {
	if listenerBackend != nil {
		listenerBackend.RemoveListener(listener)
	}
}

// GetListenerBackend 获取监听后端
func GetListenerBackend() *ListenerBackend {
	return listenerBackend
}
