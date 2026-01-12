package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// Client 封装带 Cookie 的 HTTP 客户端
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient 创建新的 API 客户端
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

// Login 执行登录
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

// checkResponse 检查API响应是否成功
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

// AddInbound 添加入站
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

// DelInbound 删除入站
func (c *Client) DelInbound(id int) error {
	apiURL := fmt.Sprintf("%s/panel/api/inbounds/del/%d", c.baseURL, id)
	resp, err := c.http.Post(apiURL, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.checkResponse(resp)
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
func (c *Client) AddInboundClient(client map[string]interface{}) (int, error) {
	apiURL := c.baseURL + "/panel/api/inbounds/addClient"
	jsonData, err := json.Marshal(client)
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

// DownloadBackup 下载数据库备份
func (c *Client) DownloadBackup() ([]byte, error) {
	apiURL := c.baseURL + "/panel/api/server/getDb"
	resp, err := c.http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backup download failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// RestoreBackup 恢复数据库备份
func (c *Client) RestoreBackup(backupData []byte) error {
	apiURL := c.baseURL + "/panel/api/server/importDB"

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("db", "x-ui.db")
	if err != nil {
		return err
	}
	if _, err := fw.Write(backupData); err != nil {
		return err
	}
	w.Close()

	httpReq, err := http.NewRequest("POST", apiURL, &b)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", w.FormDataContentType())

	restoreResp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer restoreResp.Body.Close()

	var result struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	if err := json.NewDecoder(restoreResp.Body).Decode(&result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("database restore failed: %s", result.Msg)
	}
	return nil
}