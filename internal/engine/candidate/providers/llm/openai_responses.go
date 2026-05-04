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
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
)

const (
	defaultOpenAIResponsesURL = "https://api.openai.com/v1/responses"
	defaultOpenAITimeout      = 20 * time.Second
)

// OpenAIResponsesProviderConfig 定义 OpenAI Responses API 提供器配置。
type OpenAIResponsesProviderConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// OpenAIResponsesProvider 使用 OpenAI Responses API 生成恢复候选。
type OpenAIResponsesProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAIResponsesProvider 创建 OpenAI Responses API 提供器。
func NewOpenAIResponsesProvider(cfg OpenAIResponsesProviderConfig) (*OpenAIResponsesProvider, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("openai model 不能为空")
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openai api key 不能为空")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultOpenAIResponsesURL
	}
	normalizedBaseURL, err := normalizeOpenAIResponsesURL(baseURL)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultOpenAITimeout
	}
	return &OpenAIResponsesProvider{
		baseURL: normalizedBaseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func normalizeOpenAIResponsesURL(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("openai base url 非法: %w", err)
	}
	cleanPath := strings.TrimSpace(parsed.Path)
	switch {
	case cleanPath == "":
		parsed.Path = "/v1/responses"
	case strings.HasSuffix(cleanPath, "/responses"):
		parsed.Path = cleanPath
	case cleanPath == "/v1" || cleanPath == "/v1/":
		parsed.Path = "/v1/responses"
	default:
		parsed.Path = path.Join(cleanPath, "responses")
	}
	return parsed.String(), nil
}

// BuildCandidates 调用 OpenAI Responses API 构建恢复候选。
func (p *OpenAIResponsesProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
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
	if status == http.StatusNotFound {
		return p.buildCandidatesWithChatFallback(ctx)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("openai responses 请求失败: status=%d body=%s", status, truncateText(string(body), 512))
	}

	text, err := extractOutputText(body)
	if err != nil {
		return nil, err
	}
	var output llmResponse
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 openai 结构化输出失败: %w", err)
	}
	return parseLLMCandidates(output), nil
}

// DetectPageControls 调用 OpenAI Responses API 输出页面控件区域。
func (p *OpenAIResponsesProvider) DetectPageControls(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
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
	if status == http.StatusNotFound {
		return p.detectPageControlsWithChatFallback(ctx)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("openai responses 请求失败: status=%d body=%s", status, truncateText(string(body), 512))
	}
	text, err := extractOutputText(body)
	if err != nil {
		return nil, err
	}
	var output pageControlResponse
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 openai 控件检测输出失败: %w", err)
	}
	return parsePageControlCandidates(output), nil
}

// buildRequestPayload 构建多模态请求载荷。
// 截图通过 input_image content block 传入，文本上下文通过 input_text 传入。
func (p *OpenAIResponsesProvider) buildRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	prompt := buildRecoveryPrompt(ctx)

	// 构建 user content blocks：文本 + 可选截图
	userContent := []map[string]any{
		{"type": "input_text", "text": prompt.UserContent},
	}
	if len(prompt.Screenshot) > 0 {
		userContent = append(userContent, map[string]any{
			"type":      "input_image",
			"image_url": prompt.ScreenshotBase64(),
		})
	}

	payload := map[string]any{
		"model": p.model,
		"input": []map[string]any{
			{
				"role": "system",
				"content": []map[string]any{
					{"type": "input_text", "text": prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的候选动作。"},
				},
			},
			{
				"role":    "user",
				"content": userContent,
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "trek_recovery_candidates",
				"strict": true,
				"schema": prompt.ResponseSchema,
			},
		},
	}
	return json.Marshal(payload)
}

func (p *OpenAIResponsesProvider) buildPageControlRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	prompt := buildPageControlPrompt(ctx)
	userContent := []map[string]any{
		{"type": "input_text", "text": prompt.UserContent},
	}
	if len(prompt.Screenshot) > 0 {
		userContent = append(userContent, map[string]any{
			"type":      "input_image",
			"image_url": prompt.ScreenshotBase64(),
		})
	}
	payload := map[string]any{
		"model": p.model,
		"input": []map[string]any{
			{
				"role": "system",
				"content": []map[string]any{
					{"type": "input_text", "text": prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的控件列表。"},
				},
			},
			{
				"role":    "user",
				"content": userContent,
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "trek_page_controls",
				"strict": true,
				"schema": prompt.ResponseSchema,
			},
		},
	}
	return json.Marshal(payload)
}

func (p *OpenAIResponsesProvider) postWithRetry(payload []byte) ([]byte, int, error) {
	tryOnce := func() ([]byte, int, error) {
		req, err := http.NewRequest(http.MethodPost, p.baseURL, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		const maxLLMBodySize = 50 * 1024 * 1024 // 50MB
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

func (p *OpenAIResponsesProvider) buildCandidatesWithChatFallback(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	chatProvider, err := p.newChatFallbackProvider()
	if err != nil {
		return nil, err
	}
	return chatProvider.BuildCandidates(ctx)
}

func (p *OpenAIResponsesProvider) detectPageControlsWithChatFallback(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	chatProvider, err := p.newChatFallbackProvider()
	if err != nil {
		return nil, err
	}
	return chatProvider.DetectPageControls(ctx)
}

func (p *OpenAIResponsesProvider) newChatFallbackProvider() (*OpenAIChatProvider, error) {
	chatURL, err := normalizeOpenAIChatURLFromResponsesURL(p.baseURL)
	if err != nil {
		return nil, fmt.Errorf("openai responses 404 后构建 chat 回退 provider 失败: %w", err)
	}
	return NewOpenAIChatProvider(OpenAIChatProviderConfig{
		BaseURL: chatURL,
		APIKey:  p.apiKey,
		Model:   p.model,
		Timeout: p.client.Timeout,
	})
}

func normalizeOpenAIChatURLFromResponsesURL(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("openai chat base url 非法: %w", err)
	}
	cleanPath := strings.TrimSpace(parsed.Path)
	switch {
	case cleanPath == "":
		parsed.Path = "/v1/chat/completions"
	case cleanPath == "/" || cleanPath == "/v1" || cleanPath == "/v1/":
		parsed.Path = "/v1/chat/completions"
	case strings.HasSuffix(cleanPath, "/responses"):
		parsed.Path = strings.TrimSuffix(cleanPath, "/responses") + "/chat/completions"
	case strings.HasSuffix(cleanPath, "/chat/completions"):
		parsed.Path = cleanPath
	default:
		parsed.Path = path.Join(cleanPath, "chat/completions")
	}
	return parsed.String(), nil
}

func extractOutputText(body []byte) (string, error) {
	var response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if text := strings.TrimSpace(response.OutputText); text != "" {
		return text, nil
	}
	for _, item := range response.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				return strings.TrimSpace(content.Text), nil
			}
		}
	}
	return "", fmt.Errorf("openai responses 返回中缺少 output_text")
}
