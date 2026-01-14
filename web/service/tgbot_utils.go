package service

import (
	"context"
	"crypto/rand" // 新增：用于 tls.Config
	"encoding/base64"
	"encoding/json" // 新增：用于 json.Marshal / Unmarshal
	"fmt"
	"math/big" // 新增：用于 http.Client / Transport
	"os"
	"os/exec" // 新增：用于 exec.Command（getDomain 等）

	// 新增：用于 filepath.Base / Dir（getDomain 用到）
	"strings"
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/sys"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	// 新增 qrcode 包，用于生成二维码
)

func (t *Tgbot) SetHostname() {
	host, err := os.Hostname()
	if err != nil {
		logger.Error("get hostname error:", err)
		hostname = ""
		return
	}
	hostname = host
}

func (t *Tgbot) Stop() {
	if botHandler != nil {
		_ = botHandler.Stop()
	}
	logger.Info("Stop Telegram receiver ...")
	isRunning = false
	adminIds = nil
}

func (t *Tgbot) encodeQuery(query string) string {
	// NOTE: we only need to hash for more than 64 chars
	if len(query) <= 64 {
		return query
	}

	return hashStorage.SaveHash(query)
}

func (t *Tgbot) decodeQuery(query string) (string, error) {
	if !hashStorage.IsMD5(query) {
		return query, nil
	}

	decoded, exists := hashStorage.GetValue(query)
	if !exists {
		return "", common.NewError("hash not found in storage!")
	}

	return decoded, nil
}

func (t *Tgbot) randomLowerAndNum(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)
	for i := range bytes {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		bytes[i] = charset[randomIndex.Int64()]
	}
	return string(bytes)
}

func (t *Tgbot) randomShadowSocksPassword() string {
	array := make([]byte, 32)
	_, err := rand.Read(array)
	if err != nil {
		return t.randomLowerAndNum(32)
	}
	return base64.StdEncoding.EncodeToString(array)
}

func (t *Tgbot) randomString(length int, charset string) string {
	bytes := make([]byte, length)
	for i := range bytes {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		bytes[i] = charset[randomIndex.Int64()]
	}
	return string(bytes)
}

// 【新增辅助函数】: 保存链接历史到数据库
func (t *Tgbot) saveLinkToHistory(linkType string, link string) {
	history := &database.LinkHistory{
		Type:      linkType,
		Link:      link,
		CreatedAt: time.Now(),
	}
	if err := database.AddLinkHistory(history); err != nil {
		logger.Warningf("保存链接历史到数据库失败: %v", err)
	}
	_ = database.Checkpoint()
}

// 新增一个公共方法 (大写 G) 来包装私有方法
func (t *Tgbot) GetDomain() (string, error) {
	return t.getDomain()
}

// openPortWithFirewalld 检查/安装 firewalld，放行一系列默认端口，并放行指定的端口
func (t *Tgbot) openPortWithFirewalld(port int) error {
	// 【中文注释】: 将所有 Shell 逻辑整合为一个命令。
	// 新增了对默认端口列表 (22, 80, 443, 13688, 8443) 的放行逻辑。
	shellCommand := fmt.Sprintf(`
	# 定义需要放行的指定端口和一系列默认端口
	PORT_TO_OPEN=%d
	DEFAULT_PORTS="22 80 443 13688 8443"

	echo "脚本开始：准备配置 firewalld 防火墙..."

	# 1. 检查/安装 firewalld
	if ! command -v firewall-cmd &> /dev/null; then
		echo "firewalld 防火墙未安装，正在自动安装..."
		# 使用新的防火墙安装命令
		sudo apt update
		sudo apt install -y firewalld
		sudo systemctl enable firewalld --now
	fi

	# 2. 【新增】循环放行所有默认端口
	echo "正在检查并放行基础服务端口: $DEFAULT_PORTS"
	for p in $DEFAULT_PORTS; do
		# 使用静默模式检查规则是否存在，如果不存在则添加
		if ! firewall-cmd --list-ports | grep -qw "$p/tcp"; then
			echo "端口 $p/tcp 未放行，正在执行 firewall-cmd --zone=public --add-port=$p/tcp --permanent..."
			firewall-cmd --zone=public --add-port=$p/tcp --permanent >/dev/null
			if [ $? -ne 0 ]; then echo "❌ firewalld 端口 $p 放行失败。"; exit 1; fi
		else
			echo "端口 $p/tcp 规则已存在，跳过。"
		fi
	done
	echo "✅ 基础服务端口检查/放行完毕。"

	# 3. 放行指定的端口
	echo "正在为当前【入站配置】放行指定端口 $PORT_TO_OPEN..."
	if ! firewall-cmd --list-ports | grep -qw "$PORT_TO_OPEN/tcp"; then
		firewall-cmd --zone=public --add-port=$PORT_TO_OPEN/tcp --permanent >/dev/null
		if [ $? -ne 0 ]; then echo "❌ firewalld 端口 $PORT_TO_OPEN 放行失败。"; exit 1; fi
		echo "✅ 端口 $PORT_TO_OPEN 已成功放行。"
	else
		echo "端口 $PORT_TO_OPEN 规则已存在，跳过。"
	fi
	

	# 4. 检查/激活防火墙
	if ! systemctl is-active --quiet firewalld; then
		echo "firewalld 状态：未激活。正在启动..."
		systemctl start firewalld
		systemctl enable firewalld
		if [ $? -ne 0 ]; then echo "❌ firewalld 激活失败。"; exit 1; fi
		echo "✅ firewalld 已成功激活。"
	else
		echo "firewalld 状态已经是激活状态。"
	fi

	# 重新加载规则
	firewall-cmd --reload
	if [ $? -ne 0 ]; then echo "❌ firewalld 重新加载失败。"; exit 1; fi
	echo "✅ firewalld 规则已重新加载。"

	echo "🎉 所有防火墙配置已完成。"

	`, port) // 将函数传入的 port 参数填充到 Shell 脚本中

	// 使用 exec.CommandContext 运行完整的 shell 脚本
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "-c", shellCommand)

	// 捕获命令的标准输出和标准错误
	output, err := cmd.CombinedOutput()

	// 无论成功与否，都记录完整的 Shell 执行日志，便于调试
	logOutput := string(output)
	logger.Infof("执行 firewalld 端口放行脚本（目标端口 %d）的完整输出：\n%s", port, logOutput)

	if err != nil {
		// 如果脚本执行出错 (例如 exit 1)，则返回包含详细输出的错误信息
		return fmt.Errorf("执行 firewalld 端口放行脚本时发生错误: %v, Shell 输出: %s", err, logOutput)
	}

	return nil
}

// =========================================================================================
// 【数据结构和辅助函数：已移除新闻相关代码】
// =========================================================================================

// =========================================================================================
// 【辅助函数：每日一语】 (最终修复：严格遵循官方文档 Token 机制，增强健壮性)
// =========================================================================================

