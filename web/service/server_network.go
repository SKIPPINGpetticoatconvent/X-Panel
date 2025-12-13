package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"x-ui/logger"
)

// 网络端口管理模块
// 负责端口开放、关闭、可用性检查、UFW防火墙管理等核心功能

// OpenPort 供前端调用，自动检查/安装 ufw 并放行指定的端口。
// 整个函数逻辑被放入一个 go func() 协程中，实现异步后台执行。
// 函数签名不再返回 error，因为它会立即返回，无法得知后台任务的最终结果。
func (s *ServerService) OpenPortAsync(port string) {
	// 启动一个新的协程来处理耗时任务，这样 HTTP 请求可以立刻返回。
	go func() {
		// 1. 将 port string 转换为 int
		portInt, err := strconv.Atoi(port)
		if err != nil {
			// 在后台任务中，如果出错，我们只能记录日志，因为无法再返回给前端。
			logger.Errorf("端口号格式错误，无法转换为数字: %s", port)
			return
		}

		// 2. 将 Shell 逻辑整合为一个可执行的命令，并使用 /bin/bash -c 执行
		// 此处同样增加了默认端口的定义和放行逻辑。
		shellCommand := fmt.Sprintf(`
	PORT_TO_OPEN=%d
	# 定义一个包含所有必须默认放行的端口的列表。
	DEFAULT_PORTS="22 80 443 13688 8443"
	
	echo "正在为入站配置自动检查并放行端口..."

	# 1. 检查/安装 ufw (仅限 Debian/Ubuntu 系统)
	if ! command -v ufw &>/dev/null; then
		echo "ufw 防火墙未安装，正在安装..."
		# 使用绝对路径执行 apt-get，避免 PATH 问题
		DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get update -qq >/dev/null
		DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get install -y -qq ufw >/dev/null
		if [ $? -ne 0 ]; then echo "❌ ufw 安装失败，可能不是 Debian/Ubuntu 系统，或者权限不足。"; exit 1; fi
	fi

	# 2. 新增步骤，循环检查并放行所有默认端口。
	echo "正在检查并放行基础服务端口: $DEFAULT_PORTS"
	for p in $DEFAULT_PORTS; do
		if ! ufw status | grep -qw "$p/tcp"; then
			echo "端口 $p/tcp 未放行，正在添加规则..."
			ufw allow $p/tcp >/dev/null
			if [ $? -ne 0 ]; then echo "❌ ufw 端口 $p 放行失败。"; exit 1; fi
		else
			echo "端口 $p/tcp 规则已存在，跳过。"
		fi
	done
	echo "✅ 基础服务端口检查完毕。"

	# 3. 放行前端指定的端口 (TCP/UDP)
	echo "正在检查【入站配置】并放行指定端口 $PORT_TO_OPEN..."
	if ! ufw status | grep -qw "$PORT_TO_OPEN"; then
		echo "正在执行 ufw allow $PORT_TO_OPEN..."
		ufw allow $PORT_TO_OPEN >/dev/null
		if [ $? -ne 0 ]; then echo "❌ ufw 端口 $PORT_TO_OPEN 放行失败。"; exit 1; fi
	else
		echo "端口 $PORT_TO_OPEN 规则已存在，跳过。"
	fi

	# 4. 检查/激活防火墙
	if ! ufw status | grep -q "Status: active"; then
		echo "ufw 状态：未激活。正在尝试激活..."
		ufw --force enable
		if [ $? -ne 0 ]; then echo "❌ ufw 激活失败。"; exit 1; fi
	fi
	echo "✅ 端口 $PORT_TO_OPEN 及所有基础端口已成功放行/检查。"
	`, portInt) // 使用转换后的 portInt

		// 3. 使用 exec.CommandContext 运行命令
		// 添加 70 秒超时，防止命令挂起导致 HTTP 连接断开
		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer cancel() // 确保 context 在函数退出时被取消

		cmd := exec.CommandContext(ctx, "/bin/bash", "-c", shellCommand)

		// 4. 捕获命令的输出
		output, err := cmd.CombinedOutput()

		// 5. 记录日志，以便诊断
		logOutput := strings.TrimSpace(string(output))
		logger.Infof("执行 ufw 端口放行命令（端口 %s）结果：\n%s", port, logOutput)

		// 这里的错误处理现在只用于在后台记录日志。
		if err != nil {
			errorMsg := fmt.Sprintf("后台执行端口 %s 自动放行失败。错误: %v", port, err)
			logger.Error(errorMsg)
			// 可选: 未来可以在这里加入 Telegram 机器人通知等功能，来通知管理员任务失败。
		}
	}()
}

