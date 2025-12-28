package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	imageName     = "x-panel-e2e:latest"
	containerName = "x-panel-e2e-container"
	hostPort      = "13688"
	baseURL       = "http://localhost:" + hostPort
	maxRetries    = 30
	retryInterval = 1 * time.Second
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

func TestPodmanE2E(t *testing.T) {
	// 1. 清理旧环境
	runCommand(t, "podman", "rm", "-f", containerName)

	// 2. 构建镜像
	t.Logf("Building Docker image: %s...", imageName)
	runCommand(t, "podman", "build", "-t", imageName, ".")

	// 3. 启动容器
	t.Logf("Starting container: %s...", containerName)
	runCommand(t, "podman", "run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:13688", hostPort),
		imageName,
	)

	defer func() {
		t.Logf("Cleaning up container: %s...", containerName)
		runCommand(t, "podman", "rm", "-f", containerName)
	}()

	// 4. 健康检查
	t.Logf("Waiting for service to be ready at %s...", baseURL)
	if err := waitForService(baseURL); err != nil {
		logs := runCommand(t, "podman", "logs", containerName)
		t.Logf("Container Logs:\n%s", logs)
		t.Fatalf("Service failed to start: %v", err)
	}
	t.Log("Service is ready!")

	// 5. 业务逻辑测试
	client, err := NewClient(baseURL)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 5.1 登录
	t.Log("Attempting to login...")
	if err := client.Login(username, password); err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Log("Login successful")

	// 5.2 添加入站
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

	id, err := client.AddInbound(inbound)
	if err != nil {
		t.Fatalf("Add inbound failed: %v", err)
	}
	t.Logf("Inbound added successfully, ID: %d", id)

	// 5.3 删除入站
	t.Logf("Deleting inbound ID: %d...", id)
	if err := client.DelInbound(id); err != nil {
		t.Fatalf("Delete inbound failed: %v", err)
	}
	t.Log("Inbound deleted successfully")

	t.Log("E2E Test Passed Successfully!")
}

func runCommand(t *testing.T, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		if name == "podman" && len(args) > 0 && args[0] == "rm" {
			return output
		}
		t.Fatalf("Command failed: %s %s\nOutput: %s\nError: %v", name, strings.Join(args, " "), output, err)
	}
	return output
}

func waitForService(url string) error {
	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(retryInterval)
	}
	return fmt.Errorf("service did not become ready after %d attempts", maxRetries)
}