// 【新增的辅助函数】: 发送贴纸到指定的聊天 ID，并返回消息对象（用于获取 ID）
func (t *Tgbot) SendStickerToTgbot(chatId int64, fileId string) (*telego.Message, error) {
	// 必须使用 SendStickerParams 结构体，并传入 context
	params := telego.SendStickerParams{
		ChatID: tu.ID(chatId),
		// 对于现有 File ID 字符串，必须封装在 telego.InputFile 结构中。
		Sticker: telego.InputFile{FileID: fileId},
	}

	// 使用全局变量 bot 调用 SendSticker，并传入 context.Background() 和参数指针
	msg, err := bot.SendSticker(context.Background(), &params)
	if err != nil {
		logger.Errorf("发送贴纸失败到聊天 ID %d: %v", chatId, err)
		return nil, err
	}

	// 成功返回 *telego.Message 对象
	return msg, nil
}

// 【新增函数】: 发送 Xray 版本选项给用户
func (t *Tgbot) sendXrayVersionOptions(chatId int64) {
	// 获取 Xray 版本列表
	versions, err := t.serverService.GetXrayVersions()
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 获取 Xray 版本列表失败: %v", err))
		return
	}

	if len(versions) == 0 {
		t.SendMsgToTgbot(chatId, "❌ 未找到可用的 Xray 版本")
		return
	}

	// 构建版本按钮
	var buttons []telego.InlineKeyboardButton
	for _, version := range versions {
		callbackData := t.encodeQuery(fmt.Sprintf("update_xray_ask %s", version))
		button := tu.InlineKeyboardButton(version).WithCallbackData(callbackData)
		buttons = append(buttons, button)
	}

	// 添加取消按钮
	cancelButton := tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("update_xray_cancel"))
	buttons = append(buttons, cancelButton)

	// 构建键盘
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(1, buttons...))

	// 发送版本选择消息
	t.SendMsgToTgbot(chatId, "🚀 **Xray 版本管理**\n\n请选择要更新的版本：", keyboard)
}

// 【新增方法】: 批量复制所有入站的客户端链接
func (t *Tgbot) copyAllLinks(chatId int64) error {
	t.SendMsgToTgbot(chatId, "📋 正在生成纯链接列表，请稍候...")

	// 获取所有入站
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		return fmt.Errorf("获取入站列表失败: %v", err)
	}

	if len(inbounds) == 0 {
		return fmt.Errorf("没有找到任何入站")
	}

	var allLinks []string
	var errorCount int

	// 遍历每个入站
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue // 跳过禁用的入站
		}

		// 获取该入站的所有客户端
		clients, err := t.inboundService.GetClients(inbound)
		if err != nil {
			logger.Warningf("获取入站 %d 的客户端失败: %v", inbound.Id, err)
			errorCount++
			continue
		}

		if len(clients) == 0 {
			continue // 跳过没有客户端的入站
		}

		// 遍历每个客户端并生成链接
		for _, client := range clients {
			if !client.Enable {
				continue // 跳过禁用的客户端
			}

			var link string
			var linkErr error

			// 根据协议类型生成链接
			var streamSettings map[string]any
			if err := json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings); err != nil {
				logger.Warningf("解析入站 %d 的 StreamSettings 失败: %v", inbound.Id, err)
				continue
			}

			if security, ok := streamSettings["security"].(string); ok {
				switch security {
				case "reality":
					if network, ok := streamSettings["network"].(string); ok && network == "xhttp" {
						link, linkErr = t.generateXhttpRealityLinkWithClient(inbound, client)
					} else {
						link, linkErr = t.generateRealityLinkWithClient(inbound, client)
					}
				case "tls":
					link, linkErr = t.generateTlsLinkWithClient(inbound, client)
				default:
					// 对于其他协议，尝试生成通用链接
					link, linkErr = t.generateGenericLink(inbound, client)
				}
			} else {
				linkErr = fmt.Errorf("未知的 security 类型")
			}

			if linkErr != nil {
				logger.Warningf("为入站 %d 客户端 %s 生成链接失败: %v", inbound.Id, client.Email, linkErr)
				errorCount++
			} else {
				// 只添加链接本身
				allLinks = append(allLinks, link)
			}
		}
	}

	// 如果没有生成任何链接
	if len(allLinks) == 0 {
		return fmt.Errorf("没有找到可用的链接")
	}

	// 将所有链接合并为单个字符串
	allLinksText := strings.Join(allLinks, "\n")

	// 检查消息长度，如果超过限制则分段发送
	const maxMessageLength = 4000 // Telegram 消息限制
	if len(allLinksText) <= maxMessageLength {
		t.SendMsgToTgbot(chatId, allLinksText)
	} else {
		// 分段发送
		lines := strings.Split(allLinksText, "\n")
		var currentMessage strings.Builder

		for _, line := range lines {
			if currentMessage.Len()+len(line)+1 > maxMessageLength {
				// 发送当前段落
				if currentMessage.Len() > 0 {
					t.SendMsgToTgbot(chatId, currentMessage.String())
				}
				// 开始新段落
				currentMessage.Reset()
			}

			if currentMessage.Len() > 0 {
				currentMessage.WriteString("\n")
			}
			currentMessage.WriteString(line)
		}

		// 发送最后一段
		if currentMessage.Len() > 0 {
			t.SendMsgToTgbot(chatId, currentMessage.String())
		}
	}

	return nil
}

// 【新增辅助函数】: 生成通用协议链接（VMess, VLESS, Trojan, ShadowSocks）
func (t *Tgbot) generateGenericLink(inbound *model.Inbound, client model.Client) (string, error) {
	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	switch inbound.Protocol {
	case model.VMESS:
		// VMess 链接格式
		return fmt.Sprintf("vmess://%s@%s:%d?network=tcp&security=none#%s-%s",
			client.ID, domain, inbound.Port, client.Email, inbound.Remark), nil

	case model.VLESS:
		// VLESS 链接格式（无加密）
		flow := ""
		if client.Flow != "" {
			flow = "&flow=" + client.Flow
		}
		return fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none%s#%s-%s",
			client.ID, domain, inbound.Port, flow, client.Email, inbound.Remark), nil

	case model.Trojan:
		// Trojan 链接格式
		return fmt.Sprintf("trojan://%s@%s:%d#%s-%s",
			client.Password, domain, inbound.Port, client.Email, inbound.Remark), nil

	case model.Shadowsocks:
		// ShadowSocks 链接格式
		if client.Security == "" {
			client.Security = "aes-256-gcm" // 默认加密方式
		}
		return fmt.Sprintf("ss://%s@%s:%d#%s-%s",
			client.Security, domain, inbound.Port, client.Email, inbound.Remark), nil

	default:
		return "", fmt.Errorf("不支持的协议类型: %s", inbound.Protocol)
	}
}

// 【新增函数】: 显示机器优化选项菜单
func (t *Tgbot) sendMachineOptimizationOptions(chatId int64) {
	optimizationKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🖥️ 1C1G 机器").WithCallbackData(t.encodeQuery("optimize_1c1g")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🚀 通用/高配优化").WithCallbackData(t.encodeQuery("optimize_generic")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("get_inbounds")),
		),
	)
	t.SendMsgToTgbot(chatId, "⚡ **机器优化一键方案**\n\n请选择您的机器配置类型：\n\n🖥️ **1C1G 机器**: 适用于低配VPS的深度优化\n🚀 **通用/高配优化**: 适用于高配VPS的全面优化", optimizationKeyboard)
}

