package logger

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/op/go-logging"
)

// LogListener 定义日志监听器接口
type LogListener interface {
	OnLog(level logging.Level, message string, formattedLog string)
}

// LogEntry 表示单个日志条目
type LogEntry struct {
	Time    time.Time
	Level   logging.Level
	Message string
}

// ListenerBackend 实现自定义的后端，支持监听器
type ListenerBackend struct {
	listeners []LogListener
	mu        sync.RWMutex
	next      logging.Backend
}

// NewListenerBackend 创建新的监听后端
func NewListenerBackend(next logging.Backend) *ListenerBackend {
	return &ListenerBackend{
		listeners: make([]LogListener, 0),
		next:      next,
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

// Log 实现 logging.Backend 接口
func (b *ListenerBackend) Log(level logging.Level, calldepth int, rec *logging.Record) error {
	// 先调用下一个后端
	if b.next != nil {
		if err := b.next.Log(level, calldepth+1, rec); err != nil {
			return err
		}
	}

	// 通知所有监听器
	b.mu.RLock()
	listeners := make([]LogListener, len(b.listeners))
	copy(listeners, b.listeners)
	b.mu.RUnlock()

	if len(listeners) > 0 {
		formattedLog := rec.Formatted(calldepth + 1)
		for _, listener := range listeners {
			go listener.OnLog(level, rec.Message(), formattedLog)
		}
	}

	return nil
}

var (
	logger           *logging.Logger
	logBuffer        []struct {
		time  string
		level logging.Level
		log   string
	}
	listenerBackend  *ListenerBackend
	localLogEnabled  bool
)

func init() {
	// 默认禁用本地文件日志，保持向后兼容
	InitLogger(logging.INFO, false)
}

func InitLogger(level logging.Level, enabled bool) {
	localLogEnabled = enabled
	newLogger := logging.MustGetLogger("x-ui")
	var err error
	var backend logging.Backend
	var format logging.Formatter
	ppid := os.Getppid()

	// 根据配置决定后端
	if enabled {
		// 启用本地文件日志
		filePath := "x-ui.log"
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logger.Warningf("无法创建日志文件 %s: %v", filePath, err)
			// 回退到控制台
			backend = logging.NewLogBackend(os.Stderr, "", 0)
		} else {
			backend = logging.NewLogBackend(file, "", 0)
			file.Close() // 保持文件句柄开放由logging库管理
		}
	} else {
		// 禁用文件日志，使用syslog或控制台
		backend, err = logging.NewSyslogBackend("")
		if err != nil {
			println(err)
			backend = logging.NewLogBackend(os.Stderr, "", 0)
		}
	}

	if ppid > 0 && err != nil {
		format = logging.MustStringFormatter(`%{time:2006/01/02 15:04:05} %{level} - %{message}`)
	} else {
		format = logging.MustStringFormatter(`%{level} - %{message}`)
	}

	backendFormatter := logging.NewBackendFormatter(backend, format)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "x-ui")

	// 创建监听后端，包装原始后端
	listenerBackend = NewListenerBackend(backendLeveled)
	listenerBackendFormatter := logging.NewBackendFormatter(listenerBackend, format)
	listenerBackendLeveled := logging.AddModuleLevel(listenerBackendFormatter)
	listenerBackendLeveled.SetLevel(level, "x-ui")

	newLogger.SetBackend(listenerBackendLeveled)

	logger = newLogger

	// 记录日志配置状态
	if localLogEnabled {
		logger.Info("本地文件日志已启用")
	} else {
		logger.Info("本地文件日志已禁用，仅使用控制台输出")
	}
}

func Debug(args ...any) {
	logger.Debug(args...)
	addToBuffer("DEBUG", fmt.Sprint(args...))
}

func Debugf(format string, args ...any) {
	logger.Debugf(format, args...)
	addToBuffer("DEBUG", fmt.Sprintf(format, args...))
}

func Info(args ...any) {
	logger.Info(args...)
	addToBuffer("INFO", fmt.Sprint(args...))
}

func Infof(format string, args ...any) {
	logger.Infof(format, args...)
	addToBuffer("INFO", fmt.Sprintf(format, args...))
}

func Notice(args ...any) {
	logger.Notice(args...)
	addToBuffer("NOTICE", fmt.Sprint(args...))
}

func Noticef(format string, args ...any) {
	logger.Noticef(format, args...)
	addToBuffer("NOTICE", fmt.Sprintf(format, args...))
}

func Warning(args ...any) {
	logger.Warning(args...)
	addToBuffer("WARNING", fmt.Sprint(args...))
}

func Warningf(format string, args ...any) {
	logger.Warningf(format, args...)
	addToBuffer("WARNING", fmt.Sprintf(format, args...))
}

func Error(args ...any) {
	logger.Error(args...)
	addToBuffer("ERROR", fmt.Sprint(args...))
}

func Errorf(format string, args ...any) {
	logger.Errorf(format, args...)
	addToBuffer("ERROR", fmt.Sprintf(format, args...))
}

func addToBuffer(level string, newLog string) {
	t := time.Now()
	maxSize := 10240
	if !localLogEnabled {
		// 当本地日志禁用时，使用更小的缓冲区以节省内存
		maxSize = 1000
	}
	if len(logBuffer) >= maxSize {
		logBuffer = logBuffer[1:]
	}

	logLevel, _ := logging.LogLevel(level)
	logBuffer = append(logBuffer, struct {
		time  string
		level logging.Level
		log   string
	}{
		time:  t.Format("2006/01/02 15:04:05"),
		level: logLevel,
		log:   newLog,
	})
}

func GetLogs(c int, level string) []string {
	var output []string
	logLevel, _ := logging.LogLevel(level)

	for i := len(logBuffer) - 1; i >= 0 && len(output) <= c; i-- {
		if logBuffer[i].level <= logLevel {
			output = append(output, fmt.Sprintf("%s %s - %s", logBuffer[i].time, logBuffer[i].level, logBuffer[i].log))
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

// GetListenerBackend 获取监听后端（用于高级用法）
func GetListenerBackend() *ListenerBackend {
	return listenerBackend
}
