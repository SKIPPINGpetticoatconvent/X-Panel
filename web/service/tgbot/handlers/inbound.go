package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"x-ui/database/model"
	"x-ui/util/common"
	"x-ui/web/service"
	"x-ui/web/service/tgbot/core"
	"x-ui/xray"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// InboundHandlers 入站管理处理器
type InboundHandlers struct {
	ctx            core.ContextInterface
	inboundService *service.InboundService
	serverService  *service.ServerService
	xrayService    *service.XrayService
}

// NewInboundHandlers 创建入站处理器实例
func NewInboundHandlers(
	ctx core.ContextInterface,
	inboundService *service.InboundService,
	serverService *service.ServerService,
	xrayService *service.XrayService,
) *InboundHandlers {
	return &InboundHandlers{
		ctx:            ctx,
		inboundService: inboundService,
		serverService:  serverService,
		xrayService:    xrayService,
	}
}

// HandleList 处理列出所有入站的请求
func (h *InboundHandlers) HandleList(message telego.Message) error {
	inbounds, err := h.inboundService.GetAllInbounds()
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 获取入站列表失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	if len(inbounds) == 0 {
		msg := "📋 当前没有配置任何入站"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 构建入站列表消息
	msg := fmt.Sprintf("📋 <b>入站列表</b> (共 %d 个)\n\n", len(inbounds))
	
	for i, inbound := range inbounds {
		if i >= 10 { // 限制显示数量避免消息过长
			msg += "\n... (更多入站请查看面板)"
			break
		}
		
		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		
		protocol := strings.ToUpper(string(inbound.Protocol))
		
		msg += fmt.Sprintf(`%s <b>%s</b>
   🔌 端口: %d
   📡 协议: %s
   📊 流量: ↑%s ↓%s
   ⏰ 到期: %s
   
`,
			status,
			inbound.Remark,
			inbound.Port,
			protocol,
			common.FormatTraffic(inbound.Up),
			common.FormatTraffic(inbound.Down),
			h.formatExpiryTime(inbound.ExpiryTime),
		)
	}

	// 添加操作提示
	msg += "\n💡 提示：使用 /inbound <端口号> 查看详细信息"

	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// HandleInfo 处理查看特定入站信息的请求
func (h *InboundHandlers) HandleInfo(message telego.Message, args []string) error {
	if len(args) == 0 {
		msg := "❌ 请指定要查看的入站端口号\n\n用法: /inbound <端口号>"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	port, err := strconv.Atoi(args[0])
	if err != nil {
		msg := "❌ 端口号格式错误，请输入数字"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 查找指定端口的入站
	inbounds, err := h.inboundService.GetAllInbounds()
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 获取入站信息失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	var targetInbound *model.Inbound
	for _, inbound := range inbounds {
		if inbound.Port == port {
			targetInbound = inbound
			break
		}
	}

	if targetInbound == nil {
		msg := fmt.Sprintf("❌ 未找到端口为 %d 的入站", port)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 构建入站详细信息
	status := "❌ 禁用"
	if targetInbound.Enable {
		status = "✅ 启用"
	}

	protocol := strings.ToUpper(string(targetInbound.Protocol))
	
	msg := fmt.Sprintf(`📊 <b>入站详细信息</b>

🏷️  名称: %s
🔌 端口: %d
📡 协议: %s
⚡ 状态: %s

📈 <b>流量统计</b>
   📤 上传: %s
   📥 下载: %s
   💯 总计: %s

⏰ <b>时间信息</b>
   🕒 创建时间: 未知
   ⏳ 到期时间: %s

🔧 <b>技术信息</b>
   🏷️  Tag: %s
   👤 用户ID: %s`,
		targetInbound.Remark,
		targetInbound.Port,
		protocol,
		status,
		common.FormatTraffic(targetInbound.Up),
		common.FormatTraffic(targetInbound.Down),
		common.FormatTraffic(targetInbound.Up+targetInbound.Down),
	// h.formatTimestamp(targetInbound.CreatedAt), // Inbound模型没有CreatedAt字段
		h.formatExpiryTime(targetInbound.ExpiryTime),
		targetInbound.Tag,
		strconv.Itoa(int(targetInbound.UserId)),
	)

	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// HandleClients 处理查看入站客户端列表的请求
func (h *InboundHandlers) HandleClients(message telego.Message, args []string) error {
	if len(args) == 0 {
		msg := "❌ 请指定要查看客户端的入站端口号\n\n用法: /clients <端口号>"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	port, err := strconv.Atoi(args[0])
	if err != nil {
		msg := "❌ 端口号格式错误，请输入数字"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 查找指定端口的入站
	inbounds, err := h.inboundService.GetAllInbounds()
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 获取入站信息失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	var targetInbound *model.Inbound
	for _, inbound := range inbounds {
		if inbound.Port == port {
			targetInbound = inbound
			break
		}
	}

	if targetInbound == nil {
		msg := fmt.Sprintf("❌ 未找到端口为 %d 的入站", port)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 获取客户端列表
	clients, err := h.inboundService.GetClients(targetInbound)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 获取客户端列表失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	if len(clients) == 0 {
		msg := fmt.Sprintf("📋 端口 %d 的入站没有客户端", port)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 构建客户端列表消息
	msg := fmt.Sprintf("👥 <b>客户端列表</b> - 端口 %d (共 %d 个)\n\n", port, len(clients))
	
	for i, client := range clients {
		if i >= 15 { // 限制显示数量
			msg += "\n... (更多客户端请查看面板)"
			break
		}
		
		status := "❌"
		if client.Enable {
			status = "✅"
		}
		
		// 获取客户端流量信息
		traffic, err := h.inboundService.GetClientTrafficByEmail(client.Email)
		if err != nil {
			traffic = nil // 忽略错误，显示基本信息
		}
		
		up := int64(0)
		down := int64(0)
		if traffic != nil {
			up = traffic.Up
			down = traffic.Down
		}
		
		clientInfo := fmt.Sprintf(`%s <b>%s</b>
   📧 Email: %s
   📊 流量: ↑%s ↓%s
   ⚡ 状态: %s
`,
			status,
			client.Email,
			client.Email,
			common.FormatTraffic(up),
			common.FormatTraffic(down),
			h.getClientStatus(&client, traffic),
		)
		
		msg += clientInfo + "\n"
	}

	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// HandleToggle 处理启用/禁用入站的请求
func (h *InboundHandlers) HandleToggle(message telego.Message, args []string) error {
	if len(args) == 0 {
		msg := "❌ 请指定要操作的入站端口号\n\n用法: /toggle <端口号>"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	port, err := strconv.Atoi(args[0])
	if err != nil {
		msg := "❌ 端口号格式错误，请输入数字"
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 查找指定端口的入站
	inbounds, err := h.inboundService.GetAllInbounds()
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 获取入站信息失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	var targetInbound *model.Inbound
	for _, inbound := range inbounds {
		if inbound.Port == port {
			targetInbound = inbound
			break
		}
	}

	if targetInbound == nil {
		msg := fmt.Sprintf("❌ 未找到端口为 %d 的入站", port)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
	}

	// 切换启用状态
	newStatus := !targetInbound.Enable
	targetInbound.Enable = newStatus

	// 保存更改
	_, _, err = h.inboundService.UpdateInbound(targetInbound)
	if err != nil {
		errorMsg := fmt.Sprintf("❌ 更新入站状态失败: %v", err)
		return h.ctx.SendMsgToTgbot(message.Chat.ID, errorMsg)
	}

	// 如果启用，需要重启 Xray
	if newStatus {
		h.xrayService.SetToNeedRestart()
	}



	action := "禁用"
	if newStatus {
		action = "启用"
	}

	msg := fmt.Sprintf("✅ 已%s入站: %s (端口 %d)", action, targetInbound.Remark, port)
	
	if newStatus {
		msg += "\n\n🔄 Xray 服务需要重启以应用更改"
	}

	return h.ctx.SendMsgToTgbot(message.Chat.ID, msg)
}

// formatExpiryTime 格式化到期时间
func (h *InboundHandlers) formatExpiryTime(expiryTime int64) string {
	if expiryTime == 0 {
		return "永不过期"
	}
	
	// 假设传入的是毫秒时间戳
	if expiryTime > 1e12 {
		expiryTime = expiryTime / 1000
	}
	
	return fmt.Sprintf("%s", time.Unix(expiryTime, 0).Format("2006-01-02 15:04:05"))
}

// formatTimestamp 格式化时间戳
func (h *InboundHandlers) formatTimestamp(timestamp int64) string {
	if timestamp == 0 {
		return "未知"
	}
	
	return fmt.Sprintf("%s", time.Unix(timestamp, 0).Format("2006-01-02 15:04:05"))
}

// getClientStatus 获取客户端状态描述
func (h *InboundHandlers) getClientStatus(client *model.Client, traffic *xray.ClientTraffic) string {
	if !client.Enable {
		return "已禁用"
	}
	
	if traffic == nil {
		return "正常"
	}
	
	now := time.Now().Unix()
	if traffic.ExpiryTime > 0 && traffic.ExpiryTime/1000 <= now {
		return "已过期"
	}
	
	return "正常"
}

// RegisterInboundCommands 注册入站管理命令到路由器
func (h *InboundHandlers) RegisterInboundCommands(router core.ExternalCommandRegistry) {
	// 注册 /inbound 命令
	router.RegisterCommandExt("inbound", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		
		// 解析命令参数
		_, _, args := tu.ParseCommand(message.Text)
		
		if len(args) == 0 {
			return h.HandleList(message)
		}
		
		return h.HandleInfo(message, args)
	})

	// 注册 /clients 命令
	router.RegisterCommandExt("clients", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		
		// 解析命令参数
		_, _, args := tu.ParseCommand(message.Text)
		
		return h.HandleClients(message, args)
	})

	// 注册 /toggle 命令
	router.RegisterCommandExt("toggle", func(ctx core.ContextInterface, message telego.Message, isAdmin bool) error {
		if !isAdmin {
			return ctx.SendMsgToTgbot(message.Chat.ID, "❌ 此命令仅限管理员使用")
		}
		
		// 解析命令参数
		_, _, args := tu.ParseCommand(message.Text)
		
		return h.HandleToggle(message, args)
	})
}