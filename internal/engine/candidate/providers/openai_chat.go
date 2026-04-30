package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
)

const (
	defaultOpenAIChatURL = "https://api.openai.com/v1/chat/completions"
	defaultOpenAIChatTimeout = 30 * time.Second
)

// OpenAIChatProviderConfig 定义 OpenAI Chat Completions API 提供器配置。
// 兼容所有 OpenAI Chat Completions 格式的端点（GPT-4o、Gemini 兼容层、Qwen-VL、Doubao、GLM-4V、vLLM 等）。
type OpenAIChatProviderConfig struct {
	// BaseURL 是 API 端点地址。默认为 OpenAI 官方 Chat Completions URL。
	// 设为其他地址可对接兼容层（如 Azure OpenAI、vLLM、Ollama 等）。
	BaseURL string
	// APIKey 是认证密钥。
	APIKey string
	// Model 是模型名称（如 gpt-4o、gemini-2.0-flash、qwen-vl-max 等）。
	Model string
	// Timeout 是单次请求超时时间。
	Timeout time.Duration
}

// OpenAIChatProvider 使用 OpenAI Chat Completions API 生成恢复候选。
// 支持所有兼容 /v1/chat/completions 格式的多模态模型。
type OpenAIChatProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAIChatProvider 创建 OpenAI Chat Completions API 提供器。
func NewOpenAIChatProvider(cfg OpenAIChatProviderConfig) (*OpenAIChatProvider, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, fmt.Errorf("openai chat model 不能为空")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultOpenAIChatURL
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("openai chat base url 非法: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultOpenAIChatTimeout
	}
	return &OpenAIChatProvider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(cfg.APIKey),
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// BuildCandidates 调用 OpenAI Chat Completions API 构建恢复候选。
func (p *OpenAIChatProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
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
		return nil, fmt.Errorf("openai chat 请求失败: status=%d body=%s", status, truncateText(string(body), 512))
	}

	text, err := extractChatContent(body)
	if err != nil {
		return nil, err
	}
	var output llmResponse
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 openai chat 结构化输出失败: %w", err)
	}
	return parseLLMCandidates(output), nil
}

// buildRequestPayload 构建 Chat Completions API 请求载荷。
// 使用标准 messages 格式，截图通过 image_url content block 传入。
func (p *OpenAIChatProvider) buildRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	prompt := buildRecoveryPrompt(ctx)

	// 构建 user content：文本 + 可选截图
	userContent := []map[string]any{
		{"type": "text", "text": prompt.UserContent},
	}
	if len(prompt.Screenshot) > 0 {
		imageURL := fmt.Sprintf("data:%s;base64,%s", prompt.ScreenshotMediaType, prompt.ScreenshotBase64())
		userContent = append(userContent, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url":    imageURL,
				"detail": "high",
			},
		})
	}

	payload := map[string]any{
		"model": p.model,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的候选动作。",
			},
			{
				"role":    "user",
				"content": userContent,
			},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "trek_recovery_candidates",
				"strict": true,
				"schema": prompt.ResponseSchema,
			},
		},
	}
	return json.Marshal(payload)
}

func (p *OpenAIChatProvider) postWithRetry(payload []byte) ([]byte, int, error) {
	tryOnce := func() ([]byte, int, error) {
		req, err := http.NewRequest(http.MethodPost, p.baseURL, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
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

// extractChatContent 从 Chat Completions 响应中提取 assistant 消息文本。
func extractChatContent(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析 openai chat 响应失败: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openai chat 响应中缺少 choices")
	}
	text := strings.TrimSpace(response.Choices[0].Message.Content)
	if text == "" {
		return "", fmt.Errorf("openai chat 响应中内容为空")
	}
	return text, nil
}

// parseLLMCandidates 将 LLM 响应解析为统一候选列表。
// OpenAI Chat 和 Responses provider 共享此解析逻辑。
func parseLLMCandidates(output llmResponse) []candidate.Candidate {
	rawCandidates := output.Candidates
	if len(rawCandidates) == 0 {
		rawCandidates = output.Actions
	}
	items := make([]candidate.Candidate, 0, len(rawCandidates))
	for _, raw := range rawCandidates {
		cmd, ok := toActionCommand(raw)
		if !ok || cmd == nil || !cmd.IsValid() {
			continue
		}
		metadata := map[string]string{}
		if strings.TrimSpace(raw.Reason) != "" {
			metadata["llm_reason"] = strings.TrimSpace(raw.Reason)
		}
		if strings.TrimSpace(raw.TargetHint) != "" {
			metadata["llm_target_hint"] = strings.TrimSpace(raw.TargetHint)
		}
		item := candidate.NewCandidate(cmd, candidate.SourceLLM, strings.TrimSpace(raw.Intent), metadata)
		item.Confidence = raw.Confidence
		if raw.EscapeScore > 0 {
			item.EscapeScore = raw.EscapeScore
		}
		items = append(items, item)
	}
	return items
}