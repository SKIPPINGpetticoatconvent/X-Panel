package job

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"sync"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/database/repository"
	"x-ui/logger"
	"x-ui/web/service"
	"x-ui/xray"
)

// =================================================================
// 以下是用于实现设备限制功能的核心代码
// =================================================================

// ActiveClientIPs 用于在内存中跟踪每个用户的活跃IP (TTL机制)
// 结构: map[用户email] -> map[IP地址] -> 最后活跃时间
var (
	ActiveClientIPs   = make(map[string]map[string]time.Time)
	activeClientsLock sync.RWMutex
)

// ActiveClientIPs 容量上限常量引用自 config 包
var (
	maxIPsPerEmail = config.MaxIPsPerEmail
	maxTotalEmails = config.MaxTotalEmails
)

// ClientStatus 中文注释: 用于跟踪每个用户的状态（是否因为设备超限而被禁用）
// 结构: map[用户email] -> 是否被禁用(true/false)
var (
	ClientStatus     = make(map[string]bool)
	clientStatusLock sync.RWMutex
)

// CheckDeviceLimitJob 重构后的设备限制任务，使用 LogStreamer 实现实时监控
type CheckDeviceLimitJob struct {
	inboundService *service.InboundService
	xrayService    *service.XrayService
	settingService service.SettingService
	// 新增 xrayApi 字段，用于持有 Xray API 客户端实例
	xrayApi xray.XrayAPI
	// 使用 LogStreamer 进行实时日志监控
	logStreamer *LogStreamer
	// 控制 LogStreamer 的启动和停止
	isStreamerRunning bool
	// 注入 Telegram 服务用于发送通知，确保此行存在。
	telegramService service.TelegramService
	// 等待组用于优雅关闭
	wg sync.WaitGroup
	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc
}

