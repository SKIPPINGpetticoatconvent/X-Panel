package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	imageName     = "x-panel-e2e:latest"
	containerName = "x-panel-e2e-container"
	maxRetries    = 60
	retryInterval = 2 * time.Second
	username      = "admin"
	password      = "admin"
)

// Client 封装带 Cookie 的 HTTP 客户端
type Client struct {
	http    *http.Client
	baseURL string
}

func NewClient(baseURL string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		http: &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
	}, nil
}

func (c *Client) Login(username, password string) error {
	loginURL := c.baseURL + "/login"
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	resp, err := c.http.PostForm(loginURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

func (c *Client) AddInbound(inbound map[string]interface{}) (int, error) {
	apiURL := c.baseURL + "/panel/api/inbounds/add"
	jsonData, err := json.Marshal(inbound)
	if err != nil {
		return 0, err
	}

	resp, err := c.http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}

	if !result.Success {
		return 0, fmt.Errorf("api error: %s", result.Msg)
	}

	if idFloat, ok := result.Obj["id"].(float64); ok {
		return int(idFloat), nil
	}
	return 0, fmt.Errorf("invalid id in response")
}

func (c *Client) DelInbound(id int) error {
	apiURL := fmt.Sprintf("%s/panel/api/inbounds/del/%d", c.baseURL, id)
	resp, err := c.http.Post(apiURL, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

func (c *Client) checkResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("status: %d, parse error: %v, body: %s", resp.StatusCode, err, string(body))
	}
	if !result.Success {
		return fmt.Errorf("api failed: %s", result.Msg)
	}
	return nil
}

// GetInbounds 获取入站列表
func (c *Client) GetInbounds() ([]map[string]interface{}, error) {
	apiURL := c.baseURL + "/panel/api/inbounds/list"
	resp, err := c.http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool                     `json:"success"`
		Msg     string                   `json:"msg"`
		Obj     []map[string]interface{} `json:"obj"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}

	if !result.Success {
		return nil, fmt.Errorf("api error: %s", result.Msg)
	}
	return result.Obj, nil
}

// UpdateInbound 更新入站
func (c *Client) UpdateInbound(id int, inbound map[string]interface{}) error {
	apiURL := fmt.Sprintf("%s/panel/api/inbounds/update/%d", c.baseURL, id)
	jsonData, err := json.Marshal(inbound)
	if err != nil {
		return err
	}

	resp, err := c.http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

// AddInboundClient 添加客户端
func (c *Client) AddInboundClient(inbound map[string]interface{}) (int, error) {
	apiURL := c.baseURL + "/panel/api/inbounds/addClient"
	jsonData, err := json.Marshal(inbound)
	if err != nil {
		return 0, err
	}

	resp, err := c.http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}

	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}

	if !result.Success {
		return 0, fmt.Errorf("api error: %s", result.Msg)
	}

	if idFloat, ok := result.Obj["id"].(float64); ok {
		return int(idFloat), nil
	}
	return 0, fmt.Errorf("invalid id in response")
}

// GetClientTraffics 获取客户端流量
func (c *Client) GetClientTraffics(email string) (map[string]interface{}, error) {
	apiURL := fmt.Sprintf("%s/panel/api/inbounds/getClientTraffics/%s", c.baseURL, email)
	resp, err := c.http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}

	if !result.Success {
		return nil, fmt.Errorf("api error: %s", result.Msg)
	}
	return result.Obj, nil
}

// ResetClientTraffic 重置客户端流量
func (c *Client) ResetClientTraffic(id int, email string) error {
	apiURL := fmt.Sprintf("%s/panel/api/inbounds/%d/resetClientTraffic/%s", c.baseURL, id, email)
	resp, err := c.http.Post(apiURL, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

// GetServerStatus 获取服务器状态
func (c *Client) GetServerStatus() (map[string]interface{}, error) {
	apiURL := c.baseURL + "/panel/api/server/status"
	resp, err := c.http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}
	return result, nil
}

// GetSettings 获取设置
func (c *Client) GetSettings() (map[string]interface{}, error) {
	apiURL := c.baseURL + "/panel/api/setting/all"
	resp, err := c.http.Post(apiURL, "application/json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Success bool                   `json:"success"`
		Msg     string                 `json:"msg"`
		Obj     map[string]interface{} `json:"obj"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}

	if !result.Success {
		return nil, fmt.Errorf("api error: %s", result.Msg)
	}
	return result.Obj, nil
}

