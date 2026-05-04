//go:build integration

package llm

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"trek/internal/engine/perception"
	"trek/internal/engine/perception/pagecontrol"
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
		logLLMHTTPRawResponseArtifact(t, provider, ctx, "llm_http_game_navigation_raw.json")

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
		logOpenAIRawResponseArtifact(t, provider, ctx, "openai_game_navigation_raw.json")

		items, err := provider.DetectPageControls(ctx)
		require.NoError(t, err, "OpenAI 控件检测失败")
		require.NotEmpty(t, items, "OpenAI 应返回至少一个控件")
		logOverlayArtifact(t, "openai_game_navigation_overlay.png", items)

		assertCandidatesHitExpectedControls(t, items, gameNavigationExpectedControls, extractLLMCandidateText)
	})

	t.Run("anthropic_messages", func(t *testing.T) {
		baseURL, apiKey, model := testutil.RequireAnthropicEnv(t)
		provider, err := NewAnthropicMessagesProvider(AnthropicMessagesProviderConfig{
			BaseURL: baseURL,
			APIKey:  apiKey,
			Model:   model,
		})
		require.NoError(t, err, "创建 Anthropic Messages provider 失败")
		logAnthropicRawResponseArtifact(t, provider, ctx, "anthropic_game_navigation_raw.json")

		items, err := provider.DetectPageControls(ctx)
		require.NoError(t, err, "Anthropic 控件检测失败")
		require.NotEmpty(t, items, "Anthropic 应返回至少一个控件")
		logOverlayArtifact(t, "anthropic_game_navigation_overlay.png", items)

		assertCandidatesHitExpectedControls(t, items, gameNavigationExpectedControls, extractLLMCandidateText)
	})
}

func logOverlayArtifact(t *testing.T, fileName string, items []perception.Candidate) {
	t.Helper()
	path := testutil.WriteCandidateOverlayPNG(t, testutil.FixtureGameNavigation, fileName, items)
	t.Logf("标注图已输出: %s", path)
}

func logLLMHTTPRawResponseArtifact(t *testing.T, provider *LLMHTTPProvider, ctx enginestate.TraversalContext, fileName string) {
	t.Helper()
	if provider == nil {
		return
	}
	prompt := pagecontrol.BuildPrompt(ctx)
	payload := llmRequest{
		Model:            provider.model,
		Instruction:      prompt.SystemContent,
		UserMessage:      prompt.UserContent,
		ScreenshotBase64: prompt.ScreenshotBase64(),
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err, "构建 LLM HTTP 原始请求失败")

	req, err := http.NewRequest(http.MethodPost, provider.endpoint, bytes.NewReader(data))
	require.NoError(t, err, "构建 LLM HTTP 原始请求失败")
	req.Header.Set("Content-Type", "application/json")
	if provider.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.apiKey)
	}
	for key, value := range provider.headers {
		req.Header.Set(key, value)
	}

	resp, err := provider.client.Do(req)
	require.NoError(t, err, "执行 LLM HTTP 原始请求失败")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "读取 LLM HTTP 原始响应失败")
	t.Logf("%s status=%d\n%s", fileName, resp.StatusCode, prettyJSON(body))
}

func logOpenAIRawResponseArtifact(t *testing.T, provider *OpenAIResponsesProvider, ctx enginestate.TraversalContext, fileName string) {
	t.Helper()
	if provider == nil {
		return
	}
	payload, err := provider.buildPageControlRequestPayload(ctx)
	require.NoError(t, err, "构建 OpenAI 原始请求失败")

	body, status, err := provider.postWithRetry(payload)
	require.NoError(t, err, "执行 OpenAI 原始请求失败")
	t.Logf("%s status=%d\n%s", fileName, status, prettyJSON(body))
}

func logAnthropicRawResponseArtifact(t *testing.T, provider *AnthropicMessagesProvider, ctx enginestate.TraversalContext, fileName string) {
	t.Helper()
	if provider == nil {
		return
	}
	payload, err := provider.buildPageControlRequestPayload(ctx)
	require.NoError(t, err, "构建 Anthropic 原始请求失败")

	body, status, err := provider.postWithRetry(payload)
	require.NoError(t, err, "执行 Anthropic 原始请求失败")
	t.Logf("%s status=%d\n%s", fileName, status, prettyJSON(body))
}

func prettyJSON(body []byte) string {
	formatted := body
	var pretty bytes.Buffer
	if json.Valid(body) && json.Indent(&pretty, body, "", "  ") == nil {
		formatted = pretty.Bytes()
	}
	return string(formatted)
}

func assertCandidatesHitExpectedControls(
	t *testing.T,
	items []perception.Candidate,
	expected []fixtureExpectedControl,
	textExtractor func(perception.Candidate) string,
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
	items []perception.Candidate,
	aliases []string,
	textExtractor func(perception.Candidate) string,
) (perception.Candidate, string, bool) {
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
	return perception.Candidate{}, "", false
}

func extractLLMCandidateText(item perception.Candidate) string {
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

func formatCandidates(items []perception.Candidate, textExtractor func(perception.Candidate) string) string {
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