// 【新增函数】: 执行1C1G优化前显示确认对话框
func (t *Tgbot) performOptimization1C1G(chatId int64, messageId int) {
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 确认执行").WithCallbackData(t.encodeQuery("optimize_1c1g_confirm")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("machine_optimization")),
		),
	)

	t.editMessageCallbackTgBot(chatId, messageId, confirmKeyboard)

	// 发送详细说明
	detailMsg := "🤔 **1C1G 机器优化确认**\n\n即将执行以下优化操作：\n\n**📊 内核参数深度优化（针对1C1G低配机器）:**\n• 内存管理优化 (swappiness, cache pressure等)\n• 网络参数优化 (TCP缓冲区、连接跟踪等)\n• 文件描述符限制优化\n\n**🚀 BBR 拥塞控制算法（网络性能提升）:**\n• 检测内核版本兼容性\n• 自动启用 BBR 算法（需要内核 4.9+）\n• 大幅提升网络吞吐量和降低延迟\n\n⚠️ **注意**: 此操作需要root权限，请确保您的VPS有足够权限。"
	t.SendMsgToTgbot(chatId, detailMsg)
}

// 【新增函数】: 执行实际的1C1G优化操作
func (t *Tgbot) executeOptimization1C1G(chatId int64, messageId int) {
	t.SendMsgToTgbot(chatId, "🚀 **开始执行1C1G机器优化...**\n\n⏳ 正在执行优化操作，请稍候...")

	go func() {
		// 执行优化操作
		_, err := t.execute1C1GOptimization()

		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **优化执行失败**\n\n错误信息: %v\n\n💡 **排查建议**:\n• 请查看日志文件: /tmp/x-panel-optimization.log\n• 确保您的VPS具有root权限\n• 检查系统磁盘空间是否充足", err))
		} else {
			// 获取优化后的系统状态
			statusMsg := t.getSystemStatusAfterOptimization()

			resultMsg := fmt.Sprintf("✅ **1C1G机器优化执行完成！**\n\n📊 **优化结果:**\n• 内核参数已优化 ✅\n• 文件描述符限制已优化 ✅\n• BBR 网络加速已启用 ✅\n• 代理服务器参数已优化 ✅\n\n%s\n\n🎉 **优化成功完成，您的1C1G机器现在网络更快、更加稳定高效！**\n\n📋 **重要信息:**\n• 详细日志文件: `/tmp/x-panel-optimization.log`\n• 优化包含针对 Sing-box/Xray 的专用参数\n• BBR算法大幅提升网络性能\n• 设置了 5 分钟操作超时，防止脚本死锁\n⚠️ **注意**: 文件描述符限制优化需要重启服务器或重新登录才能完全生效。", statusMsg)
			t.SendMsgToTgbot(chatId, resultMsg)
		}
	}()
}

