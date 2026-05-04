//go:build integration

package providers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
	"trek/internal/testutil"

	"github.com/stretchr/testify/require"
)

type fixtureExpectedControl struct {
	name    string
	aliases []string
	bounds  [4]float64
}

var gameNavigationExpectedControls = []fixtureExpectedControl{
	{
		name:    "basic",
		aliases: []string{"basic"},
		bounds:  [4]float64{0.1950, 0.1646, 0.2900, 0.2593},
	},
	{
		name:    "back",
		aliases: []string{"back"},
		bounds:  [4]float64{0.1614, 0.8292, 0.2048, 0.8913},
	},
}

var gameNavigationOCRExpectedControls = []fixtureExpectedControl{
	{
		name:    "basic",
		aliases: []string{"basic"},
		bounds:  [4]float64{0.1950, 0.1646, 0.2900, 0.2593},
	},
	{
		name:    "drag drop",
		aliases: []string{"drag drop"},
		bounds:  [4]float64{0.1950, 0.2904, 0.2893, 0.3851},
	},
	{
		name:    "wait ui 2",
		aliases: []string{"wait ui 2"},
		bounds:  [4]float64{0.3291, 0.1646, 0.4235, 0.2593},
	},
	{
		name:    "list view",
		aliases: []string{"list view"},
		bounds:  [4]float64{0.1950, 0.4115, 0.2893, 0.5062},
	},
	{
		name:    "local",
		aliases: []string{"local"},
		bounds:  [4]float64{0.1950, 0.5419, 0.2893, 0.6366},
	},
	{
		name:    "positioning",
		aliases: []string{"positioning"},
		bounds:  [4]float64{0.1950, 0.5419, 0.2893, 0.7578},
	},
	{
		name:    "wait ui",
		aliases: []string{"wait ui"},
		bounds:  [4]float64{0.1950, 0.7488, 0.2893, 0.8736},
	},
	{
		name:    "back",
		aliases: []string{"back"},
		bounds:  [4]float64{0.1614, 0.8736, 0.2857, 0.9984},
	},
}

func TestOCRIntegration_GameNavigationFixtureAccurateControlBounds(t *testing.T) {
	endpoint, apiKey := testutil.RequireOCREnv(t)
	screenshot := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)

	provider, err := NewOCRHTTPProvider(OCRHTTPProviderConfig{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Timeout:  30 * time.Second,
	})
	require.NoError(t, err, "创建 OCR provider 失败")
	logOCRRawResponseArtifact(t, provider, enginestate.TraversalContext{
		PageName:   "GameNavigation",
		Screenshot: screenshot,
	})

	items, err := provider.BuildCandidates(enginestate.TraversalContext{
		PageName:   "GameNavigation",
		Screenshot: screenshot,
	})
	require.NoError(t, err, "OCR 控件检测失败")
	require.NotEmpty(t, items, "OCR 应返回至少一个候选")
	logOverlayArtifact(t, "ocr_game_navigation_overlay.png", items)

	assertCandidatesHitExpectedControls(t, items, gameNavigationOCRExpectedControls, extractOCRCandidateText)
}

func TestLLMIntegration_GameNavigationFixtureAccurateControlBounds(t *testing.T) {
	screenshot := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	ctx := enginestate.TraversalContext{
		PageName:   "GameNavigation",
		Screenshot: screenshot,
	}

	t.Run("http_provider", func(t *testing.T) {
		endpoint, apiKey, model := testutil.RequireLLMEnv(t)
		provider, err := NewLLMHTTPProvider(LLMHTTPProviderConfig{
			Endpoint: endpoint,
			APIKey:   apiKey,
			Model:    model,
		})
		require.NoError(t, err, "创建 LLM HTTP provider 失败")

		items, err := provider.DetectPageControls(ctx)
		require.NoError(t, err, "LLM HTTP 控件检测失败")
		require.NotEmpty(t, items, "LLM HTTP 应返回至少一个控件")
		logOverlayArtifact(t, "llm_http_game_navigation_overlay.png", items)

		assertCandidatesHitExpectedControls(t, items, gameNavigationExpectedControls, extractLLMCandidateText)
	})

	t.Run("openai_responses", func(t *testing.T) {
		baseURL, apiKey, model := testutil.RequireOpenAIEnv(t)
		provider, err := NewOpenAIResponsesProvider(OpenAIResponsesProviderConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
		})
		require.NoError(t, err, "创建 OpenAI Responses provider 失败")

		items, err := provider.DetectPageControls(ctx)
		require.NoError(t, err, "OpenAI 控件检测失败")
		require.NotEmpty(t, items, "OpenAI 应返回至少一个控件")
		logOverlayArtifact(t, "openai_game_navigation_overlay.png", items)

		assertCandidatesHitExpectedControls(t, items, gameNavigationExpectedControls, extractLLMCandidateText)
	})
}

func logOverlayArtifact(t *testing.T, fileName string, items []candidate.Candidate) {
	t.Helper()
	path := testutil.WriteCandidateOverlayPNG(t, testutil.FixtureGameNavigation, fileName, items)
	t.Logf("标注图已输出: %s", path)
}