// InstallSubconverter 安装订阅转换服务
func (s *ServerService) InstallSubconverterAsync() error {
	// 使用一个新的 goroutine 来执行耗时的安装任务，这样 API 可以立即返回
	go func() {

		// 执行端口放行操作
		var ufwWarning string
		if ufwErr := s.openSubconverterPortsAsync(); ufwErr != nil {
			// 不中断流程，只生成警告消息
			logger.Warningf("自动放行 Subconverter 端口失败: %v", ufwErr)
			ufwWarning = fmt.Sprintf("⚠️ **警告：订阅转换端口放行失败**\n\n自动执行 UFW 命令失败，请务必**手动**在您的 VPS 上放行端口 `8000` 和 `15268`，否则服务将无法访问。失败详情：%v\n\n", ufwErr)
		}

		// 检查全局的 TgBot 实例是否存在并且正在运行
		if s.tgService == nil || !s.tgService.IsRunning() {
			logger.Warning("TgBot 未运行，无法发送【订阅转换】状态通知。")
			// 即使机器人未运行，安装流程也应继续，只是不发通知
			ufwWarning = "" // 如果机器人不在线，不发送任何警告/消息
		}

		// 脚本路径为 /usr/bin/x-ui
		// 通常，安装脚本会将主命令软链接或复制到 /usr/bin/ 目录下，使其成为一个系统命令。
		// 直接调用这个命令比调用源文件路径更规范，也能确保执行的是用户在命令行中使用的同一个脚本。
		scriptPath := "/usr/bin/x-ui"

		// 检查脚本文件是否存在
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			errMsg := fmt.Sprintf("订阅转换安装失败：关键脚本文件 `%s` 未找到。", scriptPath)
			logger.Error(errMsg)
			if s.tgService != nil && s.tgService.IsRunning() {
				// 使用 Markdown 格式发送错误消息
				s.tgService.SendMessage("❌ " + errMsg)
			}
			return
		}

		// 正确的调用方式是：命令是 "x-ui"，参数是 "subconverter"。
		cmd := exec.Command(scriptPath, "subconverter")

		// 执行命令并获取其合并的输出（标准输出 + 标准错误），方便排查问题。
		// 重要: 这个命令可能需要几分钟才能执行完毕，Go程序会在此等待直到脚本执行完成。
		output, err := cmd.CombinedOutput()

		if err != nil {
			if s.tgService != nil && s.tgService.IsRunning() {
				// 构造失败消息
				message := fmt.Sprintf("❌ **订阅转换安装失败**！\n\n**错误信息**: %v\n**输出**: %s", err, string(output))
				s.tgService.SendMessage(message)
			}
			logger.Errorf("订阅转换安装失败: %v\n输出: %s", err, string(output))
			return
		} else {

			// 如果之前端口放行失败，先发送警告消息
			if ufwWarning != "" {
				s.tgService.SendMessage(ufwWarning)
			}

			// 安装成功后，发送通知到 TG 机器人
			if s.tgService != nil && s.tgService.IsRunning() {
				// 获取面板域名，注意：t.getDomain() 是 Tgbot 的方法
				domain, getDomainErr := s.tgService.GetDomain()
				if getDomainErr != nil {
					logger.Errorf("TG Bot: 订阅转换安装成功，但获取域名失败: %v", getDomainErr)
				} else {
					// 构造消息，使用用户指定的格式
					message := fmt.Sprintf(
						"🎉 **恭喜！【订阅转换】模块已成功安装！**\n\n"+
							"您现在可以使用以下地址访问 Web 界面：\n\n"+
							"🔗 **登录地址**: `https://%s:15268`\n\n"+
							"默认用户名: `admin`\n"+
							"默认 密码: `123456`\n\n"+
							"可登录订阅转换后台修改您的密码！", domain)

					// 发送成功消息
					if sendErr := s.tgService.SendMessage(message); sendErr != nil {
						logger.Errorf("TG Bot: 订阅转换安装成功，但发送通知失败: %v", sendErr)
					} else {
						logger.Info("TG Bot: 订阅转换安装成功通知已发送。")
					}
				}
			}

			logger.Info("订阅转换安装成功。")
			return
		}
	}()

	return nil // 立即返回，表示指令已接收
}

