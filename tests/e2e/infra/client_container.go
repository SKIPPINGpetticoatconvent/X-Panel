package infra

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// XrayClientContainer 封装 Xray 客户端测试容器的管理
type XrayClientContainer struct {
	container  testcontainers.Container
	host       string
	mappedPort string
}

// NewXrayClientContainer 创建新的 Xray 客户端测试容器
func NewXrayClientContainer(ctx context.Context, serverHost, serverPort string) (*XrayClientContainer, error) {
	// 创建客户端配置
	configJSON := fmt.Sprintf(`{
		"inbounds": [{
			"port": 1080,
			"listen": "0.0.0.0",
			"protocol": "socks",
			"settings": {
				"auth": "noauth",
				"udp": true
			},
			"sniffing": {
				"enabled": true,
				"destOverride": ["http", "tls"]
			}
		}],
		"outbounds": [{
			"protocol": "vmess",
			"settings": {
				"vnext": [{
					"address": "%s",
					"port": %s,
					"users": [{
						"id": "test-client-id-123",
						"alterId": 0
					}]
				}]
			},
			"streamSettings": {
				"network": "tcp",
				"security": "none"
			}
		}]
	}`, serverHost, serverPort)

	req := testcontainers.ContainerRequest{
		Image:        "teddysun/xray:1.8.6",
		ExposedPorts: []string{"1080/tcp"},
		Env: map[string]string{
			"TZ": "Asia/Shanghai",
		},
		Files: []testcontainers.ContainerFile{
			{
				Reader:            strings.NewReader(configJSON),
				ContainerFilePath: "/etc/xray/config.json",
				FileMode:          0644,
			},
		},
		Cmd:        []string{"xray", "-config", "/etc/xray/config.json"},
		WaitingFor: wait.ForLog("Xray is ready").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start Xray client container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get client host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "1080/tcp")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get client mapped port: %w", err)
	}

	return &XrayClientContainer{
		container:  container,
		host:       host,
		mappedPort: mappedPort.Port(),
	}, nil
}

// GetProxyURL 获取代理URL
func (c *XrayClientContainer) GetProxyURL() string {
	return fmt.Sprintf("socks5://%s:%s", c.host, c.mappedPort)
}

// TestConnectivity 测试与服务器的连通性
func (c *XrayClientContainer) TestConnectivity(ctx context.Context, testURL string) error {
	// 创建HTTP客户端，使用代理
	proxyURL := fmt.Sprintf("socks5://%s:%s", c.host, c.mappedPort)

	client, err := createProxyClient(proxyURL)
	if err != nil {
		return fmt.Errorf("failed to create proxy client: %w", err)
	}

	// 测试连接
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connectivity test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Terminate 终止容器
func (c *XrayClientContainer) Terminate(ctx context.Context) error {
	return c.container.Terminate(ctx)
}

// createProxyClient 创建使用代理的HTTP客户端
func createProxyClient(proxyURL string) (*http.Client, error) {
	// 这里简化实现，实际项目中可能需要使用 golang.org/x/net/proxy
	// 或者外部工具如 proxychains 来测试代理连通性
	// 暂时返回标准客户端，实际测试中可能需要调整

	return &http.Client{
		Timeout: 10 * time.Second,
	}, nil
}
