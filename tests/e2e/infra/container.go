package infra

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	imageName     = "x-panel-e2e:latest"
	containerName = "x-panel-e2e-container"
	maxRetries    = 60
	retryInterval = 2 * time.Second
)

// XPanelContainer 封装 X-Panel 测试容器的管理
type XPanelContainer struct {
	container testcontainers.Container
	host      string
	mappedPort string
	baseURL   string
}

// NewXPanelContainer 创建新的 X-Panel 测试容器
func NewXPanelContainer(ctx context.Context) (*XPanelContainer, error) {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       ".",
			Dockerfile:    "tests/e2e/Dockerfile",
			PrintBuildLog: true,
			KeepImage:     false,
		},
		ExposedPorts: []string{"13688/tcp"},
		Env: map[string]string{
			"XPANEL_RUN_IN_CONTAINER": "true",
			"XUI_ENABLE_FAIL2BAN":     "false",
		},
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd: []string{
			"./x-ui setting -username e2e_admin -password e2e_test_pass_123 -port 13688 && " +
				"./x-ui",
		},
		WaitingFor: wait.ForLog("Web server running HTTP").
			WithStartupTimeout(2 * time.Minute),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "13688/tcp")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	return &XPanelContainer{
		container: container,
		host:      host,
		mappedPort: mappedPort.Port(),
		baseURL:   baseURL,
	}, nil
}

// GetBaseURL 获取容器的基础URL
func (c *XPanelContainer) GetBaseURL() string {
	return c.baseURL
}

// WaitForReady 等待容器就绪
func (c *XPanelContainer) WaitForReady(ctx context.Context) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	healthURL := c.baseURL + "/health"

	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryInterval):
		}
	}

	return fmt.Errorf("service did not become ready after %d attempts", maxRetries)
}

// Terminate 终止容器
func (c *XPanelContainer) Terminate(ctx context.Context) error {
	return c.container.Terminate(ctx)
}

// GetContainerInfo 获取容器信息
func (c *XPanelContainer) GetContainerInfo() (host, port, baseURL string) {
	return c.host, c.mappedPort, c.baseURL
}