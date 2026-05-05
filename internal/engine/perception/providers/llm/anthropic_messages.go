package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"trek/internal/engine/perception"
	"trek/internal/engine/perception/pagecontrol"
	enginestate "trek/internal/engine/state"
)

const (
	defaultAnthropicMessagesURL     = "https://api.anthropic.com/v1/messages"
	defaultAnthropicMessagesTimeout = 30 * time.Second
	defaultAnthropicMaxTokens       = 2048
)

// AnthropicMessagesProviderConfig 定义 Anthropic Messages API 提供器配置。
type AnthropicMessagesProviderConfig struct {
	BaseURL   string
	APIKey    string
	Model     string
	Timeout   time.Duration
	MaxTokens int
}

// AnthropicMessagesProvider 使用 Anthropic Messages API 生成恢复候选与页面控件。
type AnthropicMessagesProvider struct {
	baseURL   string
	apiKey    string
	model     string
	maxTokens int
	client    *http.Client
}

// NewAnthropicMessagesProvider 创建 Anthropic Messages API 提供器。
func NewAnthropicMessagesProvider(cfg AnthropicMessagesProviderConfig) (*AnthropicMessagesProvider, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("anthropic model 不能为空")
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic api key 不能为空")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultAnthropicMessagesURL
	}
	normalizedBaseURL, err := normalizeAnthropicMessagesURL(baseURL)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultAnthropicMessagesTimeout
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultAnthropicMaxTokens
	}
	return &AnthropicMessagesProvider{
		baseURL:   normalizedBaseURL,
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		client:    &http.Client{Timeout: timeout},
	}, nil
}

func normalizeAnthropicMessagesURL(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("anthropic base url 非法: %w", err)
	}
	cleanPath := strings.TrimSpace(parsed.Path)
	switch {
	case cleanPath == "":
		parsed.Path = "/v1/messages"
	case strings.HasSuffix(cleanPath, "/v1/messages"):
		parsed.Path = cleanPath
	case strings.HasSuffix(cleanPath, "/anthropic"):
		parsed.Path = path.Join(cleanPath, "v1/messages")
	default:
		parsed.Path = path.Join(cleanPath, "messages")
	}
	return parsed.String(), nil
}

// BuildCandidates 调用 Anthropic Messages API 构建恢复候选。
func (p *AnthropicMessagesProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if p == nil {
		return nil, nil
	}
	payload, err := p.buildRequestPayload(ctx)
	if err != nil {
		return nil, err
	}
	body, status, err := p.postWithRetry(payload)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("anthropic messages 请求失败: status=%d body=%s", status, truncateText(string(body), 512))
	}
	text, err := extractAnthropicText(body)
	if err != nil {
		return nil, err
	}
	var output llmResponse
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 anthropic 结构化输出失败: %w", err)
	}
	return parseLLMCandidates(output), nil
}

// DetectPageControls 调用 Anthropic Messages API 输出页面控件区域。
func (p *AnthropicMessagesProvider) DetectPageControls(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if p == nil {
		return nil, nil
	}
	payload, err := p.buildPageControlRequestPayload(ctx)
	if err != nil {
		return nil, err
	}
	body, status, err := p.postWithRetry(payload)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("anthropic messages 请求失败: status=%d body=%s", status, truncateText(string(body), 512))
	}
	text, err := extractAnthropicText(body)
	if err != nil {
		return nil, err
	}
	var output pagecontrol.Response
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 anthropic 控件检测输出失败: %w", err)
	}
	return pagecontrol.ParseCandidates(output), nil
}

func (p *AnthropicMessagesProvider) buildRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	prompt := buildRecoveryPrompt(ctx)
	userContent := p.buildUserContent(prompt.UserContent, prompt.ScreenshotMediaType, prompt.ScreenshotBase64())
	payload := map[string]any{
		"model":       p.model,
		"max_tokens":  p.maxTokens,
		"temperature": 0,
		"system":      prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的候选动作。",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": userContent,
			},
		},
	}
	return json.Marshal(payload)
}

func (p *AnthropicMessagesProvider) buildPageControlRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	prompt := pagecontrol.BuildPrompt(ctx)
	userContent := p.buildUserContent(prompt.UserContent, prompt.ScreenshotMediaType, prompt.ScreenshotBase64())
	payload := map[string]any{
		"model":       p.model,
		"max_tokens":  p.maxTokens,
		"temperature": 0,
		"system":      prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的控件列表。",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": userContent,
			},
		},
	}
	return json.Marshal(payload)
}

func (p *AnthropicMessagesProvider) buildUserContent(textContent, mediaType, screenshotBase64 string) []map[string]any {
	content := []map[string]any{
		{"type": "text", "text": textContent},
	}
	if screenshotBase64 != "" {
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mediaType,
				"data":       screenshotBase64,
			},
		})
	}
	return content
}

func (p *AnthropicMessagesProvider) postWithRetry(payload []byte) ([]byte, int, error) {
	tryOnce := func() ([]byte, int, error) {
		req, err := http.NewRequest(http.MethodPost, p.baseURL, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("api-key", p.apiKey)
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		const maxLLMBodySize = 50 * 1024 * 1024
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxLLMBodySize))
		if readErr != nil {
			return nil, resp.StatusCode, readErr
		}
		return body, resp.StatusCode, nil
	}

	body, status, err := tryOnce()
	if err != nil {
		time.Sleep(2 * time.Second)
		return tryOnce()
	}
	if status == http.StatusTooManyRequests {
		time.Sleep(20 * time.Second)
		return tryOnce()
	}
	if status >= 500 {
		time.Sleep(2 * time.Second)
		return tryOnce()
	}
	return body, status, nil
}

func extractAnthropicText(body []byte) (string, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析 anthropic 响应失败: %w", err)
	}
	for _, item := range response.Content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			return strings.TrimSpace(item.Text), nil
		}
	}
	return "", fmt.Errorf("anthropic 响应中缺少 text content")
}