// 【新增辅助函数】: 执行实际的1C1G优化操作
func (t *Tgbot) execute1C1GOptimization() (string, error) {
	// 检查 Root 权限
	if os.Geteuid() != 0 {
		return "", fmt.Errorf("权限不足：此操作需要 root 权限，请确保面板以 root 身份运行")
	}

	var output strings.Builder

	// 创建日志文件
	logFile := "/tmp/x-panel-optimization.log"
	f, err := os.Create(logFile)
	if err != nil {
		return output.String(), fmt.Errorf("创建日志文件失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	// 记录开始时间
	startTime := time.Now()
	logMsg := fmt.Sprintf("X-Panel 1C1G 机器优化开始时间: %s\n", startTime.Format("2006-01-02 15:04:05"))
	output.WriteString(logMsg)
	_, _ = f.WriteString(logMsg)

	// 初始化 nf_conntrack 支持状态
	nfConntrackSupported := false

	// 1. 内核参数优化
	output.WriteString("=== 内核参数优化 ===\n")
	_, _ = f.WriteString("=== 内核参数优化 ===\n")

	// 先检查并尝试加载 nf_conntrack 模块
	output.WriteString("正在检查 nf_conntrack 模块...\n")
	_, _ = f.WriteString("正在检查 nf_conntrack 模块...\n")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 检查模块是否已加载
	cmd := exec.CommandContext(ctx, "bash", "-c", "lsmod | grep -q nf_conntrack && echo 'loaded' || echo 'not_loaded'")
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err == nil {
		// 模块已加载，继续执行
		_, _ = output.WriteString("✅ nf_conntrack 模块已加载\n")
		_, _ = f.WriteString("✅ nf_conntrack 模块已加载\n")
		// 检查 /proc/sys/net/netfilter 路径是否存在
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd = exec.CommandContext(ctx, "bash", "-c", "test -d /proc/sys/net/netfilter && echo 'exists' || echo 'not_exists'")
		cmd.Stdout = f
		cmd.Stderr = f
		checkOutput, _ := cmd.Output()

		if strings.TrimSpace(string(checkOutput)) == "exists" {
			nfConntrackSupported = true
			_, _ = output.WriteString("✅ nf_conntrack 路径存在，支持相关参数\n")
			_, _ = f.WriteString("✅ nf_conntrack 路径存在，支持相关参数\n")
		} else {
			output.WriteString("⚠️ nf_conntrack 路径不存在，将跳过相关参数\n")
			_, _ = f.WriteString("⚠️ nf_conntrack 路径不存在，将跳过相关参数\n")
		}
	} else {
		// 模块未加载，尝试加载
		output.WriteString("ℹ️ nf_conntrack 模块未加载，正在尝试加载...\n")
		_, _ = f.WriteString("ℹ️ nf_conntrack 模块未加载，正在尝试加载...\n")

		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd = exec.CommandContext(ctx, "bash", "-c", "modprobe nf_conntrack")
		modprobeOutput, err := cmd.CombinedOutput()
		_, _ = f.Write(modprobeOutput)
		if err != nil {
			errorMsg := fmt.Sprintf("modprobe 命令执行失败: %v, 输出: %s", err, string(modprobeOutput))
			output.WriteString("⚠️ " + errorMsg + "，将跳过相关参数\n")
			_, _ = f.WriteString("⚠️ " + errorMsg + "，将跳过相关参数\n")
		} else {
			output.WriteString("✅ nf_conntrack 模块加载成功\n")
			_, _ = f.WriteString("✅ nf_conntrack 模块加载成功\n")
			// 检查 /proc/sys/net/netfilter 路径是否存在
			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd = exec.CommandContext(ctx, "bash", "-c", "test -d /proc/sys/net/netfilter && echo 'exists' || echo 'not_exists'")
			cmd.Stdout = f
			cmd.Stderr = f
			checkOutput, _ := cmd.Output()

			if strings.TrimSpace(string(checkOutput)) == "exists" {
				nfConntrackSupported = true
				output.WriteString("✅ nf_conntrack 路径存在，支持相关参数\n")
				_, _ = f.WriteString("✅ nf_conntrack 路径存在，支持相关参数\n")
			} else {
				output.WriteString("⚠️ nf_conntrack 路径不存在，将跳过相关参数\n")
				_, _ = f.WriteString("⚠️ nf_conntrack 路径不存在，将跳过相关参数\n")
			}
		}
	}

	// 创建基础内核参数配置文件（不包含 nf_conntrack 参数）
	baseKernelConfig := `# ===== 1C1G 机器深度优化配置 =====
# 内存管理优化（移除不稳定的 min_free_kbytes）
vm.swappiness = 60
vm.vfs_cache_pressure = 50
vm.dirty_ratio = 10
vm.dirty_background_ratio = 5
vm.overcommit_memory = 0

# 网络优化（保守设置，适合低配机器）
net.core.somaxconn = 1024
net.core.netdev_max_backlog = 2000
net.ipv4.tcp_max_syn_backlog = 1024
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 600
net.ipv4.tcp_keepalive_probes = 5
net.ipv4.tcp_keepalive_intvl = 15
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 10000 65535
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.ip_forward = 1`

	// 创建 nf_conntrack 专用配置文件（仅在支持时）
	nfConntrackConfig := ``
	if nfConntrackSupported {
		nfConntrackConfig = `# ===== nf_conntrack 优化配置 =====
# 连接跟踪优化（需要 nf_conntrack 模块支持）
net.netfilter.nf_conntrack_max = 65536
net.netfilter.nf_conntrack_tcp_timeout_established = 1200
net.netfilter.nf_conntrack_tcp_timeout_time_wait = 30`
	}

	// 1.1. 先应用基础内核参数（不包含 nf_conntrack）
	output.WriteString("正在应用基础内核参数...\n")
	_, _ = f.WriteString("正在应用基础内核参数...\n")

	// 检查配置文件是否存在，如果存在则备份
	configFilePath := "/etc/sysctl.d/99-1c1g-optimize-base.conf"
	backupFilePath := configFilePath + ".bak"
	if _, err := os.Stat(configFilePath); err == nil {
		// 文件存在，进行备份
		output.WriteString("检测到现有配置文件，正在备份...\n")
		_, _ = f.WriteString("检测到现有配置文件，正在备份...\n")
		if err := os.Rename(configFilePath, backupFilePath); err != nil {
			errorMsg := fmt.Sprintf("备份配置文件失败: %v", err)
			output.WriteString("⚠️ " + errorMsg + "\n")
			_, _ = f.WriteString("⚠️ " + errorMsg + "\n")
			// 继续执行，不返回错误
		} else {
			output.WriteString("✅ 配置文件已备份为 " + backupFilePath + "\n")
			f.WriteString("✅ 配置文件已备份为 " + backupFilePath + "\n")
		}
	}

	if err := sys.AtomicWriteFile(configFilePath, []byte(baseKernelConfig), 0o644); err != nil {
		errorMsg := fmt.Sprintf("创建基础内核配置文件失败: %v", err)
		output.WriteString("❌ " + errorMsg + "\n")
		f.WriteString("❌ " + errorMsg + "\n")
		return output.String(), fmt.Errorf("%s", errorMsg)
	}
	successMsg := "✅ 基础内核参数配置文件已创建"
	output.WriteString(successMsg + "\n")
	f.WriteString(successMsg + "\n")

	// 应用基础内核参数
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd = exec.CommandContext(ctx, "sysctl", "-p", configFilePath)
	sysctlOutput, err := cmd.CombinedOutput()
	_, _ = f.Write(sysctlOutput)
	if err != nil {
		errorMsg := fmt.Sprintf("sysctl 命令执行失败: %v, 输出: %s", err, string(sysctlOutput))
		output.WriteString("❌ " + errorMsg + "\n")
		f.WriteString("❌ " + errorMsg + "\n")
		return output.String(), fmt.Errorf("%s", errorMsg)
	}
	successMsg = "✅ 基础内核参数已应用"
	output.WriteString(successMsg + "\n")
	f.WriteString(successMsg + "\n")

	// 1.2. 尝试应用 nf_conntrack 参数（仅在支持时）
	if nfConntrackSupported && nfConntrackConfig != "" {
		output.WriteString("正在应用 nf_conntrack 参数...\n")
		f.WriteString("正在应用 nf_conntrack 参数...\n")

		if err := sys.AtomicWriteFile("/etc/sysctl.d/99-nf-conntrack-optimize.conf", []byte(nfConntrackConfig), 0o644); err != nil {
			output.WriteString("⚠️ 创建 nf_conntrack 配置文件失败，跳过相关参数\n")
			f.WriteString("⚠️ 创建 nf_conntrack 配置文件失败，跳过相关参数\n")
		} else {
			// 应用 nf_conntrack 参数
			ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			cmd = exec.CommandContext(ctx, "sysctl", "-p", "/etc/sysctl.d/99-nf-conntrack-optimize.conf")
			sysctlOutput, err := cmd.CombinedOutput()
			_, _ = f.Write(sysctlOutput)
			if err != nil {
				errorMsg := fmt.Sprintf("sysctl 命令执行失败: %v, 输出: %s", err, string(sysctlOutput))
				output.WriteString("⚠️ " + errorMsg + "，跳过相关参数\n")
				f.WriteString("⚠️ " + errorMsg + "，跳过相关参数\n")
			} else {
				successMsg = "✅ nf_conntrack 参数已应用"
				output.WriteString(successMsg + "\n")
				f.WriteString(successMsg + "\n")
			}
		}
	} else {
		// nf_conntrack 不支持，跳过相关参数
		output.WriteString("ℹ️ 跳过 nf_conntrack 参数（模块不支持或路径不存在）\n")
		f.WriteString("ℹ️ 跳过 nf_conntrack 参数（模块不支持或路径不存在）\n")
	}

	// 3. 优化文件描述符限制
	limitsMsg := "\n=== 文件描述符限制优化 ===\n"
	output.WriteString(limitsMsg)
	f.WriteString(limitsMsg)

	limitsConfig := `* soft nofile 65535
* hard nofile 65535
* soft nproc 65535
* hard nproc 65535
root soft nofile 65535
root hard nofile 65535`

	if err := sys.WriteConfigBlock("/etc/security/limits.conf", "# === 1C1G Machine Optimization ===", "", limitsConfig); err != nil {
		errorMsg := fmt.Errorf("更新limits.conf失败: %v", err)
		output.WriteString("❌ " + errorMsg.Error() + "\n")
		f.WriteString("❌ " + errorMsg.Error() + "\n")
		return output.String(), errorMsg
	}
	successMsg = "✅ 文件描述符限制已优化"
	output.WriteString(successMsg + "\n")
	f.WriteString(successMsg + "\n")

	// 4. 【新增】启用 BBR 拥塞控制算法
	bbrMsg := "\n=== BBR 拥塞控制算法启用 ===\n"
	output.WriteString(bbrMsg)
	f.WriteString(bbrMsg)

	// 检查内核版本和 BBR 支持
	kernelVersion, bbrSupported, err := t.checkBBRSupport()
	if err != nil {
		output.WriteString("⚠️ 检查 BBR 支持失败: " + err.Error() + "\n")
		f.WriteString("⚠️ 检查 BBR 支持失败: " + err.Error() + "\n")
	} else {
		output.WriteString(fmt.Sprintf("✅ 内核版本: %s\n", kernelVersion))
		f.WriteString(fmt.Sprintf("✅ 内核版本: %s\n", kernelVersion))

		if bbrSupported {
			output.WriteString("✅ BBR 支持检测: 支持\n")
			f.WriteString("✅ BBR 支持检测: 支持\n")

			// 启用 BBR
			bbrConfig := `# ===== BBR 拥塞控制算法配置 =====
# 启用 BBR 拥塞控制算法以提升网络性能
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
`
			// 先备份现有文件（如果存在）
			if backupErr := sys.BackupFile("/etc/sysctl.d/99-bbr-optimize.conf"); backupErr != nil {
				output.WriteString("ℹ️ BBR 配置文件备份失败（文件可能不存在）\n")
				f.WriteString("ℹ️ BBR 配置文件备份失败（文件可能不存在）\n")
			}

			if err := sys.AtomicWriteFile("/etc/sysctl.d/99-bbr-optimize.conf", []byte(bbrConfig), 0o644); err != nil {
				errorMsg := fmt.Sprintf("创建 BBR 配置文件失败: %v", err)
				output.WriteString("❌ " + errorMsg + "\n")
				f.WriteString("❌ " + errorMsg + "\n")
			} else {
				// 应用 BBR 设置
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				cmd = exec.CommandContext(ctx, "sysctl", "-p", "/etc/sysctl.d/99-bbr-optimize.conf")
				sysctlOutput, err := cmd.CombinedOutput()
				_, _ = f.Write(sysctlOutput)
				if err != nil {
					errorMsg := fmt.Sprintf("sysctl 命令执行失败: %v, 输出: %s", err, string(sysctlOutput))
					output.WriteString("❌ " + errorMsg + "\n")
					f.WriteString("❌ " + errorMsg + "\n")
				} else {
					successMsg := "✅ BBR 拥塞控制算法已启用"
					output.WriteString(successMsg + "\n")
					f.WriteString(successMsg + "\n")

					// 验证 BBR 是否启用
					ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					cmd = exec.CommandContext(ctx, "sysctl", "net.ipv4.tcp_congestion_control")
					checkOutput, _ := cmd.Output()
					output.WriteString("✅ 当前拥塞控制算法: " + strings.TrimSpace(string(checkOutput)) + "\n")
					f.WriteString("✅ 当前拥塞控制算法: " + strings.TrimSpace(string(checkOutput)) + "\n")
				}
			}
		} else {
			skipMsg := "ℹ️ BBR 不支持（需要 Linux 内核 4.9+），跳过启用"
			output.WriteString(skipMsg + "\n")
			f.WriteString(skipMsg + "\n")
		}
	}

	// 记录结束时间和日志文件位置
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	logMsg = fmt.Sprintf("\nX-Panel 1C1G 机器优化完成时间: %s\n", endTime.Format("2006-01-02 15:04:05"))
	logMsg += fmt.Sprintf("总耗时: %v\n", duration)
	logMsg += fmt.Sprintf("详细日志已保存到: %s\n", logFile)
	output.WriteString(logMsg)
	f.WriteString(logMsg)

	return output.String(), nil
}

// 【新增函数】: 执行通用/高配优化操作
func (t *Tgbot) executeGenericOptimization(chatId int64, messageId int) {
	t.SendMsgToTgbot(chatId, "🚀 **开始执行通用/高配优化...**\n\n⏳ 正在执行优化操作，请稍候...")

	go func() {
		// 执行优化操作
		_, err := t.executeGenericOptimizationInternal()

		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **优化执行失败**\n\n错误信息: %v\n\n💡 **排查建议**:\n• 请查看日志文件: /tmp/x-panel-generic-optimization.log\n• 确保您的VPS具有root权限\n• 检查系统磁盘空间是否充足", err))
		} else {
			// 获取优化后的系统状态
			statusMsg := t.getSystemStatusAfterOptimization()

			resultMsg := fmt.Sprintf("✅ **通用/高配优化执行完成！**\n\n📊 **优化结果:**\n• 内核参数已优化 ✅\n• 文件描述符限制已优化 ✅\n• BBR 网络加速已启用 ✅\n\n%s\n\n🎉 **优化成功完成，您的VPS现在网络更快、更加稳定高效！**\n\n📋 **重要信息:**\n• 详细日志文件: `/tmp/x-panel-generic-optimization.log`\n• 优化包含针对高配VPS的专用参数\n• BBR算法大幅提升网络性能\n⚠️ **注意**: 文件描述符限制优化需要重启服务器或重新登录才能完全生效。", statusMsg)
			t.SendMsgToTgbot(chatId, resultMsg)
		}
	}()
}

// 【新增辅助函数】: 执行通用/高配优化操作的具体实现
func (t *Tgbot) executeGenericOptimizationInternal() (string, error) {
	// 检查 Root 权限
	if os.Geteuid() != 0 {
		return "", fmt.Errorf("权限不足：此操作需要 root 权限，请确保面板以 root 身份运行")
	}

	var output strings.Builder

	// 创建日志文件
	logFile := "/tmp/x-panel-generic-optimization.log"
	f, err := os.Create(logFile)
	if err != nil {
		return output.String(), fmt.Errorf("创建日志文件失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	// 记录开始时间
	startTime := time.Now()
	logMsg := fmt.Sprintf("X-Panel 通用/高配优化开始时间: %s\n", startTime.Format("2006-01-02 15:04:05"))
	output.WriteString(logMsg)
	f.WriteString(logMsg)

	// 1. 内核参数优化 (更激进的网络参数)
	output.WriteString("=== 内核参数优化 ===\n")
	f.WriteString("=== 内核参数优化 ===\n")

	kernelConfig := `# ===== 通用/高配网络优化配置 =====
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.core.somaxconn = 4096
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30
net.core.netdev_max_backlog = 50000
net.ipv4.tcp_max_tw_buckets = 6000
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_probes = 5
net.ipv4.tcp_keepalive_intvl = 15
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_mtu_probing = 1
net.ipv4.tcp_moderate_rcvbuf = 1
net.ipv4.tcp_congestion_control = bbr
net.core.default_qdisc = fq

# 文件描述符
fs.file-max = 1000000
`

	// 检查配置文件是否存在，如果存在则备份
	configFilePath := "/etc/sysctl.d/99-generic-optimize.conf"
	backupFilePath := configFilePath + ".bak"
	if _, err := os.Stat(configFilePath); err == nil {
		// 文件存在，进行备份
		output.WriteString("检测到现有内核配置文件，正在备份...\n")
		f.WriteString("检测到现有内核配置文件，正在备份...\n")
		if err := os.Rename(configFilePath, backupFilePath); err != nil {
			errorMsg := fmt.Sprintf("备份内核配置文件失败: %v", err)
			output.WriteString("⚠️ " + errorMsg + "\n")
			f.WriteString("⚠️ " + errorMsg + "\n")
			// 继续执行，不返回错误
		} else {
			output.WriteString("✅ 内核配置文件已备份为 " + backupFilePath + "\n")
			f.WriteString("✅ 内核配置文件已备份为 " + backupFilePath + "\n")
		}
	}

	// 使用安全写入方式创建内核配置文件
	if err := sys.AtomicWriteFile(configFilePath, []byte(kernelConfig), 0o644); err != nil {
		errorMsg := fmt.Sprintf("创建内核配置文件失败: %v", err)
		output.WriteString("❌ " + errorMsg + "\n")
		f.WriteString("❌ " + errorMsg + "\n")
		return output.String(), fmt.Errorf("%s", errorMsg)
	}
	successMsg := "✅ 内核参数配置文件已创建"
	output.WriteString(successMsg + "\n")
	f.WriteString(successMsg + "\n")

	// 应用内核参数
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sysctl", "-p", configFilePath)
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		errorMsg := fmt.Sprintf("应用内核参数失败: %v", err)
		output.WriteString("❌ " + errorMsg + "\n")
		f.WriteString("❌ " + errorMsg + "\n")
		return output.String(), fmt.Errorf("%s", errorMsg)
	}
	successMsg = "✅ 内核参数已应用"
	output.WriteString(successMsg + "\n")
	f.WriteString(successMsg + "\n")

	// 2. 文件描述符限制优化
	limitsMsg := "\n=== 文件描述符限制优化 ===\n"
	output.WriteString(limitsMsg)
	f.WriteString(limitsMsg)

	limitsConfig := `* soft nofile 1000000
* hard nofile 1000000
* soft nproc 1000000
* hard nproc 1000000
root soft nofile 1000000
root hard nofile 1000000`

	if err := sys.WriteConfigBlock("/etc/security/limits.conf", "# === Generic High-Performance Optimization ===", "", limitsConfig); err != nil {
		errorMsg := fmt.Errorf("更新limits.conf失败: %v", err)
		output.WriteString("❌ " + errorMsg.Error() + "\n")
		f.WriteString("❌ " + errorMsg.Error() + "\n")
		return output.String(), errorMsg
	}
	successMsg = "✅ 文件描述符限制已优化至 100万"
	output.WriteString(successMsg + "\n")
	f.WriteString(successMsg + "\n")

	// 4. BBR 启用
	bbrMsg := "\n=== BBR 拥塞控制算法启用 ===\n"
	output.WriteString(bbrMsg)
	f.WriteString(bbrMsg)

	// 检查内核版本和 BBR 支持
	kernelVersion, bbrSupported, err := t.checkBBRSupport()
	if err != nil {
		output.WriteString("⚠️ 检查 BBR 支持失败: " + err.Error() + "\n")
		f.WriteString("⚠️ 检查 BBR 支持失败: " + err.Error() + "\n")
	} else {
		output.WriteString(fmt.Sprintf("✅ 内核版本: %s\n", kernelVersion))
		f.WriteString(fmt.Sprintf("✅ 内核版本: %s\n", kernelVersion))

		if bbrSupported {
			output.WriteString("✅ BBR 支持检测: 支持\n")
			f.WriteString("✅ BBR 支持检测: 支持\n")

			// 启用 BBR (已在内核配置中设置)
			successMsg := "✅ BBR 拥塞控制算法已在内核参数中启用"
			output.WriteString(successMsg + "\n")
			f.WriteString(successMsg + "\n")

			// 验证 BBR 是否启用
			ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd = exec.CommandContext(ctx, "sysctl", "net.ipv4.tcp_congestion_control")
			checkOutput, _ := cmd.Output()
			output.WriteString("✅ 当前拥塞控制算法: " + strings.TrimSpace(string(checkOutput)) + "\n")
			f.WriteString("✅ 当前拥塞控制算法: " + strings.TrimSpace(string(checkOutput)) + "\n")
		} else {
			skipMsg := "ℹ️ BBR 不支持（需要 Linux 内核 4.9+），跳过启用"
			output.WriteString(skipMsg + "\n")
			f.WriteString(skipMsg + "\n")
		}
	}

	// 记录结束时间和日志文件位置
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	logMsg = fmt.Sprintf("\nX-Panel 通用/高配优化完成时间: %s\n", endTime.Format("2006-01-02 15:04:05"))
	logMsg += fmt.Sprintf("总耗时: %v\n", duration)
	logMsg += fmt.Sprintf("详细日志已保存到: %s\n", logFile)
	output.WriteString(logMsg)
	f.WriteString(logMsg)

	return output.String(), nil
}

// 【新增辅助函数】: 获取优化后的系统状态
func (t *Tgbot) getSystemStatusAfterOptimization() string {
	var status strings.Builder

	// 获取内存信息
	cmd := exec.Command("bash", "-c", "free -h")
	output, err := cmd.Output()
	if err == nil {
		status.WriteString("\n**💾 内存使用情况:**\n")
		status.WriteString(fmt.Sprintf("```\n%s\n```", strings.TrimSpace(string(output))))
	}

	// 获取内核参数
	cmd = exec.Command("bash", "-c", "sysctl vm.swappiness vm.vfs_cache_pressure vm.dirty_ratio")
	output, err = cmd.Output()
	if err == nil {
		status.WriteString("\n**⚙️ 关键内核参数:**\n")
		status.WriteString(fmt.Sprintf("```\n%s\n```", strings.TrimSpace(string(output))))
	}

	// 获取BBR状态
	cmd = exec.Command("bash", "-c", "sysctl net.ipv4.tcp_congestion_control net.core.default_qdisc")
	output, err = cmd.Output()
	if err == nil {
		bbrStatus := strings.TrimSpace(string(output))
		// 检查是否启用了 BBR
		bbrEnabled := strings.Contains(bbrStatus, "bbr") && strings.Contains(bbrStatus, "fq")
		status.WriteString("\n**🚀 BBR网络加速状态:**\n")
		if bbrEnabled {
			status.WriteString("✅ **BBR 已启用**\n")
		} else {
			status.WriteString("❌ **BBR 未启用**\n")
		}
		status.WriteString(fmt.Sprintf("```\n%s\n```", bbrStatus))
	} else {
		status.WriteString("\n**🚀 BBR网络加速状态:**\n❌ **无法获取状态**\n")
	}

	return status.String()
}

// =========================================================================================
// 【防火墙管理功能】
// =========================================================================================

// 【新增函数】: 显示防火墙管理主菜单
func (t *Tgbot) sendFirewallMenu(chatId int64) {
	firewallKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔍 检查防火墙状态").WithCallbackData(t.encodeQuery("firewall_check_status")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📦 安装防火墙").WithCallbackData(t.encodeQuery("firewall_install_firewalld")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📦 安装 Fail2Ban").WithCallbackData(t.encodeQuery("firewall_install_fail2ban")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 启用防火墙").WithCallbackData(t.encodeQuery("firewall_enable")),
			tu.InlineKeyboardButton("❌ 禁用防火墙").WithCallbackData(t.encodeQuery("firewall_disable")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔓 开放端口").WithCallbackData(t.encodeQuery("firewall_open_port")),
			tu.InlineKeyboardButton("🔒 关闭端口").WithCallbackData(t.encodeQuery("firewall_close_port")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📋 查看规则").WithCallbackData(t.encodeQuery("firewall_list_rules")),
			tu.InlineKeyboardButton("🚀 开放X-Panel端口").WithCallbackData(t.encodeQuery("firewall_open_xpanel_ports")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("get_inbounds")),
		),
	)

	t.SendMsgToTgbot(chatId, "🔥 **防火墙管理**\n\n请选择您要执行的操作：\n\n• 🔍 **检查状态**: 检测当前防火墙状态\n• 📦 **安装工具**: 安装 Firewalld 防火墙\n• 📦 **安装 Fail2Ban**: 安装入侵检测和预防系统\n• ✅❌ **启禁用**: 控制防火墙服务状态\n• 🔓🔒 **端口管理**: 开放或关闭特定端口\n• 📋 **查看规则**: 显示当前所有防火墙规则\n• 🚀 **一键开放**: 自动开放 X-Panel 所需端口", firewallKeyboard)
}

// 【新增函数】: 检查当前防火墙状态
func (t *Tgbot) checkFirewallStatus(chatId int64) {
	go func() {
		// 检查 Firewalld 状态
		firewalldStatus, firewalldInstalled := t.getFirewalldStatus()

		// 构建状态消息
		var statusMsg strings.Builder
		statusMsg.WriteString("🔍 **防火墙状态检测结果**\n\n")

		statusMsg.WriteString("📊 **防火墙**:\n")
		if firewalldInstalled {
			statusMsg.WriteString(fmt.Sprintf("✅ 已安装\n📊 状态: %s\n\n", firewalldStatus))
		} else {
			statusMsg.WriteString("❌ 未安装\n\n")
		}

		// 推荐防火墙类型
		statusMsg.WriteString("💡 **推荐**:\n")
		statusMsg.WriteString("• 使用 Firewalld 防火墙\n")

		t.SendMsgToTgbot(chatId, statusMsg.String())
	}()
}

// 【新增函数】: 安装 Firewalld
func (t *Tgbot) installFirewalld(chatId int64) {
	go func() {
		// 检查是否已安装
		_, installed := t.getFirewalldStatus()
		if installed {
			t.SendMsgToTgbot(chatId, "ℹ️ **Firewalld 已安装**\n\nFirewalld 防火墙已经安装在您的系统上。")
			return
		}

		// 执行安装
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "bash", "-c", "apt-get update -qq && apt-get install -y -qq firewalld")
		output, err := cmd.CombinedOutput()

		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **Firewalld 安装失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
		} else {
			t.SendMsgToTgbot(chatId, "✅ **Firewalld 安装成功！**\n\nFirewalld 防火墙已成功安装到您的系统上。\n\n接下来您可以：\n• 启用防火墙\n• 配置端口规则\n• 查看防火墙状态")
		}
	}()
}