// RandomUUID 生成随机 UUID 的辅助函数
func RandomUUID() string {
	uuid := make([]byte, 16)
	// 使用 math/rand 而不是 crypto/rand 来避免编译错误
	//nolint:gosec
	for i := range uuid {
		uuid[i] = byte(rand.Int() & 0xFF)
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// NewCheckDeviceLimitJob 中文注释: 创建一个新的任务实例
// 增加一个 service.TelegramService 类型的参数。
func NewCheckDeviceLimitJob(inboundService *service.InboundService, xrayService *service.XrayService, telegramService service.TelegramService, settingService service.SettingService) *CheckDeviceLimitJob {
	ctx, cancel := context.WithCancel(context.Background())

	return &CheckDeviceLimitJob{
		inboundService: inboundService,
		xrayService:    xrayService,
		settingService: settingService,
		// 初始化 xrayApi 字段
		xrayApi: xray.XrayAPI{},
		// 将传入的 telegramService 赋值给结构体实例。
		telegramService:   telegramService,
		ctx:               ctx,
		cancel:            cancel,
		isStreamerRunning: false,
	}
}

func (j *CheckDeviceLimitJob) Name() string {
	return "CheckDeviceLimitJob"
}

// Start 启动重构后的设备限制任务（使用 LogStreamer）
func (j *CheckDeviceLimitJob) Start() error {
	if j.isStreamerRunning {
		return nil
	}

	// 检查 LogStreamer 是否启用
	logStreamerEnabled, err := j.settingService.GetLogStreamerEnabled()
	if err != nil {
		logger.Warningf("无法获取 LogStreamer 配置: %v", err)
		logStreamerEnabled = false // 默认禁用
	}

	if !logStreamerEnabled {
		logger.Info("LogStreamer 已禁用，不启动实时日志监控")
		// 即使不启动 LogStreamer，也启动设备限制检查的 goroutine（使用其他方式检查）
		j.wg.Add(1)
		go j.limitCheckLoop()
		return nil
	}

	// 检查日志路径并初始化 LogStreamer
	logPath, err := xray.GetAccessLogPath()
	if err != nil || logPath == "none" || logPath == "" {
		logger.Warning("设备限制任务启动失败：无法获取有效的日志路径")
		return fmt.Errorf("无效的日志路径: %v", err)
	}

	// 创建 LogStreamer
	j.logStreamer = NewLogStreamer(logPath)

	// 启动 LogStreamer
	if err := j.logStreamer.Start(); err != nil {
		return fmt.Errorf("启动日志流处理器失败: %v", err)
	}

	j.isStreamerRunning = true

	// 启动设备限制检查的 goroutine
	j.wg.Add(1)
	go j.limitCheckLoop()

	logger.Infof("重构后的设备限制任务已启动，监控日志文件: %s", logPath)
	return nil
}

// Stop 停止设备限制任务
func (j *CheckDeviceLimitJob) Stop() error {
	if !j.isStreamerRunning {
		return nil
	}

	logger.Infof("正在停止设备限制任务...")

	// 发送停止信号
	j.cancel()

	// 停止 LogStreamer
	if j.logStreamer != nil {
		if err := j.logStreamer.Stop(); err != nil {
			logger.Warningf("停止 LogStreamer 时出错: %v", err)
		}
	}

	// 等待所有 goroutine 退出
	done := make(chan struct{})
	go func() {
		j.wg.Wait()
		close(done)
	}()

	// 最多等待 10 秒
	select {
	case <-done:
		logger.Infof("设备限制任务已停止")
	case <-time.After(config.DeviceLimitStopTimeout):
		logger.Warning("设备限制任务停止超时")
	}

	j.isStreamerRunning = false
	return nil
}

// Run 中文注释: 保留原有的 Run 方法用于向后兼容，但不再进行日志解析
func (j *CheckDeviceLimitJob) Run() {
	// 新的实现中，Run() 只进行设备限制检查，日志监控由 LogStreamer 处理
	if !j.xrayService.IsXrayRunning() {
		return
	}

	// 执行设备限制检查
	j.performLimitCheck()
}

// limitCheckLoop 设备限制检查循环
func (j *CheckDeviceLimitJob) limitCheckLoop() {
	defer j.wg.Done()

	// 使用配置的检查间隔
	ticker := time.NewTicker(config.DeviceLimitCheckInterval)
	defer ticker.Stop()

	// 启动时立即执行一次检查
	j.performLimitCheck()

	for {
		select {
		case <-j.ctx.Done():
			return
		case <-ticker.C:
			j.performLimitCheck()
		}
	}
}

// performLimitCheck 执行设备限制检查
func (j *CheckDeviceLimitJob) performLimitCheck() {
	// 如果 LogStreamer 未运行，则跳过检查（因为没有实时数据）
	if !j.isStreamerRunning {
		return
	}

	// 1. 清理过期的IP
	j.cleanupExpiredIPs()

	// 2. 检查所有用户的设备限制状态
	j.checkAllClientsLimit()
}

// cleanupExpiredIPs 中文注释: 清理长时间不活跃的IP
func (j *CheckDeviceLimitJob) cleanupExpiredIPs() {
	activeClientsLock.Lock()
	defer activeClientsLock.Unlock()

	now := time.Now()
	// 活跃判断窗口(TTL): 近3分钟内出现过就算"活跃"
	activeTTL := config.DeviceLimitActiveTTL
	for email, ips := range ActiveClientIPs {
		for ip, lastSeen := range ips {
			// 如果一个IP超过3分钟没有新的连接日志，我们就认为它已经下线
			if now.Sub(lastSeen) > activeTTL {
				delete(ActiveClientIPs[email], ip)
			}
		}
		// 如果一个用户的所有IP都下线了，就从大Map中移除这个用户，节省内存
		if len(ActiveClientIPs[email]) == 0 {
			delete(ActiveClientIPs, email)
		}
	}
}

// checkAllClientsLimit 中文注释: 核心功能，检查所有用户，对超限的执行封禁，对恢复的执行解封
func (j *CheckDeviceLimitJob) checkAllClientsLimit() {
	if j.xrayService == nil {
		logger.Warning("[DeviceLimit] XrayServices not ready, skipping cycle.")
		return
	}
	db := repository.NewInboundRepository(database.GetDB()).GetDB()
	var inbounds []*model.Inbound
	// 这里仅查询启用了设备限制(device_limit > 0)并且自身是开启状态的入站规则
	db.Where("device_limit > 0 AND enable = ?", true).Find(&inbounds)

	if len(inbounds) == 0 {
		return
	}

	// 获取 API 端口。如果端口为0 (说明Xray未完全启动或有问题)，则直接返回
	apiPort := j.xrayService.GetApiPort()
	if apiPort == 0 {
		return
	}
	// 使用获取到的端口号初始化 API 客户端
	_ = j.xrayApi.Init(apiPort)
	defer j.xrayApi.Close()

	// 优化 - 在一次循环中同时获取 tag 和 protocol
	inboundInfoMap := make(map[int]struct {
		Limit    int
		Tag      string
		Protocol model.Protocol
	})
	for _, inbound := range inbounds {
		inboundInfoMap[inbound.Id] = struct {
			Limit    int
			Tag      string
			Protocol model.Protocol
		}{Limit: inbound.DeviceLimit, Tag: inbound.Tag, Protocol: inbound.Protocol}
	}

	// 获取当前的活跃客户端IP映射
	activeClientIPs := j.logStreamer.GetActiveClientIPs()

	activeClientsLock.RLock()
	clientStatusLock.Lock()
	defer activeClientsLock.RUnlock()
	defer clientStatusLock.Unlock()

	// 第一步: 处理当前在线的用户
	for email, ips := range activeClientIPs {
		traffic, err := j.inboundService.GetClientTrafficByEmail(email)
		if err != nil || traffic == nil {
			continue
		}

		info, ok := inboundInfoMap[traffic.InboundId]
		if !ok || info.Limit <= 0 {
			continue
		}

		isBanned := ClientStatus[email]
		activeIPCount := len(ips)

		// 调用封禁函数
		if activeIPCount > info.Limit && !isBanned {
			// 调用封禁函数时，传入当前的IP数用于记录日志
			j.banUser(email, activeIPCount, &info)
		}

		// 调用解封函数
		if activeIPCount <= info.Limit && isBanned {
			// 调用解封函数时，传入当前的IP数用于记录日志
			j.unbanUser(email, activeIPCount, &info)
		}
	}

	// 第二步: 专门处理那些"已被封禁"但"已不在线"的用户，为他们解封
	for email, isBanned := range ClientStatus {
		if !isBanned {
			continue
		}
		if _, online := activeClientIPs[email]; !online {
			traffic, err := j.inboundService.GetClientTrafficByEmail(email)
			if err != nil || traffic == nil {
				continue
			}
			info, ok := inboundInfoMap[traffic.InboundId]
			if !ok {
				continue
			}
			logger.Infof("已封禁用户 %s 已完全下线，执行解封操作。", email)

			// 调用解封函数，这种情况下：活跃IP数为0，我们直接传入0用于记录日志
			j.unbanUser(email, 0, &info)
		}
	}
}

// banUser 中文注释: 封装的封禁用户函数；IP数量超限，且用户当前未被封禁 -> 执行封禁 (UUID 替换)
func (j *CheckDeviceLimitJob) banUser(email string, activeIPCount int, info *struct {
	Limit    int
	Tag      string
	Protocol model.Protocol
},
) {
	_, client, err := j.inboundService.GetClientByEmail(email)
	if err != nil || client == nil {
		return
	}

	if j.xrayService == nil {
		return
	}

	logger.Infof("〔设备限制〕超限：用户 %s. 限制: %d, 当前活跃: %d. 执行封禁掐网。", email, info.Limit, activeIPCount)

	// 以下是发送 Telegram 通知的核心代码，
	// 它会调用我们注入的 telegramService 的 SendMessage 方法。
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		// 在调用前，先判断服务实例是否为 nil，增加代码健壮性。
		if j.telegramService == nil {
			return
		}
		tgMessage := fmt.Sprintf(
			"<b>〔X-Panel面板〕设备超限提醒</b>\n\n"+
				"  ------------------------------------\n"+
				"  👤 用户 Email：%s\n"+
				"  🖥️ 设备限制数量：%d\n"+
				"  🌐 当前在线IP数：%d\n"+
				"  ------------------------------------\n\n"+
				"<b><i>⚠ 该用户已被自动掐网封禁！</i></b>",
			email, info.Limit, activeIPCount,
		)
		// 调用接口方法发送消息。
		err := j.telegramService.SendMessage(tgMessage)
		if err != nil {
			logger.Warningf("发送 Telegram 封禁通知失败: %v", err)
		}
	}()

	// 步骤一：先从 Xray-Core 中删除该用户。
	_ = j.xrayApi.RemoveUser(info.Tag, email)

	// 使用配置的延时，解决竞态条件问题
	time.Sleep(config.DeviceLimitOperationDelay)

	// 创建一个带有随机UUID/Password的临时客户端配置用于"封禁"
	tempClient := *client

	// 适用于 VMess/VLESS
	if tempClient.ID != "" {
		tempClient.ID = RandomUUID()
	}

	// 适用于 Trojan/Shadowsocks/Socks
	if tempClient.Password != "" {
		tempClient.Password = RandomUUID()
	}

	var clientMap map[string]interface{}
	clientJson, _ := json.Marshal(tempClient)
	_ = json.Unmarshal(clientJson, &clientMap)

	// 步骤二：将这个带有错误UUID/Password的临时用户添加回去。
	// 客户端持有的还是旧的UUID，自然就无法通过验证，从而达到了"封禁"的效果。
	err = j.xrayApi.AddUser(string(info.Protocol), info.Tag, clientMap)
	if err != nil {
		logger.Warningf("通过API封禁用户 %s 失败: %v", email, err)
	} else {
		// 封禁成功后，在内存中标记该用户为"已封禁"状态。
		ClientStatus[email] = true
	}
}

// unbanUser 中文注释: 封装的解封用户函数；IP数量已恢复正常，但用户处于封禁状态 -> 执行解封 (恢复原始 UUID)
func (j *CheckDeviceLimitJob) unbanUser(email string, activeIPCount int, info *struct {
	Limit    int
	Tag      string
	Protocol model.Protocol
},
) {
	_, client, err := j.inboundService.GetClientByEmail(email)
	if err != nil || client == nil {
		return
	}
	logger.Infof("〔设备数量〕已恢复：用户 %s. 限制: %d, 当前活跃: %d. 执行解封/恢复用户。", email, info.Limit, activeIPCount)

	// 步骤一：先从 Xray-Core 中删除用于"封禁"的那个临时用户。
	_ = j.xrayApi.RemoveUser(info.Tag, email)

	// 使用配置的延时，确保解封操作的稳定性
	time.Sleep(config.DeviceLimitOperationDelay)

	var clientMap map[string]interface{}
	clientJson, _ := json.Marshal(client)
	_ = json.Unmarshal(clientJson, &clientMap)

	// 步骤二：将数据库中原始的、正确的用户信息重新添加回 Xray-Core，从而实现"解封"。
	err = j.xrayApi.AddUser(string(info.Protocol), info.Tag, clientMap)
	if err != nil {
		logger.Warningf("通过API恢复用户 %s 失败: %v", email, err)
	} else {
		// 解封成功后，从内存中移除该用户的"已封禁"状态标记。
		delete(ClientStatus, email)
	}
}

// =================================================================
// 以下是原有的 CheckClientIpJob 代码保持不变
// =================================================================

type CheckClientIpJob struct {
	lastClear     int64
	disAllowedIps []string
	inboundRepo   repository.InboundRepository
	clientIPRepo  repository.ClientIPRepository
}

// getInboundRepo 延迟初始化并返回 InboundRepository
func (j *CheckClientIpJob) getInboundRepo() repository.InboundRepository {
	if j.inboundRepo == nil {
		j.inboundRepo = repository.NewInboundRepository(database.GetDB())
	}
	return j.inboundRepo
}

// getClientIPRepo 延迟初始化并返回 ClientIPRepository
func (j *CheckClientIpJob) getClientIPRepo() repository.ClientIPRepository {
	if j.clientIPRepo == nil {
		j.clientIPRepo = repository.NewClientIPRepository(database.GetDB())
	}
	return j.clientIPRepo
}

var job *CheckClientIpJob

func NewCheckClientIpJob() *CheckClientIpJob {
	job = new(CheckClientIpJob)
	return job
}

func (j *CheckClientIpJob) Run() {
	if j.lastClear == 0 {
		j.lastClear = time.Now().Unix()
	}

	shouldClearAccessLog := false
	iplimitActive := j.hasLimitIp()
	f2bInstalled := j.checkFail2BanInstalled()
	isAccessLogAvailable := j.checkAccessLogAvailable(iplimitActive)

	if isAccessLogAvailable {
		if runtime.GOOS == "windows" {
			if iplimitActive {
				shouldClearAccessLog = j.processLogFile()
			}
		} else {
			if iplimitActive {
				if f2bInstalled {
					shouldClearAccessLog = j.processLogFile()
				} else {
					if !f2bInstalled {
						logger.Warning("[LimitIP] Fail2Ban is not installed, Please install Fail2Ban from the x-ui bash menu.")
					}
				}
			}
		}
	}

	if shouldClearAccessLog || (isAccessLogAvailable && time.Now().Unix()-j.lastClear > 3600) {
		j.clearAccessLog()
	}
}

func (j *CheckClientIpJob) clearAccessLog() {
	logAccessP, err := os.OpenFile(xray.GetAccessPersistentLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	j.checkError(err)
	defer func() { _ = logAccessP.Close() }()

	accessLogPath, err := xray.GetAccessLogPath()
	j.checkError(err)

	//nolint:gosec
	file, err := os.Open(accessLogPath)
	if err != nil {
		j.checkError(err)
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(logAccessP, file)
	j.checkError(err)

	err = os.Truncate(accessLogPath, 0)
	j.checkError(err)

	j.lastClear = time.Now().Unix()
}

func (j *CheckClientIpJob) hasLimitIp() bool {
	inbounds, err := j.getInboundRepo().FindAll()
	if err != nil {
		return false
	}

	for _, inbound := range inbounds {
		if inbound.Settings == "" {
			continue
		}

		settings := map[string][]model.Client{}
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)
		clients := settings["clients"]

		for _, client := range clients {
			limitIp := client.LimitIP
			if limitIp > 0 {
				return true
			}
		}
	}

	return false
}

func (j *CheckClientIpJob) processLogFile() bool {
	ipRegex := regexp.MustCompile(`from (?:tcp:|udp:)?\[?([0-9a-fA-F\.:]+)\]?:\d+ accepted`)
	emailRegex := regexp.MustCompile(`email: (.+)$`)

	accessLogPath, _ := xray.GetAccessLogPath()
	//nolint:gosec
	file, _ := os.Open(accessLogPath)
	defer func() { _ = file.Close() }()

	inboundClientIps := make(map[string]map[string]struct{}, 100)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		ipMatches := ipRegex.FindStringSubmatch(line)
		if len(ipMatches) < 2 {
			continue
		}

		ip := ipMatches[1]

		if ip == "127.0.0.1" || ip == "::1" {
			continue
		}

		emailMatches := emailRegex.FindStringSubmatch(line)
		if len(emailMatches) < 2 {
			continue
		}
		email := emailMatches[1]

		if _, exists := inboundClientIps[email]; !exists {
			inboundClientIps[email] = make(map[string]struct{})
		}
		inboundClientIps[email][ip] = struct{}{}
	}

	shouldCleanLog := false
	for email, uniqueIps := range inboundClientIps {

		ips := make([]string, 0, len(uniqueIps))
		for ip := range uniqueIps {
			ips = append(ips, ip)
		}
		sort.Strings(ips)

		clientIpsRecord, err := j.getInboundClientIps(email)
		if err != nil {
			_ = j.addInboundClientIps(email, ips)
			continue
		}

		shouldCleanLog = j.updateInboundClientIps(clientIpsRecord, email, ips) || shouldCleanLog
	}

	return shouldCleanLog
}

func (j *CheckClientIpJob) checkFail2BanInstalled() bool {
	cmd := "fail2ban-client"
	args := []string{"-h"}
	err := exec.Command(cmd, args...).Run()
	return err == nil
}

func (j *CheckClientIpJob) checkAccessLogAvailable(iplimitActive bool) bool {
	accessLogPath, err := xray.GetAccessLogPath()
	if err != nil {
		return false
	}

	if accessLogPath == "none" || accessLogPath == "" {
		if iplimitActive {
			logger.Warning("[LimitIP] Access log path is not set, Please configure the access log path in Xray configs.")
		}
		return false
	}

	return true
}

func (j *CheckClientIpJob) checkError(e error) {
	if e != nil {
		logger.Warning("client ip job err:", e)
	}
}

func (j *CheckClientIpJob) getInboundClientIps(clientEmail string) (*model.InboundClientIps, error) {
	return j.getClientIPRepo().FindByEmail(clientEmail)
}

func (j *CheckClientIpJob) addInboundClientIps(clientEmail string, ips []string) error {
	jsonIps, err := json.Marshal(ips)
	j.checkError(err)

	inboundClientIps := &model.InboundClientIps{
		ClientEmail: clientEmail,
		Ips:         string(jsonIps),
	}

	return j.getClientIPRepo().Create(inboundClientIps)
}

func (j *CheckClientIpJob) updateInboundClientIps(inboundClientIps *model.InboundClientIps, clientEmail string, ips []string) bool {
	jsonIps, err := json.Marshal(ips)
	if err != nil {
		logger.Error("failed to marshal IPs to JSON:", err)
		return false
	}

	inboundClientIps.ClientEmail = clientEmail
	inboundClientIps.Ips = string(jsonIps)

	inbound, err := j.getInboundByEmail(clientEmail)
	if err != nil {
		logger.Errorf("failed to fetch inbound settings for email %s: %s", clientEmail, err)
		return false
	}
	if inbound == nil {
		logger.Debugf("no inbound found for email %s", clientEmail)
		return false
	}

	if inbound.Settings == "" {
		logger.Debug("wrong data:", inbound)
		return false
	}

	settings := map[string][]model.Client{}
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	clients := settings["clients"]
	shouldCleanLog := false
	j.disAllowedIps = []string{}

	logIpFile, err := os.OpenFile(xray.GetIPLimitLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		logger.Errorf("failed to open IP limit log file: %s", err)
		return false
	}
	defer func() { _ = logIpFile.Close() }()
	log.SetOutput(logIpFile)
	log.SetFlags(log.LstdFlags)

	for _, client := range clients {
		if client.Email == clientEmail {
			limitIp := client.LimitIP

			if limitIp > 0 && inbound.Enable {
				shouldCleanLog = true

				if limitIp < len(ips) {
					j.disAllowedIps = append(j.disAllowedIps, ips[limitIp:]...)
					for i := limitIp; i < len(ips); i++ {
						log.Printf("[LIMIT_IP] Email = %s || SRC = %s", clientEmail, ips[i])
					}
				}
			}
		}
	}

	sort.Strings(j.disAllowedIps)

	if len(j.disAllowedIps) > 0 {
		logger.Debug("disAllowedIps:", j.disAllowedIps)
	}

	err = j.getClientIPRepo().Update(inboundClientIps)
	if err != nil {
		logger.Error("failed to save inboundClientIps:", err)
		return false
	}

	return shouldCleanLog
}

func (j *CheckClientIpJob) getInboundByEmail(clientEmail string) (*model.Inbound, error) {
	inbounds, err := j.getInboundRepo().Search(clientEmail)
	if err != nil {
		return nil, err
	}
	if len(inbounds) == 0 {
		return nil, nil
	}
	return inbounds[0], nil
}
