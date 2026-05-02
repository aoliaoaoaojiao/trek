package providers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
)

const (
	defaultLLMTimeout          = 15 * time.Second
	llmRetryBackoff429         = 20 * time.Second
	llmRetryBackoffServerError = 2 * time.Second
)

// LLMHTTPProviderConfig 定义基于 HTTP 的 LLM 候选提供器配置。
type LLMHTTPProviderConfig struct {
	Endpoint string
	APIKey   string
	Model    string
	Timeout  time.Duration
	Headers  map[string]string
}

// LLMHTTPProvider 通过外部 HTTP 接口获取恢复候选。
type LLMHTTPProvider struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
	headers  map[string]string
	sleep    func(time.Duration)
}

// NewLLMHTTPProvider 创建 HTTP LLM 提供器。
func NewLLMHTTPProvider(cfg LLMHTTPProviderConfig) (*LLMHTTPProvider, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("llm endpoint 不能为空")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultLLMTimeout
	}
	headers := make(map[string]string, len(cfg.Headers))
	for key, value := range cfg.Headers {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		headers[k] = value
	}
	return &LLMHTTPProvider{
		endpoint: endpoint,
		apiKey:   strings.TrimSpace(cfg.APIKey),
		model:    strings.TrimSpace(cfg.Model),
		client:   &http.Client{Timeout: timeout},
		headers:  headers,
		sleep:    time.Sleep,
	}, nil
}

// BuildCandidates 调用 LLM 接口并转换为统一 Candidate。
func (p *LLMHTTPProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if p == nil {
		return nil, nil
	}
	prompt := buildRecoveryPrompt(ctx)

	payload := llmRequest{
		Model:            p.model,
		Instruction:      prompt.SystemContent,
		UserMessage:      prompt.UserContent,
		Context:          prompt.ContextFields,
		ScreenshotBase64: prompt.ScreenshotBase64(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	body, status, err := p.postWithRetry(data)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("llm endpoint 响应异常: status=%d body=%s", status, truncateText(string(body), 512))
	}

	var output llmResponse
	if err := json.Unmarshal(body, &output); err != nil {
		return nil, fmt.Errorf("解析 llm 响应失败: %w", err)
	}
	return parseLLMCandidates(output), nil
}

// DetectPageControls 调用 LLM 接口输出页面控件区域，而不是恢复动作。
func (p *LLMHTTPProvider) DetectPageControls(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if p == nil {
		return nil, nil
	}
	prompt := buildPageControlPrompt(ctx)
	payload := llmRequest{
		Model:            p.model,
		Instruction:      prompt.SystemContent,
		UserMessage:      prompt.UserContent,
		ScreenshotBase64: prompt.ScreenshotBase64(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	body, status, err := p.postWithRetry(data)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("llm endpoint 响应异常: status=%d body=%s", status, truncateText(string(body), 512))
	}

	var output pageControlResponse
	if err := json.Unmarshal(body, &output); err != nil {
		return nil, fmt.Errorf("解析 llm 控件检测响应失败: %w", err)
	}
	return parsePageControlCandidates(output), nil
}

func (p *LLMHTTPProvider) postWithRetry(payload []byte) ([]byte, int, error) {
	tryOnce := func() ([]byte, int, error) {
		req, err := http.NewRequest(http.MethodPost, p.endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
		for key, value := range p.headers {
			req.Header.Set(key, value)
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
		if isTimeoutError(err) {
			p.retrySleep(llmRetryBackoffServerError)
			return tryOnce()
		}
		return nil, 0, err
	}
	if status == http.StatusTooManyRequests {
		p.retrySleep(llmRetryBackoff429)
		return tryOnce()
	}
	if status >= 500 {
		p.retrySleep(llmRetryBackoffServerError)
		return tryOnce()
	}
	return body, status, nil
}

func (p *LLMHTTPProvider) retrySleep(delay time.Duration) {
	if p == nil || p.sleep == nil {
		time.Sleep(delay)
		return
	}
	p.sleep(delay)
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

type llmRequest struct {
	Model            string                `json:"model,omitempty"`
	Instruction      string                `json:"instruction"`
	UserMessage      string                `json:"user_message,omitempty"`
	Context          RecoveryContextFields `json:"context"`
	ScreenshotBase64 string                `json:"screenshot_base64,omitempty"`
}

type llmResponse struct {
	Candidates []llmCandidate `json:"candidates"`
	Actions    []llmCandidate `json:"actions"`
}

// llmCandidate 是 LLM 返回的候选动作。
// Point 使用归一化坐标 [0,1]：x 水平（左→右），y 垂直（上→下）。
type llmCandidate struct {
	Intent      string    `json:"intent"`
	ActionType  string    `json:"action_type"`
	Point       *llmPoint `json:"point,omitempty"`
	Bounds      []float64 `json:"bounds,omitempty"` // 向后兼容：旧 provider 仍可能返回 bounds
	Confidence  float64   `json:"confidence"`
	EscapeScore float64   `json:"escape_score"`
	Reason      string    `json:"reason"`
	TargetHint  string    `json:"target_hint"`
}

// llmPoint 是归一化坐标点。
type llmPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// toActionCommand 将 LLM 候选动作转换为引擎动作命令。
// 优先使用 point（归一化坐标），回退到 bounds（向后兼容）。
// point 转为以 (x,y) 为中心的小矩形，避免 IsEmpty() 判定为空。
const pointBoxSize = 0.02 // 归一化坐标系下 ±1% 的点击区域

func toActionCommand(raw llmCandidate) (*types.ActionCommand, bool) {
	act, ok := parseActionType(raw.ActionType)
	if !ok {
		return nil, false
	}
	cmd := types.NewActionCommand()
	cmd.Act = act

	// 优先使用归一化 point 坐标，转换为以点为中心的小矩形
	if raw.Point != nil {
		half := pointBoxSize / 2
		cmd.Pos = *types.NewRect(
			raw.Point.X-half,
			raw.Point.Y-half,
			raw.Point.X+half,
			raw.Point.Y+half,
		)
	} else if len(raw.Bounds) == 4 {
		// 向后兼容：旧 provider 返回 bounds 数组
		cmd.Pos = *types.NewRect(raw.Bounds[0], raw.Bounds[1], raw.Bounds[2], raw.Bounds[3])
	}

	return cmd, true
}

func parseActionType(text string) (types.ActionType, bool) {
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "BACK":
		return types.BACK, true
	case "CLICK":
		return types.CLICK, true
	case "LONG_CLICK":
		return types.LONG_CLICK, true
	case "SCROLL_TOP_DOWN":
		return types.SCROLL_TOP_DOWN, true
	case "SCROLL_BOTTOM_UP":
		return types.SCROLL_BOTTOM_UP, true
	case "SCROLL_LEFT_RIGHT":
		return types.SCROLL_LEFT_RIGHT, true
	case "SCROLL_RIGHT_LEFT":
		return types.SCROLL_RIGHT_LEFT, true
	case "SCROLL_BOTTOM_UP_N":
		return types.SCROLL_BOTTOM_UP_N, true
	case "ACTIVATE":
		return types.ACTIVATE, true
	default:
		return types.NOP, false
	}
}

func truncateText(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}
