package uia

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"trek/logger"
)

// BaseResp 表示 UIA 服务返回的基础响应结构。
type BaseResp struct {
	SessionId *string         `json:"sessionId"`
	Status    int             `json:"status"`
	Value     json.RawMessage `json:"value"`
}

// ErrorDetail 表示 UIA 服务返回的错误详情。
type ErrorDetail struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Stacktrace string `json:"stacktrace"`
}

// SessionInfo 表示会话创建成功后的返回信息。
type SessionInfo struct {
	SessionId    string                 `json:"sessionId"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// WindowSize 表示窗口大小。
type WindowSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// UiaClient 是 UIA HTTP 客户端。
type UiaClient struct {
	RemoteUrl  string
	SessionId  string
	httpClient *http.Client
	windowSize *WindowSize
}

// NewUiaClient 创建新的 UIA 客户端，并自动协商可用的 base path。
func NewUiaClient(remoteUrl string) (*UiaClient, error) {
	client := &UiaClient{
		RemoteUrl:  strings.TrimRight(remoteUrl, "/"),
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

// NewUiaPageWithSession 使用已有会话创建 UIA 客户端。
func NewUiaPageWithSession(remoteUrl, sessionId string) *UiaClient {
	return &UiaClient{
		RemoteUrl:  strings.TrimRight(remoteUrl, "/"),
		SessionId:  sessionId,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetHTTPClient 设置自定义 HTTP 客户端。
func (u *UiaClient) SetHTTPClient(client *http.Client) {
	u.httpClient = client
}

// CheckSessionId 检查当前会话是否存在。
func (u *UiaClient) CheckSessionId() error {
	if u.SessionId == "" {
		return fmt.Errorf("sessionId not found")
	}
	return nil
}

// SessionURL 返回当前会话下的完整接口地址。
func (u *UiaClient) SessionURL(path string) string {
	return fmt.Sprintf("%s/session/%s%s", u.RemoteUrl, u.SessionId, path)
}

// Request 执行 HTTP 请求。
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

	const maxUIABodySize = 10 * 1024 * 1024 // 10MB
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxUIABodySize))
	if err != nil {
		return nil, err
	}

	logger.Debugf("uia response: %s", string(respBody))

	var baseResp BaseResp
	if err := json.Unmarshal(respBody, &baseResp); err != nil {
		return nil, err
	}

	var errorDetail ErrorDetail
	if len(baseResp.Value) > 0 && json.Unmarshal(baseResp.Value, &errorDetail) == nil && errorDetail.Error != "" {
		if errorDetail.Message != "" {
			return nil, fmt.Errorf("uia request failed: %s: %s", errorDetail.Error, errorDetail.Message)
		}
		return nil, fmt.Errorf("uia request failed: %s", errorDetail.Error)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("uia request failed: http %d", resp.StatusCode)
	}

	return &baseResp, nil
}

// NewSession 创建新的会话。
func (u *UiaClient) NewSession(capabilities map[string]interface{}) error {
	data := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"alwaysMatch": capabilities,
			"firstMatch":  []map[string]interface{}{{}},
		},
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	candidates := []string{
		u.RemoteUrl,
		u.RemoteUrl + "/wd/hub",
	}

	var lastErr error
	for _, baseURL := range candidates {
		resp, reqErr := u.Request(http.MethodPost, baseURL+"/session", body)
		if reqErr != nil {
			lastErr = reqErr
			continue
		}

		var sessionInfo SessionInfo
		if err := json.Unmarshal(resp.Value, &sessionInfo); err != nil {
			lastErr = err
			continue
		}

		if sessionInfo.SessionId == "" && resp.SessionId != nil {
			sessionInfo.SessionId = *resp.SessionId
		}
		if sessionInfo.SessionId == "" {
			lastErr = fmt.Errorf("uia new session failed: empty session id")
			continue
		}

		u.RemoteUrl = baseURL
		u.SessionId = sessionInfo.SessionId
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("uia new session failed")
	}
	return lastErr
}

// GetSessionId 获取当前会话 ID。
func (u *UiaClient) GetSessionId() string {
	return u.SessionId
}

// SetSessionId 设置会话 ID。
func (u *UiaClient) SetSessionId(sessionId string) {
	u.SessionId = sessionId
}

// Screenshot 截图。
func (u *UiaClient) Screenshot() ([]byte, error) {
	if err := u.CheckSessionId(); err != nil {
		return nil, err
	}

	resp, err := u.Request(http.MethodGet, u.SessionURL("/screenshot"), nil, 60*time.Second)
	if err != nil {
		return nil, err
	}

	var base64Str string
	if err := json.Unmarshal(resp.Value, &base64Str); err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(base64Str)
}

// SendKeys 发送文本。
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

	_, err = u.Request(http.MethodPost, u.SessionURL("/keys"), body)
	return err
}

// SetPasteboard 设置剪贴板内容。
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

	_, err = u.Request(http.MethodPost, u.SessionURL("/appium/device/set_clipboard"), body)
	return err
}

// SetAppiumSettings 设置 Appium 参数。
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

	_, err = u.Request(http.MethodPost, u.SessionURL("/appium/settings"), body)
	return err
}

// Close 关闭当前会话。
func (u *UiaClient) Close() error {
	if err := u.CheckSessionId(); err != nil {
		return err
	}

	_, err := u.Request(http.MethodDelete, u.SessionURL(""), nil)
	if err != nil {
		return err
	}

	u.SessionId = ""
	return nil
}
