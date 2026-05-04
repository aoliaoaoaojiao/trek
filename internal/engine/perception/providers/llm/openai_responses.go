package llm

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

const (
	defaultOpenAITimeout = 20 * time.Second
)

// OpenAIResponsesProviderConfig 定义 OpenAI Responses API 提供器配置。
type OpenAIResponsesProviderConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

// OpenAIResponsesProvider 保留历史命名，但实际统一走 Chat Completions 接口。
type OpenAIResponsesProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAIResponsesProvider 创建 OpenAI provider。
// 兼容历史命名，但内部统一归一化到 chat/completions。
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
		baseURL = defaultOpenAIChatURL
	}
	normalizedBaseURL, err := normalizeOpenAIChatURLFromResponsesURL(baseURL)
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

// BuildCandidates 调用 OpenAI Chat Completions API 构建恢复候选。
func (p *OpenAIResponsesProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	chatProvider, err := p.newChatProvider()
	if err != nil {
		return nil, err
	}
	return chatProvider.BuildCandidates(ctx)
}

// DetectPageControls 调用 OpenAI Chat Completions API 输出页面控件区域。
func (p *OpenAIResponsesProvider) DetectPageControls(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	chatProvider, err := p.newChatProvider()
	if err != nil {
		return nil, err
	}
	return chatProvider.DetectPageControls(ctx)
}

func (p *OpenAIResponsesProvider) buildRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	chatProvider, err := p.newChatProvider()
	if err != nil {
		return nil, err
	}
	return chatProvider.buildRequestPayload(ctx)
}

func (p *OpenAIResponsesProvider) buildPageControlRequestPayload(ctx enginestate.TraversalContext) ([]byte, error) {
	chatProvider, err := p.newChatProvider()
	if err != nil {
		return nil, err
	}
	return chatProvider.buildPageControlRequestPayload(ctx)
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

func (p *OpenAIResponsesProvider) newChatProvider() (*OpenAIChatProvider, error) {
	return NewOpenAIChatProvider(OpenAIChatProviderConfig{
		BaseURL: p.baseURL,
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
