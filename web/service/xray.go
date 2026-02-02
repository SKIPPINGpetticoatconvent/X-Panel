package service

import (
	"encoding/json"
	"errors"
	"runtime"
	"strconv"
	"sync"

	"x-ui/logger"
	json_util "x-ui/util/json_util"
	"x-ui/xray"

	"go.uber.org/atomic"
)

// 注意：全局变量正在逐步迁移到 XrayService 结构体中
// 以下变量保留用于向后兼容，将在后续版本中移除
var (
	// Deprecated: 使用 XrayService.GetProcess() 替代
	p *xray.Process
	// Deprecated: 使用 XrayService 实例方法替代
	lock sync.Mutex
	// Deprecated: 使用 XrayService.IsNeedRestart() 替代
	isNeedXrayRestart atomic.Bool
	// Deprecated: 使用 XrayService.IsManuallyStopped() 替代
	isManuallyStopped atomic.Bool
	// Deprecated: 使用 XrayService.GetResult() 替代
	result string
)

// XrayService 管理 Xray 进程的服务
type XrayService struct {
	inboundService *InboundService
	settingService SettingService
	xrayAPI        xray.XrayAPI

	// 以下字段用于封装进程状态（新架构）
	process         *xray.Process
	processLock     sync.Mutex
	needRestart     atomic.Bool
	manuallyStopped atomic.Bool
	lastResult      string
}

// SetInboundService 用于从外部注入 InboundService 实例
func (s *XrayService) SetInboundService(inbound *InboundService) {
	s.inboundService = inbound
}

// SetXrayAPI 用于从外部注入 XrayAPI 实例
func (s *XrayService) SetXrayAPI(api xray.XrayAPI) {
	s.xrayAPI = api
}

// IsXrayRunning 检查 Xray 是否正在运行
func (s *XrayService) IsXrayRunning() bool {
	// 优先使用新架构的 process 字段
	if s.process != nil {
		return s.process.IsRunning()
	}
	// 向后兼容：回退到全局变量
	return p != nil && p.IsRunning()
}

// 中文注释:
// 新增 GetApiPort 函数。
// 这个函数的作用是安全地返回当前 Xray 进程正在监听的 API 端口号。
// 如果 Xray 没有运行 (p == nil)，则返回 0。
// 我们的后台任务将调用这个函数来获取端口号。
func (s *XrayService) GetApiPort() int {
	// 优先使用新架构的 process 字段
	if s.process != nil {
		return s.process.GetAPIPort()
	}
	// 向后兼容：回退到全局变量
	if p == nil {
		return 0
	}
	return p.GetAPIPort()
}

func (s *XrayService) GetXrayErr() error {
	proc := s.process
	if proc == nil {
		proc = p // 向后兼容
	}
	if proc == nil {
		return nil
	}

	err := proc.GetErr()
	if err == nil {
		return nil
	}

	if runtime.GOOS == "windows" && err.Error() == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return nil
	}

	return err
}

func (s *XrayService) GetXrayResult() string {
	// 优先检查新架构的 lastResult
	if s.lastResult != "" {
		return s.lastResult
	}
	// 向后兼容：检查全局变量
	if result != "" {
		return result
	}
	if s.IsXrayRunning() {
		return ""
	}
	proc := s.process
	if proc == nil {
		proc = p // 向后兼容
	}
	if proc == nil {
		return ""
	}

	s.lastResult = proc.GetResult()
	result = s.lastResult // 向后兼容

	if runtime.GOOS == "windows" && result == "exit status 1" {
		// exit status 1 on Windows means that Xray process was killed
		// as we kill process to stop in on Windows, this is not an error
		return ""
	}

	return result
}

func (s *XrayService) GetXrayVersion() string {
	proc := s.process
	if proc == nil {
		proc = p // 向后兼容
	}
	if proc == nil {
		return "Unknown"
	}
	return proc.GetVersion()
}

func RemoveIndex(s []any, index int) []any {
	return append(s[:index], s[index+1:]...)
}