// UpdateSettings 更新设置
func (c *Client) UpdateSettings(settings map[string]interface{}) error {
	apiURL := c.baseURL + "/panel/api/setting/update"
	jsonData, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	resp, err := c.http.Post(apiURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

// GetNewSNI 获取新的SNI
func (c *Client) GetNewSNI() (string, error) {
	apiURL := c.baseURL + "/panel/api/server/getNewSNI"
	resp, err := c.http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Sni string `json:"sni"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse error: %v, body: %s", err, string(body))
	}
	return result.Sni, nil
}

// BackupToTgBot 备份到Telegram Bot
func (c *Client) BackupToTgBot() error {
	apiURL := c.baseURL + "/panel/api/backuptotgbot"
	resp, err := c.http.Get(apiURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
}

func TestDockerE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// 使用 Testcontainers 创建容器
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../../",
			Dockerfile:    "tests/e2e/Dockerfile",
			PrintBuildLog: true,
			KeepImage:     false,
		},
		ExposedPorts: []string{"13688/tcp"},
		Env: map[string]string{
			"XPANEL_RUN_IN_CONTAINER": "true",
			"XUI_ENABLE_FAIL2BAN":     "false",
		},
		// 覆盖 Entrypoint 以确保 Cmd 能够执行
		Entrypoint: []string{"/bin/sh", "-c"},
		// 初始化并启动应用
		Cmd: []string{
			"./x-ui setting -username admin -password admin -port 13688 && " +
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
		t.Fatalf("Failed to start container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// 获取映射的端口（Testcontainers 自动处理端口映射）
	mappedPort, err := container.MappedPort(ctx, "13688/tcp")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	t.Logf("Container is running at: %s", baseURL)

	// 执行健康检查
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	// 业务逻辑测试
	client, err := NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 5.1 登录测试
	t.Log("Testing login functionality...")
	if err := client.Login(username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Log("Login successful")

	// 5.2 服务器状态测试
	t.Log("Testing server status...")
	status, err := client.GetServerStatus()
	if err != nil {
		t.Fatalf("Get server status failed: %v", err)
	}
	t.Logf("Server status retrieved: %v", status)

	// 5.3 设置管理测试
	t.Log("Testing settings management...")
	_, err = client.GetSettings()
	if err != nil {
		t.Fatalf("Get settings failed: %v", err)
	}
	t.Logf("Settings retrieved successfully")

	// 更新设置测试（可选，谨慎操作）
	// 注意：这里不实际更新设置以避免影响其他测试

	// 5.4 SNI 功能测试
	t.Log("Testing SNI functionality...")
	sni, err := client.GetNewSNI()
	if err != nil {
		t.Fatalf("Get new SNI failed: %v", err)
	}
	if sni == "" {
		t.Fatalf("SNI should not be empty")
	}
	t.Logf("New SNI retrieved: %s", sni)

	// 5.5 入站管理测试
	t.Log("Testing inbound management...")

	// 获取初始入站列表
	initialInbounds, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Get inbounds failed: %v", err)
	}
	initialCount := len(initialInbounds)
	t.Logf("Initial inbounds count: %d", initialCount)

	// 添加入站
	t.Log("Adding new inbound...")
	vmessSettings := `{"clients": [{"id": "505f1194-a603-46d6-896f-29d93a635831", "alterId": 0}], "disableInsecureEncryption": false}`
	streamSettings := `{"network": "tcp", "security": "none", "tcpSettings": {}}`
	inbound := map[string]interface{}{
		"enable":         true,
		"remark":         "e2e-test-vmess-" + time.Now().Format("150405"),
		"listen":         "",
		"port":           20000 + (time.Now().Unix() % 10000),
		"protocol":       "vmess",
		"up":             0,
		"down":           0,
		"total":          0,
		"settings":       vmessSettings,
		"streamSettings": streamSettings,
		"sniffing":       "{}",
	}

	inboundID, err := client.AddInbound(inbound)
	if err != nil {
		t.Fatalf("Add inbound failed: %v", err)
	}
	t.Logf("Inbound added successfully, ID: %d", inboundID)

	// 验证入站已添加
	inboundsAfterAdd, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Get inbounds after add failed: %v", err)
	}
	if len(inboundsAfterAdd) != initialCount+1 {
		t.Fatalf("Expected %d inbounds, got %d", initialCount+1, len(inboundsAfterAdd))
	}
	t.Logf("Inbound count after add: %d", len(inboundsAfterAdd))

	// 更新入站
	t.Logf("Updating inbound ID: %d...", inboundID)
	inbound["remark"] = "e2e-test-vmess-updated-" + time.Now().Format("150405")
	if err := client.UpdateInbound(inboundID, inbound); err != nil {
		t.Fatalf("Update inbound failed: %v", err)
	}
	t.Log("Inbound updated successfully")

	// 5.6 客户端管理测试
	t.Log("Testing client management...")

	// 添加客户端
	clientData := map[string]interface{}{
		"id": inboundID,
		"settings": `{
			"clients": [
				{
					"id": "505f1194-a603-46d6-896f-29d93a635831",
					"alterId": 0,
					"email": "test-client@example.com"
				}
			]
		}`,
	}

	clientID, err := client.AddInboundClient(clientData)
	if err != nil {
		t.Fatalf("Add inbound client failed: %v", err)
	}
	t.Logf("Client added successfully, ID: %d", clientID)

	// 获取客户端流量
	traffics, err := client.GetClientTraffics("test-client@example.com")
	if err != nil {
		t.Fatalf("Get client traffics failed: %v", err)
	}
	t.Logf("Client traffics retrieved: %v", traffics)

	// 重置客户端流量
	if err := client.ResetClientTraffic(inboundID, "test-client@example.com"); err != nil {
		t.Fatalf("Reset client traffic failed: %v", err)
	}
	t.Log("Client traffic reset successfully")

	// 5.7 备份功能测试
	t.Log("Testing backup functionality...")
	// 注意：备份到Telegram可能需要配置，这里只测试API调用
	if err := client.BackupToTgBot(); err != nil {
		// 备份可能因为未配置Telegram而失败，这是正常的
		t.Logf("Backup to TgBot (expected to fail without config): %v", err)
	} else {
		t.Log("Backup to TgBot successful")
	}

	// 5.8 清理测试数据
	t.Log("Cleaning up test data...")

	// 删除入站
	t.Logf("Deleting inbound ID: %d...", inboundID)
	if err := client.DelInbound(inboundID); err != nil {
		t.Fatalf("Delete inbound failed: %v", err)
	}
	t.Log("Inbound deleted successfully")

	// 验证入站已删除
	finalInbounds, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Get inbounds after delete failed: %v", err)
	}
	if len(finalInbounds) != initialCount {
		t.Fatalf("Expected %d inbounds after cleanup, got %d", initialCount, len(finalInbounds))
	}
	t.Logf("Final inbounds count: %d", len(finalInbounds))

	t.Log("E2E Test Passed Successfully!")
}

// TestDockerE2EPerformance 性能测试
func TestDockerE2EPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// 使用 Testcontainers 创建容器
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../../",
			Dockerfile:    "tests/e2e/Dockerfile",
			PrintBuildLog: true,
			KeepImage:     false,
		},
		ExposedPorts: []string{"13688/tcp"},
		Env: map[string]string{
			"XPANEL_RUN_IN_CONTAINER": "true",
			"XUI_ENABLE_FAIL2BAN":     "false",
		},
		// 覆盖 Entrypoint 并初始化应用
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd: []string{
			"./x-ui setting -username admin -password admin -port 13688 && " +
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
		t.Fatalf("Failed to start container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// 获取映射的端口
	mappedPort, err := container.MappedPort(ctx, "13688/tcp")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	// 执行健康检查
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	// API性能测试
	client, err := NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 登录
	if err := client.Login(username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// 测试多个API调用的性能
	t.Log("Testing API performance...")

	// 测试获取服务器状态的性能
	statusStart := time.Now()
	for i := 0; i < 10; i++ {
		_, err := client.GetServerStatus()
		if err != nil {
			t.Fatalf("Get server status failed on iteration %d: %v", i, err)
		}
	}
	statusTime := time.Since(statusStart)
	avgStatusTime := statusTime / 10
	t.Logf("Average server status response time: %v", avgStatusTime)

	// 测试入站列表性能
	inboundsStart := time.Now()
	for i := 0; i < 10; i++ {
		_, err := client.GetInbounds()
		if err != nil {
			t.Fatalf("Get inbounds failed on iteration %d: %v", i, err)
		}
	}
	inboundsTime := time.Since(inboundsStart)
	avgInboundsTime := inboundsTime / 10
	t.Logf("Average inbounds list response time: %v", avgInboundsTime)

	// 性能断言
	if avgStatusTime > 500*time.Millisecond {
		t.Errorf("Server status response too slow: %v (should be < 500ms)", avgStatusTime)
	}
	if avgInboundsTime > 1*time.Second {
		t.Errorf("Inbounds list response too slow: %v (should be < 1s)", avgInboundsTime)
	}

	t.Log("Performance Test Passed Successfully!")
}

// TestDockerE2EErrorHandling 错误处理测试
func TestDockerE2EErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// 使用 Testcontainers 创建容器
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../../",
			Dockerfile:    "tests/e2e/Dockerfile",
			PrintBuildLog: true,
			KeepImage:     false,
		},
		ExposedPorts: []string{"13688/tcp"},
		Env: map[string]string{
			"XPANEL_RUN_IN_CONTAINER": "true",
			"XUI_ENABLE_FAIL2BAN":     "false",
		},
		// 覆盖 Entrypoint 并初始化应用
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd: []string{
			"./x-ui setting -username admin -password admin -port 13688 && " +
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
		t.Fatalf("Failed to start container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// 获取映射的端口
	mappedPort, err := container.MappedPort(ctx, "13688/tcp")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	// 执行健康检查
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	// 错误处理测试
	client, err := NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Log("Testing error handling...")

	// 测试无效登录
	t.Log("Testing invalid login...")
	invalidClient, _ := NewClient(baseURL)
	if err := invalidClient.Login("invalid", "invalid"); err == nil {
		t.Error("Expected login to fail with invalid credentials")
	} else {
		t.Logf("Invalid login correctly failed: %v", err)
	}

	// 先登录获取有效session
	if err := client.Login(username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// 测试删除不存在的入站
	t.Log("Testing delete non-existent inbound...")
	if err := client.DelInbound(99999); err == nil {
		t.Error("Expected delete non-existent inbound to fail")
	} else {
		t.Logf("Delete non-existent inbound correctly failed: %v", err)
	}

	// 测试获取不存在的客户端流量
	t.Log("Testing get traffic for non-existent client...")
	_, err = client.GetClientTraffics("nonexistent@example.com")
	if err == nil {
		t.Error("Expected get traffic for non-existent client to fail")
	} else {
		t.Logf("Get traffic for non-existent client correctly failed: %v", err)
	}

	// 测试无效的入站数据
	t.Log("Testing invalid inbound data...")
	invalidInbound := map[string]interface{}{
		"enable":   true,
		"remark":   "test",
		"port":     -1, // 无效端口
		"protocol": "invalid_protocol",
	}
	_, err = client.AddInbound(invalidInbound)
	if err == nil {
		t.Error("Expected add inbound with invalid data to fail")
	} else {
		t.Logf("Add inbound with invalid data correctly failed: %v", err)
	}

	t.Log("Error Handling Test Passed Successfully!")
}

// TestDockerE2EBackupRestore 备份恢复E2E测试
func TestDockerE2EBackupRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// 使用 Testcontainers 创建容器
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../../",
			Dockerfile:    "tests/e2e/Dockerfile",
			PrintBuildLog: true,
			KeepImage:     false,
		},
		ExposedPorts: []string{"13688/tcp"},
		Env: map[string]string{
			"XPANEL_RUN_IN_CONTAINER": "true",
			"XUI_ENABLE_FAIL2BAN":     "false",
		},
		// 覆盖 Entrypoint 并初始化应用
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd: []string{
			"./x-ui setting -username admin -password admin -port 13688 && " +
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
		t.Fatalf("Failed to start container: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// 获取映射的端口
	mappedPort, err := container.MappedPort(ctx, "13688/tcp")
	if err != nil {
		t.Fatalf("Failed to get mapped port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get host: %v", err)
	}

	baseURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	// 执行健康检查
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		t.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	healthURL := baseURL + "/health"

	// 备份恢复测试
	client, err := NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 登录
	if err := client.Login(username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	t.Log("Testing database backup and restore...")

	// 5.1 创建测试数据
	t.Log("Creating test data...")

	// 添加测试入站
	testInbound := map[string]interface{}{
		"enable":         true,
		"remark":         "backup-test-inbound",
		"listen":         "",
		"port":           30000,
		"protocol":       "vmess",
		"up":             0,
		"down":           0,
		"total":          0,
		"settings":       `{"clients": [{"id": "test-id-123", "alterId": 0}], "disableInsecureEncryption": false}`,
		"streamSettings": `{"network": "tcp", "security": "none", "tcpSettings": {}}`,
		"sniffing":       "{}",
	}

	inboundID, err := client.AddInbound(testInbound)
	if err != nil {
		t.Fatalf("Failed to create test inbound: %v", err)
	}
	t.Logf("Test inbound created with ID: %d", inboundID)

	// 验证入站创建成功
	inbounds, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Failed to get inbounds: %v", err)
	}

	found := false
	for _, inbound := range inbounds {
		if id, ok := inbound["id"].(float64); ok && int(id) == inboundID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Test inbound not found in inbounds list")
	}

	// 5.2 执行数据库备份
	t.Log("Performing database backup...")
	backupURL := baseURL + "/panel/api/server/getDb"
	backupResp, err := client.http.Get(backupURL)
	if err != nil {
		t.Fatalf("Failed to download database backup: %v", err)
	}
	defer backupResp.Body.Close()

	if backupResp.StatusCode != http.StatusOK {
		t.Fatalf("Backup download failed with status: %d", backupResp.StatusCode)
	}

	// 读取备份数据
	backupData, err := io.ReadAll(backupResp.Body)
	if err != nil {
		t.Fatalf("Failed to read backup data: %v", err)
	}

	if len(backupData) == 0 {
		t.Fatalf("Backup data is empty")
	}
	t.Logf("Database backup successful, size: %d bytes", len(backupData))

	// 5.3 模拟数据丢失（删除测试入站）
	t.Log("Simulating data loss by deleting test inbound...")
	if err := client.DelInbound(inboundID); err != nil {
		t.Fatalf("Failed to delete test inbound: %v", err)
	}

	// 验证入站已删除
	inboundsAfterDelete, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Failed to get inbounds after delete: %v", err)
	}

	deleted := true
	for _, inbound := range inboundsAfterDelete {
		if id, ok := inbound["id"].(float64); ok && int(id) == inboundID {
			deleted = false
			break
		}
	}
	if !deleted {
		t.Fatalf("Test inbound was not deleted")
	}
	t.Log("Test inbound successfully deleted")

	// 5.4 执行数据库恢复
	t.Log("Performing database restore...")
	restoreURL := baseURL + "/panel/api/server/importDB"

	// 创建multipart表单
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("db", "x-ui.db")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := fw.Write(backupData); err != nil {
		t.Fatalf("Failed to write backup data to form: %v", err)
	}
	w.Close()

	httpReq, err := http.NewRequest("POST", restoreURL, &b)
	if err != nil {
		t.Fatalf("Failed to create restore request: %v", err)
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	// 使用已登录的客户端发送请求
	restoreResp, err := client.http.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to send restore request: %v", err)
	}
	defer restoreResp.Body.Close()

	var restoreResult struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(restoreResp.Body).Decode(&restoreResult); err != nil {
		t.Fatalf("Failed to decode restore response: %v", err)
	}

	if !restoreResult.Success {
		t.Fatalf("Database restore failed: %s", restoreResult.Msg)
	}
	t.Log("Database restore successful")

	// 等待服务重启
	t.Log("Waiting for service to restart after restore...")
	time.Sleep(3 * time.Second) // 等待重启

	// 重新等待服务就绪
	if err := waitForService(healthURL); err != nil {
		t.Fatalf("Service failed to restart after restore: %v", err)
	}

	// 重新登录
	client, err = NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to recreate client: %v", err)
	}
	if err := client.Login(username, password); err != nil {
		t.Fatalf("Login failed after restore: %v", err)
	}

	// 5.5 验证数据恢复
	t.Log("Verifying data restoration...")
	inboundsAfterRestore, err := client.GetInbounds()
	if err != nil {
		t.Fatalf("Failed to get inbounds after restore: %v", err)
	}

	restored := false
	for _, inbound := range inboundsAfterRestore {
		if id, ok := inbound["id"].(float64); ok && int(id) == inboundID {
			if remark, ok := inbound["remark"].(string); ok && remark == "backup-test-inbound" {
				restored = true
				break
			}
		}
	}

	if !restored {
		t.Fatalf("Test inbound was not restored after database restore")
	}
	t.Log("Data restoration verified successfully")

	// 清理测试数据
	t.Log("Cleaning up test data...")
	if err := client.DelInbound(inboundID); err != nil {
		t.Logf("Warning: Failed to clean up test inbound: %v", err)
	}

	t.Log("Backup and Restore E2E Test Passed Successfully!")
}

func runCommand(t *testing.T, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		if name == "docker" && len(args) > 0 && args[0] == "rm" {
			return output
		}
		t.Fatalf("Command failed: %s %s\nOutput: %s\nError: %v", name, strings.Join(args, " "), output, err)
	}
	return output
}

func waitForService(url string) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			// 接受200-399范围的状态码（包括重定向）
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
		time.Sleep(retryInterval)
	}
	return fmt.Errorf("service did not become ready after %d attempts", maxRetries)
}