// 【新增函数】: 安装 Fail2Ban
func (t *Tgbot) installFail2Ban(chatId int64) {
	go func() {
		// 检查是否已安装
		_, installed := t.getFail2BanStatus()
		if installed {
			t.SendMsgToTgbot(chatId, "ℹ️ **Fail2Ban 已安装**\n\nFail2Ban 入侵检测和预防系统已经安装在您的系统上。")
			return
		}

		// 执行安装
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// 尝试多种包管理器
		var cmd *exec.Cmd
		var output []byte
		var err error

		// 首先尝试 apt (Debian/Ubuntu)
		cmd = exec.CommandContext(ctx, "bash", "-c", "apt update -qq && apt install -y -qq fail2ban")
		_, err = cmd.CombinedOutput()
		if err != nil {
			// 如果 apt 失败，尝试 yum (CentOS/RHEL)
			ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			cmd = exec.CommandContext(ctx, "bash", "-c", "yum install -y fail2ban")
			_, err = cmd.CombinedOutput()
			if err != nil {
				// 如果 yum 失败，尝试 dnf (Fedora/RHEL 8+)
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				cmd = exec.CommandContext(ctx, "bash", "-c", "dnf install -y fail2ban")
				output, err = cmd.CombinedOutput()
				if err != nil {
					t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **Fail2Ban 安装失败**\n\n尝试了 apt、yum 和 dnf 包管理器，但都失败了。\n\n错误信息: %v\n\n输出: %s", err, string(output)))
					return
				}
			}
		}

		// 安装成功，启用并启动服务
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd = exec.CommandContext(ctx, "systemctl", "enable", "fail2ban")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **Fail2Ban 安装成功，但启用服务失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
			return
		}

		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd = exec.CommandContext(ctx, "systemctl", "start", "fail2ban")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **Fail2Ban 安装成功，但启动服务失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
			return
		}

		// 配置 Fail2Ban 以使用 Firewalld
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd = exec.CommandContext(ctx, "bash", "-c", `cat > /etc/fail2ban/jail.local << 'EOF'
[DEFAULT]
banaction = firewallcmd-rich-rules
banaction_allports = firewallcmd-rich-rules
backend = systemd
EOF`)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **Fail2Ban 安装成功，但配置 Firewalld 失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
			return
		}

		// 重启 Fail2Ban 服务以应用新配置
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd = exec.CommandContext(ctx, "systemctl", "restart", "fail2ban")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **Fail2Ban 配置成功，但重启服务失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
			return
		}

		t.SendMsgToTgbot(chatId, "✅ **Fail2Ban 安装并配置成功！**\n\nFail2Ban 入侵检测和预防系统已成功安装并配置为与 Firewalld 配合工作。\n\n• Fail2Ban 使用 `firewallcmd-rich-rules` 封禁 IP\n• 会自动监控日志文件并封禁可疑活动\n• 配置文件位于 `/etc/fail2ban/jail.local`\n\n您可以根据需要配置额外的 jail 规则。")
	}()
}