func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	if err := json.Unmarshal([]byte(templateConfig), xrayConfig); err != nil {
		return nil, err
	}

	// Sanitize Outbounds: Remove allowInsecure to adapt to Xray-core v25+
	// Sanitize Outbounds: Remove allowInsecure to adapt to Xray-core v25+
	_ = xrayConfig.AdaptToXrayCoreV25()

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}

	// =================================================================
	// 中文注释: 动态限速核心逻辑 - 第一步: 收集所有限速值
	// =================================================================
	// 创建一个 map 用于存储所有出现过的、不为0的限速值
	uniqueSpeeds := make(map[int]bool)
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}

		// 获取该入站下的所有客户端设置
		dbClients, _ := s.inboundService.GetClients(inbound)
		for _, dbClient := range dbClients {
			if dbClient.SpeedLimit > 0 {
				uniqueSpeeds[dbClient.SpeedLimit] = true
			}
		}
	}

	// =================================================================
	// 中文注释: 动态限速核心逻辑 - 第二步: 根据收集到的限速值，动态生成 Policy Levels
	// =================================================================

	// 1. 先从模板中解析出已有的 policy 对象
	var finalPolicy map[string]interface{}
	if xrayConfig.Policy != nil {
		if err := json.Unmarshal(xrayConfig.Policy, &finalPolicy); err != nil {
			logger.Warningf("无法解析模板中的 policy: %v", err)
			finalPolicy = make(map[string]interface{})
		}
	} else {
		finalPolicy = make(map[string]interface{})
	}

	// 2. 初始化 policy levels，获取或创建 policy中的 levels map
	var policyLevels map[string]interface{}
	if levels, ok := finalPolicy["levels"].(map[string]interface{}); ok {
		policyLevels = levels
	} else {
		policyLevels = make(map[string]interface{})
	}

	// 3. 〔重要修改〕: 确保 level 0 策略的完整性，这是让设备限制和默认用户统计生效的关键
	var level0 map[string]interface{}
	if l0, ok := policyLevels["0"].(map[string]interface{}); ok {
		// 〔中文注释〕: 如果模板中已存在 level 0，使用它作为基础进行修改。
		level0 = l0
	} else {
		// 〔中文注释〕: 如果模板中不存在，则创建一个全新的 map。
		level0 = make(map[string]interface{})
	}
	// 〔中文注释〕: 无论 level 0 是否存在，都为其补充或覆盖以下关键参数。
	// handshake 和 connIdle 是激活 Xray 连接统计的前提，
	// uplinkOnly 和 downlinkOnly 设置为 0 代表不限速，这是 level 0 用户的默认行为。
	// statsUserUplink 和 statsUserDownlink 确保用户的流量能够被统计。
	level0["handshake"] = 4
	level0["connIdle"] = 300
	level0["uplinkOnly"] = 0
	level0["downlinkOnly"] = 0
	level0["statsUserUplink"] = true
	level0["statsUserDownlink"] = true
	// 〔新增〕: 增加此关键选项以启用 Xray-core 的在线 IP 统计功能。
	// 这是让【设备限制】功能正常工作的前提。
	level0["statsUserOnline"] = true

	// 〔中文注释〕: 将完整配置好的 level 0 写回 policyLevels，确保最终生成的 config.json 是正确的。
	policyLevels["0"] = level0

	// 4. 遍历所有收集到的限速值，为每个独立的限速值创建对应的 level
	for speed := range uniqueSpeeds {
		// 为每个速率创建一个 level，level 的名字就是速率的字符串形式
		// 例如，速率 1024 KB/s 对应 level "1024"
		policyLevels[strconv.Itoa(speed)] = map[string]interface{}{
			"downlinkOnly":      speed,
			"uplinkOnly":        speed,
			"handshake":         4,
			"connIdle":          300,
			"statsUserUplink":   true,
			"statsUserDownlink": true,
			"statsUserOnline":   true,
		}
	}

	// 5. 将修改后的 levels 写回 policy 对象，并序列化回 xrayConfig.Policy，将生成的 policy 应用到 Xray 配置中
	finalPolicy["levels"] = policyLevels
	policyJSON, err := json.Marshal(finalPolicy)
	if err != nil {
		return nil, err
	}
	xrayConfig.Policy = json_util.RawMessage(policyJSON)

	// =================================================================
	// 中文注释: 在这里增加日志，打印最终生成的限速策略
	// =================================================================
	if len(uniqueSpeeds) > 0 {
		finalPolicyLog, _ := json.Marshal(policyLevels)
		logger.Infof("已为Xray动态生成〔限速策略〕: %s", string(finalPolicyLog))
	}

	// =================================================================
	// 中文注释: 动态限速核心逻辑 - 第三步: 为设置了限速的用户分配对应的 Level，逐个 inbound 构建 inboundConfig
	// =================================================================
	// 触发一次空调用以处理可能的残留任务
	_, _ = s.inboundService.AddTraffic(nil, nil)

	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}

		// 先生成一个 inboundConfig（后面会覆盖 Settings/StreamSettings）
		inboundConfig := inbound.GenXrayInboundConfig()

		// 从 DB clients 建立 email/id -> speedLimit 映射（优先使用 DB 的值）
		speedByEmail := make(map[string]int)
		speedById := make(map[string]int)
		dbClients, _ := s.inboundService.GetClients(inbound)
		for _, dbc := range dbClients {
			if dbc.Email != "" {
				speedByEmail[dbc.Email] = dbc.SpeedLimit
			}
			// 如果有 id 字段也建立映射（以防 email 不存在）
			if dbc.ID != "" {
				speedById[dbc.ID] = dbc.SpeedLimit
			}
		}

		// 解析 inbound.Settings
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			logger.Warningf("无法解析 inbound.Settings (inbound %d): %v ，跳过该入站", inbound.Id, err)
			continue
		}

		originalClients, ok := settings["clients"].([]interface{})
		if ok {
			clientStats := inbound.ClientStats

			var xrayClients []interface{}
			for _, clientRaw := range originalClients {
				c, ok := clientRaw.(map[string]interface{})
				if !ok {
					continue
				}

				// -----------------------------------------------------------------
				// 中文注释: 用户过滤 - 1) settings 中的 enable 字段检查
				// -----------------------------------------------------------------
				if en, ok := c["enable"].(bool); ok && !en {
					if em, _ := c["email"].(string); em != "" {
						logger.Infof("已从Xray配置中移除被settings标记为禁用的用户: %s", em)
					}
					continue
				}

				// -----------------------------------------------------------------
				// 中文注释: 用户过滤 - 2) inbound.ClientStats 检查 (DB/流量层禁用)
				// -----------------------------------------------------------------
				email, _ := c["email"].(string)
				idStr, _ := c["id"].(string)
				disabledByStat := false
				for _, stat := range clientStats {
					if stat.Email == email && !stat.Enable {
						disabledByStat = true
						break
					}
				}
				if disabledByStat {
					logger.Infof("已从Xray配置中移除被禁用的用户: %s", email)
					continue
				}

				// -----------------------------------------------------------------
				// 中文注释: 构建干净的 xrayClient（只保留白名单字段）
				// -----------------------------------------------------------------
				xrayClient := make(map[string]interface{})
				if id, ok := c["id"]; ok {
					xrayClient["id"] = id
				}
				if email != "" {
					xrayClient["email"] = email
				}

				// 规范化 flow
				if flow, ok := c["flow"]; ok {
					if fs, ok2 := flow.(string); ok2 && fs == "xtls-rprx-vision-udp443" {
						xrayClient["flow"] = "xtls-rprx-vision"
					} else {
						xrayClient["flow"] = flow
					}
				}
				if password, ok := c["password"]; ok {
					xrayClient["password"] = password
				}
				if method, ok := c["method"]; ok {
					xrayClient["method"] = method
				}

				// ⚠️ security 字段已移除，不再加入到 xrayClient

				// -----------------------------------------------------------------
				// 中文注释: 限速等级映射（优先 DB，再回退 settings.speedLimit）
				// -----------------------------------------------------------------

				// =================================================================
				// 这里的逻辑是准备将 client 对象提交给 Xray-core。
				// 我们需要将 speedLimit 转换为 Xray 认识的 level 字段。
				// 这样可以确保包含 speedLimit 的完整用户信息被用于生成配置。
				// =================================================================
				level := 0
				if email != "" {
					if v, ok := speedByEmail[email]; ok && v > 0 {
						level = v
					}
				}
				if level == 0 && idStr != "" {
					if v, ok := speedById[idStr]; ok && v > 0 {
						level = v
					}
				}
				if level == 0 {
					if sl, ok := c["speedLimit"]; ok {
						switch vv := sl.(type) {
						case float64:
							level = int(vv)
						case int:
							level = vv
						case int64:
							level = int(vv)
						case string:
							if n, err := strconv.Atoi(vv); err == nil {
								level = n
							}
						}
					}
				}

				// 【新增功能】在这里添加日志记录
				// 只有当最终计算出的 level 大于 0，且 email 存在时，才记录日志
				if level > 0 && email != "" {
					logger.Infof("为用户 %s 应用〔独立限速〕: %d KB/s", email, level)
				}
				// =================================================================

				xrayClient["level"] = level

				xrayClients = append(xrayClients, xrayClient)
			}

			// 把纯净的 clients 应用到 settings，并写入 inboundConfig.Settings
			settings["clients"] = xrayClients
			finalSettingsForXray, err := json.Marshal(settings)
			if err != nil {
				logger.Warningf("无法序列化用于Xray的入站设置 in GetXrayConfig for inbound %d: %v，跳过该入站", inbound.Id, err)
				continue
			}
			inboundConfig.Settings = json_util.RawMessage(finalSettingsForXray)
		}

		// -----------------------------------------------------------------
		// 中文注释: 处理 StreamSettings（清理敏感字段）
		// -----------------------------------------------------------------
		if len(inbound.StreamSettings) > 0 {
			var stream map[string]interface{}
			if err := json.Unmarshal([]byte(inbound.StreamSettings), &stream); err != nil {
				logger.Warningf("无法解析 StreamSettings (inbound %d): %v ，跳过该入站", inbound.Id, err)
				continue
			}

			if tlsSettings, ok := stream["tlsSettings"].(map[string]interface{}); ok {
				delete(tlsSettings, "settings")
			}
			if realitySettings, ok := stream["realitySettings"].(map[string]interface{}); ok {
				delete(realitySettings, "settings")
			}
			delete(stream, "externalProxy")

			newStream, err := json.Marshal(stream)
			if err != nil {
				return nil, err
			}
			inboundConfig.StreamSettings = json_util.RawMessage(newStream)
		}

		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}

	return xrayConfig, nil
}

