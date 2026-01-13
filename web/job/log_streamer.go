package job

import (
	"context"
	"regexp"
	"sync"
	"time"

	"x-ui/logger"

	"github.com/nxadm/tail"
)

// LogStreamer 使用 tail 库实现实时日志流处理
type LogStreamer struct {
	logPath    string
	emailRegex *regexp.Regexp
	ipRegex    *regexp.Regexp
	tailer     *tail.Tail
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	mu         sync.RWMutex
}

// NewLogStreamer 创建一个新的日志流处理器
func NewLogStreamer(logPath string) *LogStreamer {
	ctx, cancel := context.WithCancel(context.Background())

	return &LogStreamer{
		logPath:    logPath,
		emailRegex: regexp.MustCompile(`email: ([^ ]+)`),
		ipRegex:    regexp.MustCompile(`from (?:tcp:|udp:)?\[?([0-9a-fA-F\.:]+)\]?:\d+ accepted`),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动日志流处理器
func (ls *LogStreamer) Start() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.isRunning {
		return nil
	}

	// 配置 tail 选项
	config := tail.Config{
		Follow:    true,                                 // 跟随文件变化
		ReOpen:    true,                                 // 重新打开文件（处理日志轮转）
		MustExist: false,                                // 文件不存在时不报错，会等待创建
		Poll:      true,                                 // 使用轮询而不是 inotify（更兼容）
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // 从文件末尾开始
	}

	var err error
	ls.tailer, err = tail.TailFile(ls.logPath, config)
	if err != nil {
		return err
	}

	ls.isRunning = true
	ls.wg.Add(1)

	// 启动日志处理 goroutine
	go ls.processLogLines()

	logger.Infof("LogStreamer 已启动，监控日志文件: %s", ls.logPath)
	return nil
}

// Stop 停止日志流处理器
func (ls *LogStreamer) Stop() error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if !ls.isRunning {
		return nil
	}

	ls.cancel()

	if ls.tailer != nil {
		_ = ls.tailer.Stop()
		ls.tailer.Cleanup()
	}

	done := make(chan struct{})
	go func() {
		ls.wg.Wait()
		close(done)
	}()

	// 等待最多5秒让goroutine优雅退出
	select {
	case <-done:
	case <-make(chan struct{}, 1):
		logger.Warning("LogStreamer 停止超时")
	}

	ls.isRunning = false
	logger.Infof("LogStreamer 已停止: %s", ls.logPath)
	return nil
}

// IsRunning 返回日志流处理器是否在运行
func (ls *LogStreamer) IsRunning() bool {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.isRunning
}

// processLogLines 处理日志行
func (ls *LogStreamer) processLogLines() {
	defer ls.wg.Done()

	for {
		select {
		case <-ls.ctx.Done():
			return
		case line, ok := <-ls.tailer.Lines:
			if !ok {
				return
			}
			if line.Err != nil {
				logger.Errorf("读取日志行错误: %v", line.Err)
				continue
			}
			ls.parseLogLine(line.Text)
		}
	}
}

// parseLogLine 解析单行日志
func (ls *LogStreamer) parseLogLine(line string) {
	emailMatch := ls.emailRegex.FindStringSubmatch(line)
	ipMatch := ls.ipRegex.FindStringSubmatch(line)

	if len(emailMatch) > 1 && len(ipMatch) > 1 {
		email := emailMatch[1]
		ip := ipMatch[1]

		// 过滤本地回环地址
		if ip == "127.0.0.1" || ip == "::1" {
			return
		}

		// 更新活跃客户端IP
		ls.updateActiveClientIP(email, ip)
	}
}

// updateActiveClientIP 更新活跃客户端IP
func (ls *LogStreamer) updateActiveClientIP(email string, ip string) {
	activeClientsLock.Lock()
	defer activeClientsLock.Unlock()

	now := time.Now()

	if _, ok := ActiveClientIPs[email]; !ok {
		ActiveClientIPs[email] = make(map[string]time.Time)
	}

	ActiveClientIPs[email][ip] = now
}

// GetActiveClientIPs 获取当前活跃的客户端IP映射（供外部查询）
func (ls *LogStreamer) GetActiveClientIPs() map[string]map[string]time.Time {
	activeClientsLock.RLock()
	defer activeClientsLock.RUnlock()

	// 返回副本避免并发问题
	result := make(map[string]map[string]time.Time)
	for email, ips := range ActiveClientIPs {
		result[email] = make(map[string]time.Time)
		for ip, t := range ips {
			result[email][ip] = t
		}
	}

	return result
}