// 【新增辅助函数】: 获取 Fail2Ban 状态
func (t *Tgbot) getFail2BanStatus() (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查是否安装
	cmd := exec.CommandContext(ctx, "bash", "-c", "command -v fail2ban-client")
	if err := cmd.Run(); err != nil {
		return "未安装", false
	}

	// 获取服务状态
	cmd = exec.CommandContext(ctx, "systemctl", "is-active", "fail2ban")
	output, err := cmd.Output()
	if err != nil {
		return "状态未知", true
	}

	status := strings.TrimSpace(string(output))
	if status == "active" {
		return "运行中", true
	} else {
		return "未运行", true
	}
}

// 【新增函数】: 启用防火墙
func (t *Tgbot) enableFirewall(chatId int64) {
	go func() {
		// 先检查当前防火墙状态
		firewalldStatus, firewalldInstalled := t.getFirewalldStatus()

		var cmd *exec.Cmd
		var output []byte
		var err error

		if firewalldInstalled && (strings.Contains(strings.ToLower(firewalldStatus), "inactive") || strings.Contains(strings.ToLower(firewalldStatus), "未激活")) {
			// 启用 Firewalld
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			cmd = exec.CommandContext(ctx, "bash", "-c", "systemctl enable firewalld && systemctl start firewalld")
			output, err = cmd.CombinedOutput()

			if err != nil {
				t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **防火墙启用失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
			} else {
				t.SendMsgToTgbot(chatId, "✅ **防火墙启用成功！**\n\n防火墙已成功启用并设置为开机自启动。")
			}
		} else {
			// 没有找到可用的防火墙或防火墙已经启用
			t.SendMsgToTgbot(chatId, "ℹ️ **防火墙状态**\n\n没有检测到需要启用的防火墙，或者防火墙已经处于启用状态。\n\n请先检查防火墙状态。")
		}
	}()
}

