package providers

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
)

func TestOCRHTTPProviderBuildCandidatesWithNormalizedBounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method 错误: %s", r.Method)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if req["screenshot_base64"] == "" {
			t.Fatalf("请求应包含 screenshot_base64")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"regions": []map[string]any{
				{"text": "登录", "confidence": 0.88, "bounds": []float64{0.1, 0.2, 0.3, 0.4}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		PageName:   "LoginActivity",
		Screenshot: mustPNG(t, 100, 200),
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Source != candidate.SourceOCR {
		t.Fatalf("候选来源错误: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types.CLICK {
		t.Fatalf("候选动作错误: %+v", items[0].Command)
	}
	if items[0].Command.Pos.Left != 0.1 || items[0].Command.Pos.Top != 0.2 {
		t.Fatalf("归一化坐标解析错误: %+v", items[0].Command.Pos)
	}
	if got := items[0].Metadata["ocr_text"]; got != "登录" {
		t.Fatalf("ocr_text 元数据错误: %s", got)
	}
}

func TestOCRHTTPProviderBuildCandidatesAistudioLayoutRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token test-token" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if _, ok := req["file"].(string); !ok {
			t.Fatalf("aistudio 请求应包含 file 字段")
		}
		if fileType, ok := req["fileType"].(float64); !ok || int(fileType) != 1 {
			t.Fatalf("aistudio 请求 fileType 错误: %+v", req["fileType"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"layoutParsingResults": []map[string]any{
					{
						"lines": []map[string]any{
							{"text": "登录", "bbox": []float64{10, 20, 50, 80}, "confidence": 0.9},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{
		Endpoint: server.URL + "/layout-parsing",
		APIKey:   "test-token",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		Screenshot: mustPNG(t, 100, 200),
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Command == nil || items[0].Command.Act != types.CLICK {
		t.Fatalf("候选动作错误: %+v", items[0].Command)
	}
	if items[0].Command.Pos.Left != 0.1 || items[0].Command.Pos.Top != 0.1 {
		t.Fatalf("bbox 转归一化错误: %+v", items[0].Command.Pos)
	}
}

func TestOCRHTTPProviderBuildCandidatesWithPixelBounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"text": "确定", "bounds": []float64{10, 40, 50, 80}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{Endpoint: server.URL})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		Screenshot: mustPNG(t, 100, 200),
	})
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Command.Pos.Left != 0.1 || items[0].Command.Pos.Top != 0.2 {
		t.Fatalf("像素坐标归一化错误: %+v", items[0].Command.Pos)
	}
	if items[0].Command.Pos.Right != 0.5 || items[0].Command.Pos.Bottom != 0.4 {
		t.Fatalf("像素坐标归一化错误: %+v", items[0].Command.Pos)
	}
	if items[0].Confidence != defaultOCRCandidateConfidence {
		t.Fatalf("默认置信度错误: %v", items[0].Confidence)
	}
}

func TestOCRHTTPProviderSkipsWhenNoScreenshot(t *testing.T) {
	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{Endpoint: "http://example.com"})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	items, err := provider.BuildCandidates(enginestate.TraversalContext{})
	if err != nil {
		t.Fatalf("无截图时不应报错: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("无截图时应返回空候选，实际: %d", len(items))
	}
}

func TestExtractOCRRegionsFromArbitraryJSONPolygon(t *testing.T) {
	raw := []byte(`{
		"result": {
			"layoutParsingResults": [
				{
					"blocks": [
						{
							"text": "按钮",
							"polygon": [[10, 20], [50, 20], [50, 60], [10, 60]],
							"score": 0.8
						}
					]
				}
			]
		}
	}`)
	regions := extractOCRRegionsFromArbitraryJSON(raw)
	if len(regions) != 1 {
		t.Fatalf("提取区域数量错误: %d", len(regions))
	}
	if len(regions[0].Bounds) != 4 {
		t.Fatalf("提取 bounds 错误: %+v", regions[0].Bounds)
	}
	if regions[0].Bounds[0] != 10 || regions[0].Bounds[1] != 20 || regions[0].Bounds[2] != 50 || regions[0].Bounds[3] != 60 {
		t.Fatalf("polygon 包围框错误: %+v", regions[0].Bounds)
	}
}

func mustPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码 png 失败: %v", err)
	}
	return buf.Bytes()
}
