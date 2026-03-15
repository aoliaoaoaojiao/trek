package uia

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BaseResp 基础响应结构
type BaseResp struct {
	SessionId *string         `json:"SessionId"`
	Status    int             `json:"status"`
	Value     json.RawMessage `json:"value"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Message string `json:"message"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	SessionId    string                 `json:"sessionId"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// WindowSize 窗口大小
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// UiaClient UIA 客户端
type UiaClient struct {
	RemoteUrl  string
	SessionId  string
	httpClient *http.Client
	windowSize *WindowSize
}

// NewUiaClient 创建新的 UiaClient 实例
func NewUiaClient(remoteUrl string) (*UiaClient, error) {
	client := &UiaClient{
		RemoteUrl:  remoteUrl,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}

	err := client.NewSession(map[string]interface{}{
		"platformName":              "Android",
		"appium:automationName":     "UiAutomator2",
		"appium:newCommandTimeout":  1800,
		"appium:noReset":            true,
		"appium:shouldTerminateApp": false,
		"appium:forceAppLaunch":     false,
		"appium:dontStopAppOnReset": true,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

// NewUiaPageWithSession 使用已有会话创建 UiaClient 实例
func NewUiaPageWithSession(remoteUrl, sessionId string) *UiaClient {
	return &UiaClient{
		RemoteUrl:  remoteUrl,
		SessionId:  sessionId,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetHTTPClient 设置自定义 HTTP 客户端
func (u *UiaClient) SetHTTPClient(client *http.Client) {
	u.httpClient = client
}

// CheckSessionId 检查会话 ID 是否存在
func (u *UiaClient) CheckSessionId() error {
	if u.SessionId == "" {
		return fmt.Errorf("SessionId not found")
	}
	return nil
}

// Request 执行 HTTP 请求
func (u *UiaClient) Request(method, url string, body []byte, timeout ...time.Duration) (*BaseResp, error) {
	client := u.httpClient
	if len(timeout) > 0 && timeout[0] > 0 {
		client = &http.Client{Timeout: timeout[0]}
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println("uia response:", string(respBody))

	var baseResp BaseResp
	if err := json.Unmarshal(respBody, &baseResp); err != nil {
		return nil, err
	}

	return &baseResp, nil
}

// NewSession 创建新会话
func (u *UiaClient) NewSession(capabilities map[string]interface{}) error {
	data := map[string]interface{}{
		"capabilities": capabilities,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := u.Request(http.MethodPost, u.RemoteUrl+"/session", body)
	if err != nil {
		return err
	}

	var sessionInfo SessionInfo
	if err := json.Unmarshal(resp.Value, &sessionInfo); err != nil {
		return err
	}

	u.SessionId = sessionInfo.SessionId
	return nil
}

// GetSessionId 获取当前会话 ID
func (u *UiaClient) GetSessionId() string {
	return u.SessionId
}

// SetSessionId 设置会话 ID
func (u *UiaClient) SetSessionId(sessionId string) {
	u.SessionId = sessionId
}

// Screenshot 截图
func (u *UiaClient) Screenshot() ([]byte, error) {
	if err := u.CheckSessionId(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/session/%s/screenshot", u.RemoteUrl, u.SessionId)
	resp, err := u.Request(http.MethodGet, url, nil, 60*time.Second)
	if err != nil {
		return nil, err
	}

	var base64Str string
	if err := json.Unmarshal(resp.Value, &base64Str); err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(base64Str)
}

// SendKeys 发送文本
func (u *UiaClient) SendKeys(text string, isCover bool) error {
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	data := map[string]interface{}{
		"text":    text,
		"replace": isCover,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/keys", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

// SetPasteboard 设置剪贴板内容
func (u *UiaClient) SetPasteboard(contentType, content string) error {
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	data := map[string]interface{}{
		"contentType": contentType,
		"content":     base64.StdEncoding.EncodeToString([]byte(content)),
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/appium/device/set_clipboard", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

// SetAppiumSettings 设置 Appium 设置
func (u *UiaClient) SetAppiumSettings(settings map[string]interface{}) error {
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	data := map[string]interface{}{
		"settings": settings,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s/appium/settings", u.RemoteUrl, u.SessionId)
	_, err = u.Request(http.MethodPost, url, body)
	return err
}

// Close 关闭会话
func (u *UiaClient) Close() error {
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/session/%s", u.RemoteUrl, u.SessionId)
	_, err := u.Request(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	u.SessionId = ""
	return nil
}