func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, []*xray.ClientTraffic, error) {
	if !s.IsXrayRunning() {
		err := errors.New("xray is not running")
		logger.Debug("Attempted to fetch Xray traffic, but Xray is not running:", err)
		return nil, nil, err
	}
	proc := s.process
	if proc == nil {
		proc = p // 向后兼容
	}
	apiPort := proc.GetAPIPort()
	_ = s.xrayAPI.Init(apiPort)
	defer s.xrayAPI.Close()

	traffic, clientTraffic, err := s.xrayAPI.GetTraffic(true)
	if err != nil {
		logger.Debug("Failed to fetch Xray traffic:", err)
		return nil, nil, err
	}
	return traffic, clientTraffic, nil
}

func (s *XrayService) RestartXray(isForce bool) error {
	s.processLock.Lock()
	defer s.processLock.Unlock()
	// 同步全局变量（向后兼容）
	lock.Lock()
	defer lock.Unlock()

	logger.Debug("restart Xray, force:", isForce)
	s.manuallyStopped.Store(false)
	isManuallyStopped.Store(false) // 向后兼容

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	// 【新功能】重启时，将完整配置打印到 Debug 日志以供验证
	configBytes, jsonErr := json.MarshalIndent(xrayConfig, "", "  ")
	if jsonErr == nil {
		logger.Debugf("使用新配置重启 Xray：\n%s", string(configBytes))
	} else {
		logger.Warning("无法将 Xray 配置编组以进行日志记录：", jsonErr)
	}

	if s.IsXrayRunning() {
		if !isForce && s.process.GetConfig().Equals(xrayConfig) && !s.needRestart.Load() {
			logger.Debug("It does not need to restart Xray")
			return nil
		}
		_ = s.process.Stop()
	}

	newProcess := xray.NewProcess(xrayConfig)
	s.process = newProcess
	p = newProcess // 向后兼容
	s.lastResult = ""
	result = "" // 向后兼容
	err = s.process.Start()
	if err != nil {
		return err
	}

	return nil
}

func (s *XrayService) StopXray() error {
	s.processLock.Lock()
	defer s.processLock.Unlock()
	// 同步全局变量（向后兼容）
	lock.Lock()
	defer lock.Unlock()

	s.manuallyStopped.Store(true)
	isManuallyStopped.Store(true) // 向后兼容
	logger.Debug("Attempting to stop Xray...")
	if s.IsXrayRunning() {
		return s.process.Stop()
	}
	return errors.New("xray is not running")
}

func (s *XrayService) SetToNeedRestart() {
	s.needRestart.Store(true)
	isNeedXrayRestart.Store(true) // 向后兼容
}

func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	// 同时更新两个变量
	result := s.needRestart.CompareAndSwap(true, false)
	_ = isNeedXrayRestart.CompareAndSwap(true, false) // 向后兼容
	return result
}

// Check if Xray is not running and wasn't stopped manually, i.e. crashed
func (s *XrayService) DidXrayCrash() bool {
	return !s.IsXrayRunning() && !s.manuallyStopped.Load()
}

// GetProcess 返回当前 Xray 进程实例
func (s *XrayService) GetProcess() *xray.Process {
	return s.process
}