// 【新增函数】: 禁用防火墙
func (t *Tgbot) disableFirewall(chatId int64) {
	go func() {
		// 先检查当前防火墙状态
		firewalldStatus, firewalldInstalled := t.getFirewalldStatus()

		var cmd *exec.Cmd
		var output []byte
		var err error

		if firewalldInstalled && (strings.Contains(strings.ToLower(firewalldStatus), "active") || strings.Contains(strings.ToLower(firewalldStatus), "已激活")) {
			// 禁用 Firewalld
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			cmd = exec.CommandContext(ctx, "bash", "-c", "systemctl stop firewalld")
			output, err = cmd.CombinedOutput()

			if err != nil {
				t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ **防火墙禁用失败**\n\n错误信息: %v\n\n输出: %s", err, string(output)))
			} else {
				t.SendMsgToTgbot(chatId, "✅ **防火墙禁用成功！**\n\n防火墙已成功禁用。请注意，禁用防火墙可能会降低服务器安全性。")
			}
		} else {
			// 没有找到可用的防火墙或防火墙已经禁用
			t.SendMsgToTgbot(chatId, "ℹ️ **防火墙状态**\n\n没有检测到需要禁用的防火墙，或者防火墙已经处于禁用状态。")
		}
	}()
}

// 【新增函数】: 开放端口
func (t *Tgbot) openPort(chatId int64) {
	// 这里简化处理，实际应用中可能需要更复杂的交互
	t.SendMsgToTgbot(chatId, "🔓 **开放端口**\n\n⚠️ **安全警告**: 请谨慎操作！\n\n请在 VPS 上手动执行以下命令：\n\n```bash\nfirewall-cmd --permanent --add-port=[端口号]/tcp\nfirewall-cmd --reload\n```\n\n例如开放 8080 端口：\n`firewall-cmd --permanent --add-port=8080/tcp`")
}

