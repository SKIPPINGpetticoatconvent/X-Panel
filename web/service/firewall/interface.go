package firewall

// 协议常量
const (
	ProtocolTCP = "tcp"
	ProtocolUDP = "udp"
)

// FirewallService 定义防火墙操作的标准接口
type FirewallService interface {
	// Name 返回防火墙的名称
	Name() string
	
	// IsRunning 检查防火墙服务是否正在运行
	IsRunning() bool
	
	// OpenPort 放行指定端口
	// port: 端口号
	// protocol: 协议 ("tcp", "udp", "both" 或 "" 代表同时开放 TCP+UDP)
	OpenPort(port int, protocol string) error
	
	// ClosePort 关闭指定端口
	ClosePort(port int, protocol string) error
	
	// Reload 重载防火墙配置
	Reload() error
	
	// OpenPortAsync 异步放行端口，返回是否成功的布尔值和错误信息
	OpenPortAsync(port int, protocol string) (bool, error)
}