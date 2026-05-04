package llm

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	enginestate "trek/internal/engine/state"
)

func TestAnthropicMessagesProviderDetectPageControls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		if got := r.Header.Get("api-key"); got != "sk-test" {
			t.Fatalf("api-key 错误: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if req["model"] != "mimo-v2.5" {
			t.Fatalf("model 错误: %v", req["model"])
		}
		messages, ok := req["messages"].([]any)
		if !ok || len(messages) != 1 {
			t.Fatalf("messages 数量错误: %+v", req["messages"])
		}
		userMsg, ok := messages[0].(map[string]any)
		if !ok || userMsg["role"] != "user" {
			t.Fatalf("user message 格式错误: %+v", messages[0])
		}
		content, ok := userMsg["content"].([]any)
		if !ok || len(content) < 2 {
			t.Fatalf("content 应包含文本和图片: %+v", userMsg["content"])
		}
		textBlock, _ := content[0].(map[string]any)
		if textBlock["type"] != "text" {
			t.Fatalf("第一个 block 应为 text: %+v", textBlock)
		}
		imageBlock, _ := content[1].(map[string]any)
		if imageBlock["type"] != "image" {
			t.Fatalf("第二个 block 应为 image: %+v", imageBlock)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": `{"controls":[{"action_type":"click","control_type":"button","text":"确定","clickable":true,"confidence":0.91,"bounds":[0.2,0.3,0.5,0.6]}]}`,
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewAnthropicMessagesProvider(AnthropicMessagesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "mimo-v2.5",
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
		t.Fatalf("控件文本错误: %+v", items[0].Metadata)
	}
}

func TestAnthropicMessagesProviderBuildCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": `{"candidates":[{"intent":"返回","action_type":"BACK","confidence":0.95}]}`,
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewAnthropicMessagesProvider(AnthropicMessagesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "mimo-v2.5-pro",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		PageName: "MainPage",
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
}

func TestNewAnthropicMessagesProviderNormalizeBaseURL(t *testing.T) {
	provider, err := NewAnthropicMessagesProvider(AnthropicMessagesProviderConfig{
		BaseURL: "https://api.xiaomimimo.com/anthropic",
		APIKey:  "sk-test",
		Model:   "mimo-v2.5-pro",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if provider.baseURL != "https://api.xiaomimimo.com/anthropic/v1/messages" {
		t.Fatalf("baseURL 归一化错误: %s", provider.baseURL)
	}
}

func TestAnthropicMessagesProviderEncodesScreenshotAsBase64Image(t *testing.T) {
	rawImage := []byte("raw-image")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		messages := req["messages"].([]any)
		content := messages[0].(map[string]any)["content"].([]any)
		imageBlock := content[1].(map[string]any)
		source := imageBlock["source"].(map[string]any)
		if source["type"] != "base64" {
			t.Fatalf("图片 source.type 错误: %+v", source)
		}
		if source["media_type"] != "image/png" {
			t.Fatalf("图片 media_type 错误: %+v", source)
		}
		if source["data"] != base64.StdEncoding.EncodeToString(rawImage) {
			t.Fatalf("图片 base64 编码错误: %+v", source)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": `{"controls":[{"action_type":"click","control_type":"button","text":"确定","clickable":true,"confidence":0.88,"bounds":{"left":0.2,"top":0.3,"right":0.5,"bottom":0.6}}]}`,
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewAnthropicMessagesProvider(AnthropicMessagesProviderConfig{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "mimo-v2.5",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	_, err = provider.DetectPageControls(enginestate.TraversalContext{
		PageName:   "DialogPage",
		Screenshot: rawImage,
	})
	if err != nil {
		t.Fatalf("控件检测失败: %v", err)
	}
}

func TestExtractAnthropicText(t *testing.T) {
	text, err := extractAnthropicText([]byte(`{"content":[{"type":"text","text":"{\"controls\":[]}"}]}`))
	if err != nil {
		t.Fatalf("提取文本失败: %v", err)
	}
	if !strings.Contains(text, `"controls"`) {
		t.Fatalf("提取结果错误: %s", text)
	}
}
