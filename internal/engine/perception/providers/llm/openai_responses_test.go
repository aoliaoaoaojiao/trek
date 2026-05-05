package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"trek/internal/engine/core/types"
	enginestate "trek/internal/engine/state"
)

func TestOpenAIResponsesProviderBuildCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if req["model"] != "gpt-4.1-mini" {
			t.Fatalf("model 错误: %v", req["model"])
		}
		responseFormat, ok := req["response_format"].(map[string]any)
		if !ok || responseFormat["type"] != "json_schema" {
			t.Fatalf("应包含 response_format: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"candidates":[{"intent":"返回","action_type":"BACK","confidence":0.91,"reason":"疑似弹窗"},{"intent":"点击主区域","action_type":"CLICK","point":{"x":0.1,"y":0.2},"confidence":0.7}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		Step:        10,
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
	if items[0].Command == nil || items[0].Command.Act != types.BACK {
		t.Fatalf("第一条候选动作错误: %+v", items[0].Command)
	}
	if items[0].Metadata["llm_reason"] == "" {
		t.Fatalf("应包含 llm_reason")
	}
	// point 格式：x=0.1, y=0.2 → 创建以点为中心的小矩形
	if items[1].Command == nil || items[1].Command.Act != types.CLICK {
		t.Fatalf("第二条候选动作错误: %+v", items[1].Command)
	}
	// 以 (0.1, 0.2) 为中心，±1% 的矩形
	if items[1].Command.Pos.Left >= 0.1 || items[1].Command.Pos.Top >= 0.2 {
		t.Fatalf("点击候选坐标解析错误: pos=%+v", items[1].Command.Pos)
	}
	if items[1].Command.Pos.Right <= 0.1 || items[1].Command.Pos.Bottom <= 0.2 {
		t.Fatalf("点击候选坐标解析错误: pos=%+v", items[1].Command.Pos)
	}
}

func TestOpenAIResponsesProviderWithScreenshot(t *testing.T) {
	// 验证截图通过 input_image content block 传入
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		// 检查 messages 中有 image_url block
		inputArr, ok := req["messages"].([]any)
		if !ok || len(inputArr) < 2 {
			t.Fatalf("messages 应有至少 2 条消息")
		}
		userMsg, ok := inputArr[1].(map[string]any)
		if !ok {
			t.Fatalf("第二条消息应为 user 消息")
		}
		contentArr, ok := userMsg["content"].([]any)
		if !ok || len(contentArr) < 2 {
			t.Fatalf("user content 应包含文本和截图两个 block")
		}
		// 第一个 block 应是 text，第二个应是 image_url
		textBlock, _ := contentArr[0].(map[string]any)
		if textBlock["type"] != "text" {
			t.Fatalf("第一个 block 应为 text, 实际: %v", textBlock["type"])
		}
		imgBlock, _ := contentArr[1].(map[string]any)
		if imgBlock["type"] != "image_url" {
			t.Fatalf("第二个 block 应为 image_url, 实际: %v", imgBlock["type"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"candidates":[{"action_type":"BACK","confidence":0.95}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4.1-mini",
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
}

func TestNewOpenAIResponsesProviderValidateConfig(t *testing.T) {
	_, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		Model: "gpt-4.1-mini",
	})
	if err == nil {
		t.Fatalf("未提供 api key 应报错")
	}
}

func TestOpenAIResponsesProviderDetectPageControls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		responseFormat, ok := req["response_format"].(map[string]any)
		if !ok || responseFormat["type"] != "json_schema" {
			t.Fatalf("应包含 response_format 配置")
		}
		jsonSchema, ok := responseFormat["json_schema"].(map[string]any)
		if !ok || jsonSchema["name"] != "trek_page_controls" {
			t.Fatalf("控件检测 schema 名称错误: %+v", responseFormat)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"controls":[{"action_type":"click","control_type":"button","text":"确定","hint":"确认","clickable":true,"confidence":0.88,"bounds":{"left":0.2,"top":0.3,"right":0.5,"bottom":0.6}}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	items, err := provider.DetectPageControls(enginestate.TraversalContext{
		PageName:   "DialogPage",
		Screenshot: []byte("fake-image"),
	})
	if err != nil {
		t.Fatalf("控件检测失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("控件候选数量错误: %d", len(items))
	}
	if items[0].Metadata["llm_control_text"] != "确定" {
		t.Fatalf("控件文本元数据错误: %+v", items[0].Metadata)
	}
}

func TestOpenAIResponsesProviderDetectPageControlsWithBoundsArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"controls":[{"action_type":"click","control_type":"button","text":"确定","hint":"确认","clickable":true,"confidence":0.88,"bounds":[0.2,0.3,0.5,0.6]}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4.1-mini",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	items, err := provider.DetectPageControls(enginestate.TraversalContext{
		PageName:   "DialogPage",
		Screenshot: []byte("fake-image"),
	})
	if err != nil {
		t.Fatalf("控件检测失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("控件候选数量错误: %d", len(items))
	}
	if items[0].Command == nil || items[0].Command.Pos.Left != 0.2 || items[0].Command.Pos.Bottom != 0.6 {
		t.Fatalf("数组 bounds 解析错误: %+v", items[0].Command)
	}
}

func TestNewOpenAIResponsesProviderNormalizeBaseURL(t *testing.T) {
	t.Run("自动补全_v1_chat_completions", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/chat/completions" {
				t.Fatalf("请求路径错误: %s", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"content": `{"controls":[{"action_type":"click","control_type":"button","text":"确定","clickable":true,"confidence":0.88,"bounds":{"left":0.2,"top":0.3,"right":0.5,"bottom":0.6}}]}`,
						},
					},
				},
			})
		}))
		defer server.Close()

		provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
			BaseURL: server.URL + "/v1",
			APIKey:  "sk-test",
			Model:   "gpt-4.1-mini",
		})
		if err != nil {
			t.Fatalf("创建 provider 失败: %v", err)
		}
		if !strings.HasSuffix(provider.baseURL, "/v1/chat/completions") {
			t.Fatalf("归一化后的 baseURL 错误: %s", provider.baseURL)
		}

		items, err := provider.DetectPageControls(enginestate.TraversalContext{
			PageName:   "DialogPage",
			Screenshot: []byte("fake-image"),
		})
		if err != nil {
			t.Fatalf("控件检测失败: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("控件候选数量错误: %d", len(items))
		}
	})

	t.Run("完整_chat_completions_endpoint_保持不变", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/custom/chat/completions" {
				t.Fatalf("请求路径错误: %s", r.URL.Path)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"content": `{"controls":[{"action_type":"click","control_type":"button","text":"返回","clickable":true,"confidence":0.88,"bounds":{"left":0.1,"top":0.1,"right":0.2,"bottom":0.2}}]}`,
						},
					},
				},
			})
		}))
		defer server.Close()

		provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
			BaseURL: server.URL + "/custom/chat/completions",
			APIKey:  "sk-test",
			Model:   "gpt-4.1-mini",
		})
		if err != nil {
			t.Fatalf("创建 provider 失败: %v", err)
		}
		if provider.baseURL != server.URL+"/custom/chat/completions" {
			t.Fatalf("完整 endpoint 不应被改写: %s", provider.baseURL)
		}
	})
}