// openSubconverterPorts 检查/安装 ufw 并放行 8000 和 15268 端口
func (s *ServerService) openSubconverterPortsAsync() error {
	// Shell 脚本更新，增加了默认端口列表和相应的放行逻辑。
	shellCommand := `
	PORTS_TO_OPEN="8000 15268"
	# 定义一个包含所有必须默认放行的端口的列表。
	DEFAULT_PORTS="22 80 443 13688 8443"
	
	echo "脚本启动：正在为订阅转换服务配置防火墙..."

	# 1. 检查/安装 ufw
	if ! command -v ufw &>/dev/null; then
		echo "ufw 防火墙未安装，正在安装..."
		# 静默更新和安装
		DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get update -qq >/dev/null
		DEBIAN_FRONTEND=noninteractive /usr/bin/apt-get install -y -qq ufw >/dev/null
		if [ $? -ne 0 ]; then echo "❌ ufw 安装失败或权限不足。"; exit 1; fi
	fi

	# 2. 新增步骤，循环检查并放行所有默认端口。
	echo "正在检查并放行基础服务端口: $DEFAULT_PORTS"
	for p in $DEFAULT_PORTS; do
		# 检查规则是否已存在，不存在时才添加，避免重复
		if ! ufw status | grep -qw "$p/tcp"; then
			echo "端口 $p/tcp 未放行，正在添加规则..."
			ufw allow $p/tcp >/dev/null
			if [ $? -ne 0 ]; then echo "❌ ufw 端口 $p 放行失败。"; exit 1; fi
		else
			echo "端口 $p/tcp 规则已存在，跳过。"
		fi
	done
	echo "✅ 基础服务端口检查完毕。"


	# 3. 放行 Subconverter 自身需要的端口
	echo "正在检查并放行订阅转换服务端口: $PORTS_TO_OPEN"
	for port in $PORTS_TO_OPEN; do
		if ! ufw status | grep -qw "$port"; then
			echo "正在执行 ufw allow $port..."
			ufw allow $port >/dev/null
			if [ $? -ne 0 ]; then echo "❌ ufw 端口 $port 放行失败。"; exit 1; fi
		else
			echo "端口 $port 规则已存在，跳过。"
		fi
	done

	# 4. 检查/激活防火墙
	if ! ufw status | grep -q "Status: active"; then
		echo "ufw 状态：未激活。正在尝试激活..."
		ufw --force enable
		if [ $? -ne 0 ]; then echo "❌ ufw 激活失败。"; exit 1; fi
	fi
    
    echo "✅ 所有端口 ($DEFAULT_PORTS $PORTS_TO_OPEN) 已成功放行/检查。"
    exit 0
	`

	// 使用 /bin/bash -c 执行命令，并捕获输出
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "-c", shellCommand)
	output, err := cmd.CombinedOutput()
	logOutput := string(output)

	// 记录日志，无论成功与否
	logger.Infof("执行 Subconverter 端口放行命令结果:\n%s", logOutput)

	if err != nil {
		// 如果 Shell 命令返回非零退出码，则返回错误
		return fmt.Errorf("ufw 端口放行失败: %v. 脚本输出: %s", err, logOutput)
	}

	return nil
}