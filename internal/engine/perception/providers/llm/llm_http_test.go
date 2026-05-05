package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

func TestLLMHTTPProviderBuildCandidates(t *testing.T) {
	respData := map[string]any{
		"candidates": []map[string]any{
			{"intent": "返回上一层", "action_type": "BACK", "confidence": 0.9, "reason": "检测到可能卡在弹窗层"},
			{"intent": "点击主区域", "action_type": "CLICK", "point": map[string]any{"x": 0.2, "y": 0.3}, "confidence": 0.8},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if _, ok := req["user_message"]; !ok {
			t.Fatalf("请求应包含 user_message 字段")
		}
		json.NewEncoder(w).Encode(respData)
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Model:    "gpt-x",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		Step:        12,
		Mode:        "Recover",
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Source != perception.SourceLLM || items[1].Source != perception.SourceLLM {
		t.Fatalf("候选来源错误: %s %s", items[0].Source, items[1].Source)
	}
	if items[0].Metadata["llm_reason"] == "" {
		t.Fatalf("应包含 llm_reason 元数据")
	}
	// point 格式：x=0.2, y=0.3
	// point 格式：x=0.2, y=0.3 → 创建以点为中心的小矩形
	if items[1].Command == nil || items[1].Command.Act != types.CLICK {
		t.Fatalf("点击候选动作错误: %+v", items[1].Command)
	}
	// Left < 0.2 < Right, Top < 0.3 < Bottom
	if items[1].Command.Pos.Left >= 0.2 || items[1].Command.Pos.Top >= 0.3 {
		t.Fatalf("点击候选坐标解析错误: pos=%+v", items[1].Command.Pos)
	}
	if items[1].Command.Pos.Right <= 0.2 || items[1].Command.Pos.Bottom <= 0.3 {
		t.Fatalf("点击候选坐标解析错误: pos=%+v", items[1].Command.Pos)
	}
}

func TestLLMHTTPProviderBoundsBackwardCompatibility(t *testing.T) {
	respBoundsData := map[string]any{
		"candidates": []map[string]any{
			{"action_type": "CLICK", "bounds": []float64{0.1, 0.2, 0.3, 0.4}, "confidence": 0.7},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(respBoundsData)
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Command.Pos.Left != 0.1 || items[0].Command.Pos.Top != 0.2 {
		t.Fatalf("bounds 回退解析错误: pos=%+v", items[0].Command.Pos)
	}
}

func TestLLMHTTPProviderSkipsInvalidCandidates(t *testing.T) {
	respData := map[string]any{
		"candidates": []map[string]any{
			{"action_type": "UNKNOWN_ACTION"},
			{"action_type": "CLICK"}, // 无 point 也无 bounds，属于无效 click
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(respData)
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("应过滤无效候选，实际: %d", len(items))
	}
}

func TestLLMHTTPProviderPointPriorityOverBounds(t *testing.T) {
	respData := map[string]any{
		"candidates": []map[string]any{
			{"action_type": "CLICK", "point": map[string]any{"x": 0.5, "y": 0.6}, "bounds": []float64{0.1, 0.2, 0.3, 0.4}, "confidence": 0.9},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(respData)
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	// 应使用 point 而非 bounds，以 (0.5, 0.6) 为中心
	if items[0].Command.Pos.Left >= 0.5 || items[0].Command.Pos.Top >= 0.6 {
		t.Fatalf("应优先使用 point 坐标，实际: pos=%+v", items[0].Command.Pos)
	}
	if items[0].Command.Pos.Right <= 0.5 || items[0].Command.Pos.Bottom <= 0.6 {
		t.Fatalf("应优先使用 point 坐标，实际: pos=%+v", items[0].Command.Pos)
	}
}

func TestLLMHTTPProviderScreenshotInPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if b64, ok := req["screenshot_base64"].(string); !ok || b64 == "" {
			t.Fatalf("请求应包含非空的 screenshot_base64 字段")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"action_type": "BACK", "confidence": 0.9},
			},
		})
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	_, err = provider.BuildCandidates(enginestate.TraversalContext{
		Step:       1,
		Mode:       "Recover",
		Screenshot: []byte("fake-png-data"),
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
}

func TestLLMHTTPProviderRetriesOn429(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"action_type": "BACK", "confidence": 0.9},
			},
		})
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	var delays []time.Duration
	provider.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{Mode: "Recover"})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if callCount != 2 {
		t.Fatalf("429 场景应重试一次，实际请求次数: %d", callCount)
	}
	if len(delays) != 1 || delays[0] != llmRetryBackoff429 {
		t.Fatalf("429 场景退避时间错误: %+v", delays)
	}
}

func TestLLMHTTPProviderRetriesOn5xx(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"bad gateway"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{"action_type": "BACK", "confidence": 0.9},
			},
		})
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	var delays []time.Duration
	provider.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{Mode: "Recover"})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if callCount != 2 {
		t.Fatalf("5xx 场景应重试一次，实际请求次数: %d", callCount)
	}
	if len(delays) != 1 || delays[0] != llmRetryBackoffServerError {
		t.Fatalf("5xx 场景退避时间错误: %+v", delays)
	}
}

func TestLLMHTTPProviderRetriesOnTimeout(t *testing.T) {
	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: "http://example.local/test"})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	var delays []time.Duration
	provider.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}
	provider.client = &http.Client{
		Transport: &timeoutRetryTransport{},
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{Mode: "Recover"})
	if err != nil {
		t.Fatalf("超时重试后应成功，实际错误: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if len(delays) != 1 || delays[0] != llmRetryBackoffServerError {
		t.Fatalf("超时场景退避时间错误: %+v", delays)
	}
}

func TestLLMHTTPProviderDetectPageControls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if _, ok := req["instruction"].(string); !ok {
			t.Fatalf("请求应包含 instruction")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"controls": []map[string]any{
				{
					"action_type":  "click",
					"control_type": "button",
					"text":         "登录",
					"hint":         "主按钮",
					"clickable":    true,
					"confidence":   0.93,
					"bounds": map[string]any{
						"left":   0.1,
						"top":    0.2,
						"right":  0.3,
						"bottom": 0.4,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	items, err := provider.DetectPageControls(enginestate.TraversalContext{
		PageName:   "LoginPage",
		Screenshot: []byte("fake-png-data"),
	})
	if err != nil {
		t.Fatalf("控件检测失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("控件候选数量错误: %d", len(items))
	}
	if items[0].Metadata["llm_control_text"] != "登录" {
		t.Fatalf("应保留控件文本元数据: %+v", items[0].Metadata)
	}
	if items[0].Command == nil || items[0].Command.Pos.Left != 0.1 || items[0].Command.Pos.Bottom != 0.4 {
		t.Fatalf("控件区域解析错误: %+v", items[0].Command)
	}
}

type timeoutRetryTransport struct {
	callCount int
}

func (t *timeoutRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.callCount++
	if t.callCount == 1 {
		return nil, timeoutNetError{msg: "timeout"}
	}
	body := io.NopCloser(strings.NewReader(`{"candidates":[{"action_type":"BACK","confidence":0.9}]}`))
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     make(http.Header),
	}, nil
}

type timeoutNetError struct {
	msg string
}

func (e timeoutNetError) Error() string   { return e.msg }
func (e timeoutNetError) Timeout() bool   { return true }
func (e timeoutNetError) Temporary() bool { return true }