func logOCRRawResponseArtifact(t *testing.T, provider *OCRHTTPProvider, ctx enginestate.TraversalContext) {
	t.Helper()
	if provider == nil {
		return
	}
	width, height := decodeImageSize(ctx.Screenshot)
	payload, err := provider.buildPayload(ctx, width, height)
	require.NoError(t, err, "构建 OCR 原始请求失败")

	req, err := http.NewRequest(http.MethodPost, provider.endpoint, bytes.NewReader(payload))
	require.NoError(t, err, "构建 OCR 原始请求失败")
	req.Header.Set("Content-Type", "application/json")
	provider.setAuthHeader(req)
	for key, value := range provider.headers {
		req.Header.Set(key, value)
	}

	resp, err := provider.client.Do(req)
	require.NoError(t, err, "执行 OCR 原始请求失败")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "读取 OCR 原始响应失败")

	formatted := body
	var pretty bytes.Buffer
	if json.Valid(body) && json.Indent(&pretty, body, "", "  ") == nil {
		formatted = pretty.Bytes()
	}
	path := testutil.WriteArtifactBytes(t, "ocr_game_navigation_raw.json", formatted)
	t.Logf("OCR 原始响应已输出: %s (status=%d)", path, resp.StatusCode)

	rawRects := extractRawOCRBoxes(body)
	if len(rawRects) > 0 {
		rawOverlayPath := testutil.WritePixelRectOverlayPNG(t, testutil.FixtureGameNavigation, "ocr_game_navigation_http_overlay.png", rawRects)
		t.Logf("OCR 原始框标注图已输出: %s", rawOverlayPath)
	}
}

func extractRawOCRBoxes(body []byte) []testutil.PixelRect {
	var payload struct {
		Result struct {
			OCRResults []struct {
				PrunedResult struct {
					RecBoxes [][]int `json:"rec_boxes"`
				} `json:"prunedResult"`
			} `json:"ocrResults"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	rects := make([]testutil.PixelRect, 0, 8)
	for _, result := range payload.Result.OCRResults {
		for _, box := range result.PrunedResult.RecBoxes {
			if len(box) != 4 {
				continue
			}
			rects = append(rects, testutil.PixelRect{
				Left:   box[0],
				Top:    box[1],
				Right:  box[2],
				Bottom: box[3],
			})
		}
	}
	return rects
}

func assertCandidatesHitExpectedControls(
	t *testing.T,
	items []candidate.Candidate,
	expected []fixtureExpectedControl,
	textExtractor func(candidate.Candidate) string,
) {
	t.Helper()
	const marginX = 0.03
	const marginY = 0.04

	for _, control := range expected {
		item, text, ok := findCandidateByAliases(items, control.aliases, textExtractor)
		require.Truef(t, ok, "未找到控件 %q，对应候选如下:\n%s", control.name, formatCandidates(items, textExtractor))
		require.NotNilf(t, item.Command, "控件 %q 的 Command 不应为空", control.name)

		centerX := (item.Command.Pos.Left + item.Command.Pos.Right) / 2
		centerY := (item.Command.Pos.Top + item.Command.Pos.Bottom) / 2

		if centerX < control.bounds[0]-marginX || centerX > control.bounds[2]+marginX {
			t.Fatalf("控件 %q 水平中心超出预期区域: text=%q centerX=%.4f expected=[%.4f, %.4f]",
				control.name, text, centerX, control.bounds[0], control.bounds[2])
		}
		if centerY < control.bounds[1]-marginY || centerY > control.bounds[3]+marginY {
			t.Fatalf("控件 %q 垂直中心超出预期区域: text=%q centerY=%.4f expected=[%.4f, %.4f]",
				control.name, text, centerY, control.bounds[1], control.bounds[3])
		}
		t.Logf("命中控件 %s: text=%q bounds=[%.4f,%.4f,%.4f,%.4f] center=[%.4f,%.4f]",
			control.name,
			text,
			item.Command.Pos.Left, item.Command.Pos.Top,
			item.Command.Pos.Right, item.Command.Pos.Bottom,
			centerX, centerY)
	}
}

func findCandidateByAliases(
	items []candidate.Candidate,
	aliases []string,
	textExtractor func(candidate.Candidate) string,
) (candidate.Candidate, string, bool) {
	for _, item := range items {
		text := normalizeFixtureText(textExtractor(item))
		if text == "" {
			continue
		}
		for _, alias := range aliases {
			normalizedAlias := normalizeFixtureText(alias)
			if normalizedAlias != "" && text == normalizedAlias {
				return item, text, true
			}
		}
	}
	for _, item := range items {
		text := normalizeFixtureText(textExtractor(item))
		if text == "" {
			continue
		}
		for _, alias := range aliases {
			normalizedAlias := normalizeFixtureText(alias)
			if normalizedAlias == "" {
				continue
			}
			if strings.Contains(text, normalizedAlias) || strings.Contains(normalizedAlias, text) {
				return item, text, true
			}
		}
	}
	return candidate.Candidate{}, "", false
}

func extractOCRCandidateText(item candidate.Candidate) string {
	if item.Metadata == nil {
		return ""
	}
	return item.Metadata["ocr_text"]
}

func extractLLMCandidateText(item candidate.Candidate) string {
	if item.Metadata == nil {
		return ""
	}
	if text := strings.TrimSpace(item.Metadata["llm_control_text"]); text != "" {
		return text
	}
	return item.Metadata["llm_target_hint"]
}

func normalizeFixtureText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func formatCandidates(items []candidate.Candidate, textExtractor func(candidate.Candidate) string) string {
	if len(items) == 0 {
		return "(空)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		label := normalizeFixtureText(textExtractor(item))
		if item.Command == nil {
			lines = append(lines, "- text="+label+" command=nil")
			continue
		}
		lines = append(lines,
			"- text="+label+
				" bounds=["+
				formatFloat(item.Command.Pos.Left)+","+
				formatFloat(item.Command.Pos.Top)+","+
				formatFloat(item.Command.Pos.Right)+","+
				formatFloat(item.Command.Pos.Bottom)+"]")
	}
	return strings.Join(lines, "\n")
}

func formatFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 4, 64), "0"), ".")
}