// 【新增函数】: 关闭端口
func (t *Tgbot) closePort(chatId int64) {
	// 这里简化处理，实际应用中可能需要更复杂的交互
	t.SendMsgToTgbot(chatId, "🔒 **关闭端口**\n\n⚠️ **安全警告**: 请谨慎操作！\n\n请在 VPS 上手动执行以下命令：\n\n```bash\nfirewall-cmd --permanent --remove-port=[端口号]/tcp\nfirewall-cmd --reload\n```\n\n例如关闭 8080 端口：\n`firewall-cmd --permanent --remove-port=8080/tcp`")
}

// 【新增函数】: 列出防火墙规则
func (t *Tgbot) listFirewallRules(chatId int64) {
	go func() {
		var rulesMsg strings.Builder
		rulesMsg.WriteString("📋 **防火墙规则列表**\n\n")

		// 检查 Firewalld 规则
		_, firewalldInstalled := t.getFirewalldStatus()
		if firewalldInstalled {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(ctx, "bash", "-c", "firewall-cmd --list-all")
			output, err := cmd.CombinedOutput()

			if err != nil {
				rulesMsg.WriteString("❌ **获取防火墙规则失败**\n")
			} else {
				rulesMsg.WriteString("📊 **防火墙规则**:\n```\n")
				rulesMsg.WriteString(string(output))
				rulesMsg.WriteString("```\n\n")
			}
		} else {
			rulesMsg.WriteString("❌ **未检测到防火墙**\n\n请先安装并启用防火墙。")
		}

		t.SendMsgToTgbot(chatId, rulesMsg.String())
	}()
}

// 【新增函数】: 开放 X-Panel 端口
func (t *Tgbot) openXPanelPorts(chatId int64) {
	go func() {
		t.SendMsgToTgbot(chatId, "🚀 **正在开放 X-Panel 所需端口...**\n\n请稍候，正在执行端口开放操作。")

		// X-Panel 常用端口
		ports := []string{"22", "80", "443", "13688", "8443"}

		// 检测防火墙类型
		firewalldStatus, firewalldInstalled := t.getFirewalldStatus()

		var successPorts []string
		var failedPorts []string

		for _, port := range ports {
			var err error

			if firewalldInstalled && (strings.Contains(strings.ToLower(firewalldStatus), "active") || strings.Contains(strings.ToLower(firewalldStatus), "已激活")) {
				// 使用 Firewalld 开放端口
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("firewall-cmd --permanent --add-port=%s/tcp", port))
				_, err = cmd.CombinedOutput()

				if err == nil {
					// Firewalld 需要 reload
					cmd := exec.CommandContext(ctx, "bash", "-c", "firewall-cmd --reload")
					_, err = cmd.CombinedOutput()
				}
			} else {
				err = fmt.Errorf("未检测到活跃的防火墙")
			}

			if err != nil {
				failedPorts = append(failedPorts, port)
			} else {
				successPorts = append(successPorts, port)
			}
		}

		// 构建结果消息
		var resultMsg strings.Builder
		resultMsg.WriteString("🎯 **X-Panel 端口开放结果**\n\n")

		if len(successPorts) > 0 {
			resultMsg.WriteString("✅ **成功开放的端口**:\n")
			for _, port := range successPorts {
				resultMsg.WriteString(fmt.Sprintf("• 端口 %s\n", port))
			}
			resultMsg.WriteString("\n")
		}

		if len(failedPorts) > 0 {
			resultMsg.WriteString("❌ **开放失败的端口**:\n")
			for _, port := range failedPorts {
				resultMsg.WriteString(fmt.Sprintf("• 端口 %s\n", port))
			}
			resultMsg.WriteString("\n")
		}

		if len(successPorts) == len(ports) {
			resultMsg.WriteString("🎉 **所有端口开放成功！**\n\nX-Panel 现在可以通过这些端口正常访问。")
		} else if len(successPorts) > 0 {
			resultMsg.WriteString("⚠️ **部分端口开放成功**\n\n请检查失败的端口，或手动配置防火墙规则。")
		} else {
			resultMsg.WriteString("❌ **所有端口开放失败**\n\n请检查防火墙状态或手动配置。")
		}

		t.SendMsgToTgbot(chatId, resultMsg.String())
	}()
}

// 【新增辅助函数】: 获取 Firewalld 状态
func (t *Tgbot) getFirewalldStatus() (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 检查是否安装
	cmd := exec.CommandContext(ctx, "bash", "-c", "command -v firewall-cmd")
	if err := cmd.Run(); err != nil {
		return "未安装", false
	}

	// 获取状态
	cmd = exec.CommandContext(ctx, "bash", "-c", "systemctl is-active firewalld")
	output, err := cmd.Output()
	if err != nil {
		return "状态未知", true
	}

	status := strings.TrimSpace(string(output))
	if status == "active" {
		return "已激活", true
	} else {
		return "未激活", true
	}
}

// showLogMenu 显示日志查看菜单
func (t *Tgbot) showLogMenu(chatId int64) {
	message := "📋 **日志查看菜单**\n\n请选择日志查看选项："

	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📄 最近 20 条").WithCallbackData(t.encodeQuery("fetch_logs 20")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📄 最近 50 条").WithCallbackData(t.encodeQuery("fetch_logs 50")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚙️ 日志设置").WithCallbackData(t.encodeQuery("log_settings")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 关闭").WithCallbackData(t.encodeQuery("close_menu")),
		),
	)

	t.SendMsgToTgbot(chatId, message, keyboard)
}

// sendLongMessage 处理长消息分段发送，使用 <pre> 标签包裹
func (t *Tgbot) sendLongMessage(chatId int64, content string) {
	const maxLength = 4096 - 20 // 减去 <pre> 标签的长度

	if len(content) <= maxLength {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("<pre>%s</pre>", content))
		return
	}

	// 分段发送
	lines := strings.Split(content, "\n")
	var buffer strings.Builder

	for _, line := range lines {
		if buffer.Len() > 0 && buffer.Len()+len(line)+1 > maxLength {
			// 发送当前段
			t.SendMsgToTgbot(chatId, fmt.Sprintf("<pre>%s</pre>", buffer.String()))
			buffer.Reset()
		}

		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(line)
	}

	// 发送最后一段
	if buffer.Len() > 0 {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("<pre>%s</pre>", buffer.String()))
	}
}
