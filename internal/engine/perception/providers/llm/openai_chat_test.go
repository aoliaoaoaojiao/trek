package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"trek/internal/engine/core/types"
	enginestate "trek/internal/engine/state"
)

func TestOpenAIChatProviderBuildCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-chat-test" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if req["model"] != "gpt-4o" {
			t.Fatalf("model 错误: %v", req["model"])
		}
		// 验证 messages 结构
		messages, ok := req["messages"].([]any)
		if !ok || len(messages) < 2 {
			t.Fatalf("messages 应有至少 2 条")
		}
		// 验证 response_format
		respFmt, ok := req["response_format"].(map[string]any)
		if !ok {
			t.Fatalf("response_format 类型错误")
		}
		if respFmt["type"] != "json_schema" {
			t.Fatalf("response_format.type 应为 json_schema, 实际: %v", respFmt["type"])
		}

		// 返回 Chat Completions 格式响应
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"candidates":[{"intent":"关闭弹窗","action_type":"BACK","confidence":0.92,"reason":"检测到遮挡弹窗"},{"intent":"点击登录按钮","action_type":"CLICK","point":{"x":0.5,"y":0.7},"confidence":0.85}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIChatProvider(OpenAIChatProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-chat-test",
		Model:   "gpt-4o",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		Step:     15,
		Mode:     "Recover",
		PageName: "LoginActivity",
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Command == nil || items[0].Command.Act != types.BACK {
		t.Fatalf("第一条候选动作错误: %+v", items[0].Command)
	}
	if items[0].Metadata["llm_reason"] == "" {
		t.Fatalf("应包含 llm_reason")
	}
	if items[1].Command == nil || items[1].Command.Act != types.CLICK {
		t.Fatalf("第二条候选动作错误: %+v", items[1].Command)
	}
	// point 格式验证：以 (0.5, 0.7) 为中心的小矩形
	if items[1].Command.Pos.Left >= 0.5 || items[1].Command.Pos.Right <= 0.5 {
		t.Fatalf("点击候选 x 坐标解析错误: pos=%+v", items[1].Command.Pos)
	}
	if items[1].Command.Pos.Top >= 0.7 || items[1].Command.Pos.Bottom <= 0.7 {
		t.Fatalf("点击候选 y 坐标解析错误: pos=%+v", items[1].Command.Pos)
	}
}

func TestOpenAIChatProviderWithScreenshot(t *testing.T) {
	// 验证截图通过 image_url content block 传入
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		received <- req

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"candidates":[{"action_type":"BACK","confidence":0.95}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIChatProvider(OpenAIChatProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4o",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		Step:       5,
		Mode:       "Recover",
		Screenshot: []byte("fake-screenshot-data"),
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}

	// 验证请求结构
	req := <-received
	messages, ok := req["messages"].([]any)
	if !ok || len(messages) < 2 {
		t.Fatalf("messages 应有至少 2 条")
	}
	userMsg, ok := messages[1].(map[string]any)
	if !ok {
		t.Fatalf("第二条消息应为 user 消息")
	}
	content, ok := userMsg["content"].([]any)
	if !ok || len(content) < 2 {
		t.Fatalf("user content 应包含文本和截图，实际长度: %v", content)
	}
	textBlock, _ := content[0].(map[string]any)
	if textBlock["type"] != "text" {
		t.Fatalf("第一个 block 应为 text, 实际: %v", textBlock["type"])
	}
	imgBlock, _ := content[1].(map[string]any)
	if imgBlock["type"] != "image_url" {
		t.Fatalf("第二个 block 应为 image_url, 实际: %v", imgBlock["type"])
	}
	imgURL, ok := imgBlock["image_url"].(map[string]any)
	if !ok {
		t.Fatalf("image_url 应为对象")
	}
	urlStr, _ := imgURL["url"].(string)
	if len(urlStr) == 0 || urlStr[:5] != "data:" {
		t.Fatalf("image_url.url 应为 data URI")
	}
}

func TestOpenAIChatProviderNoAPIKey(t *testing.T) {
	// 允许不提供 API Key（用于本地模型或公开端点）
	provider, err := NewOpenAIChatProvider(OpenAIChatProviderConfig{
		Model:   "local-model",
		APIKey:  "", // 空 key 应允许
		BaseURL: "http://localhost:8080/v1/chat/completions",
	})
	if err != nil {
		t.Fatalf("空 API Key 应允许: %v", err)
	}
	if provider.apiKey != "" {
		t.Fatalf("apiKey 应为空, 实际: %s", provider.apiKey)
	}
}

func TestOpenAIChatProviderValidateConfig(t *testing.T) {
	_, err := NewOpenAIChatProvider(OpenAIChatProviderConfig{})
	if err == nil {
		t.Fatalf("未提供 model 应报错")
	}
}

func TestExtractChatContent(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name:    "正常响应",
			body:    `{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`,
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "空 choices",
			body:    `{"choices":[]}`,
			wantErr: true,
		},
		{
			name:    "空 content",
			body:    `{"choices":[{"message":{"role":"assistant","content":""}}]}`,
			wantErr: true,
		},
		{
			name:    "无效 JSON",
			body:    `{invalid}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractChatContent([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望错误，实际无错误")
				}
				return
			}
			if err != nil {
				t.Fatalf("未期望错误: %v", err)
			}
			if got != tt.want {
				t.Fatalf("期望 %q, 实际 %q", tt.want, got)
			}
		})
	}
}
