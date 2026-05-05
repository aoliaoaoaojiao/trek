//go:build integration

package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
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

type llmFixtureExpectedControl struct {
	name    string
	aliases []string
	bounds  [4]float64
}

var gameNavigationLLMExpectedControls = []llmFixtureExpectedControl{
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
		items, err := detectPageControlsWithLoggedHTTPResponse(t, endpoint, apiKey, model, ctx, "llm_http_game_navigation_raw.json")
		require.NoError(t, err, "LLM HTTP 控件检测失败")
		require.NotEmpty(t, items, "LLM HTTP 应返回至少一个控件")
		logLLMOverlayArtifact(t, testutil.FixtureGameNavigation, "llm_http_game_navigation_overlay.png", items)

		assertLLMCandidatesHitExpectedControls(t, items, gameNavigationLLMExpectedControls, extractLLMCandidateText)
	})

	t.Run("openai_responses", func(t *testing.T) {
		baseURL, apiKey, model := testutil.RequireOpenAIEnv(t)
		items, err := detectPageControlsWithLoggedOpenAIResponse(t, baseURL, apiKey, model, ctx, "openai_game_navigation_raw.json")
		require.NoError(t, err, "OpenAI 控件检测失败")
		require.NotEmpty(t, items, "OpenAI 应返回至少一个控件")
		logLLMOverlayArtifact(t, testutil.FixtureGameNavigation, "openai_game_navigation_overlay.png", items)

		assertLLMCandidatesHitExpectedControls(t, items, gameNavigationLLMExpectedControls, extractLLMCandidateText)
	})

	t.Run("anthropic_messages", func(t *testing.T) {
		baseURL, apiKey, model := testutil.RequireAnthropicEnv(t)
		items, err := detectPageControlsWithLoggedAnthropicResponse(t, baseURL, apiKey, model, ctx, "anthropic_game_navigation_raw.json")
		require.NoError(t, err, "Anthropic 控件检测失败")
		require.NotEmpty(t, items, "Anthropic 应返回至少一个控件")
		logLLMOverlayArtifact(t, testutil.FixtureGameNavigation, "anthropic_game_navigation_overlay.png", items)

		assertLLMCandidatesHitExpectedControls(t, items, gameNavigationLLMExpectedControls, extractLLMCandidateText)
	})
}

func TestLLMIntegration_BatchAllFixturesReturnCandidates(t *testing.T) {
	fixtures := testutil.ListRootFixtures(t)

	t.Run("http_provider", func(t *testing.T) {
		endpoint, apiKey, model := testutil.RequireLLMEnv(t)
		for _, fixtureName := range fixtures {
			fixtureName := fixtureName
			t.Run(testutil.FixtureStem(fixtureName), func(t *testing.T) {
				screenshot := testutil.ReadRootFixture(t, fixtureName)
				ctx := enginestate.TraversalContext{
					PageName:   testutil.FixtureStem(fixtureName),
					Screenshot: screenshot,
				}
				items, err := detectPageControlsWithLoggedHTTPResponse(t, endpoint, apiKey, model, ctx, testutil.FixtureStem(fixtureName)+"_llm_http_raw.json")
				require.NoError(t, err, "LLM HTTP 控件检测失败")
				require.NotEmpty(t, items, "LLM HTTP 应返回至少一个控件")
				logLLMOverlayArtifact(t, fixtureName, testutil.FixtureStem(fixtureName)+"_llm_http_overlay.png", items)
			})
		}
	})

	t.Run("openai_responses", func(t *testing.T) {
		baseURL, apiKey, model := testutil.RequireOpenAIEnv(t)
		for _, fixtureName := range fixtures {
			fixtureName := fixtureName
			t.Run(testutil.FixtureStem(fixtureName), func(t *testing.T) {
				screenshot := testutil.ReadRootFixture(t, fixtureName)
				ctx := enginestate.TraversalContext{
					PageName:   testutil.FixtureStem(fixtureName),
					Screenshot: screenshot,
				}
				items, err := detectPageControlsWithLoggedOpenAIResponse(t, baseURL, apiKey, model, ctx, testutil.FixtureStem(fixtureName)+"_openai_raw.json")
				require.NoError(t, err, "OpenAI 控件检测失败")
				require.NotEmpty(t, items, "OpenAI 应返回至少一个控件")
				logLLMOverlayArtifact(t, fixtureName, testutil.FixtureStem(fixtureName)+"_openai_overlay.png", items)
			})
		}
	})

	t.Run("anthropic_messages", func(t *testing.T) {
		baseURL, apiKey, model := testutil.RequireAnthropicEnv(t)
		for _, fixtureName := range fixtures {
			fixtureName := fixtureName
			t.Run(testutil.FixtureStem(fixtureName), func(t *testing.T) {
				screenshot := testutil.ReadRootFixture(t, fixtureName)
				ctx := enginestate.TraversalContext{
					PageName:   testutil.FixtureStem(fixtureName),
					Screenshot: screenshot,
				}
				items, err := detectPageControlsWithLoggedAnthropicResponse(t, baseURL, apiKey, model, ctx, testutil.FixtureStem(fixtureName)+"_anthropic_raw.json")
				require.NoError(t, err, "Anthropic 控件检测失败")
				require.NotEmpty(t, items, "Anthropic 应返回至少一个控件")
				logLLMOverlayArtifact(t, fixtureName, testutil.FixtureStem(fixtureName)+"_anthropic_overlay.png", items)
			})
		}
	})
}

func logLLMOverlayArtifact(t *testing.T, fixtureName string, fileName string, items []perception.Candidate) {
	t.Helper()
	path := testutil.WriteCandidateOverlayPNG(t, fixtureName, fileName, items)
	t.Logf("标注图已输出: %s", path)
}

func detectPageControlsWithLoggedHTTPResponse(
	t *testing.T,
	endpoint string,
	apiKey string,
	model string,
	ctx enginestate.TraversalContext,
	fileName string,
) ([]perception.Candidate, error) {
	t.Helper()
	prompt := pagecontrol.BuildPrompt(ctx)
	payload := map[string]any{
		"model":             strings.TrimSpace(model),
		"instruction":       prompt.SystemContent,
		"user_message":      prompt.UserContent,
		"screenshot_base64": prompt.ScreenshotBase64(),
	}
	body, status, err := postJSONWithBearerAuth(endpoint, apiKey, payload, nil)
	if err != nil {
		return nil, err
	}
	t.Logf("%s status=%d\n%s", fileName, status, prettyLLMJSON(body))
	if status < 200 || status >= 300 {
		return nil, errUnexpectedStatus("llm http", status, body)
	}
	var output pagecontrol.Response
	if err := json.Unmarshal(body, &output); err != nil {
		return nil, fmt.Errorf("解析 llm http 控件检测响应失败: %w", err)
	}
	return pagecontrol.ParseCandidates(output), nil
}

func detectPageControlsWithLoggedOpenAIResponse(
	t *testing.T,
	baseURL string,
	apiKey string,
	model string,
	ctx enginestate.TraversalContext,
	fileName string,
) ([]perception.Candidate, error) {
	t.Helper()
	prompt := pagecontrol.BuildPrompt(ctx)
	payload := map[string]any{
		"model":       strings.TrimSpace(model),
		"temperature": 0,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的控件列表。",
			},
			{
				"role":    "user",
				"content": buildOpenAIUserContent(prompt),
			},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "trek_page_controls",
				"strict": true,
				"schema": prompt.ResponseSchema,
			},
		},
	}
	endpoint := normalizeOpenAIChatEndpointForTest(baseURL)
	body, status, err := postJSONWithBearerAuth(endpoint, apiKey, payload, nil)
	if err != nil {
		return nil, err
	}
	t.Logf("%s status=%d\n%s", fileName, status, prettyLLMJSON(body))
	if status < 200 || status >= 300 {
		return nil, errUnexpectedStatus("openai chat", status, body)
	}
	text, err := extractChatContentForTest(body)
	if err != nil {
		return nil, err
	}
	var output pagecontrol.Response
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 openai chat 控件检测输出失败: %w", err)
	}
	return pagecontrol.ParseCandidates(output), nil
}

func detectPageControlsWithLoggedAnthropicResponse(
	t *testing.T,
	baseURL string,
	apiKey string,
	model string,
	ctx enginestate.TraversalContext,
	fileName string,
) ([]perception.Candidate, error) {
	t.Helper()
	prompt := pagecontrol.BuildPrompt(ctx)
	payload := map[string]any{
		"model":       strings.TrimSpace(model),
		"max_tokens":  2048,
		"temperature": 0,
		"system":      prompt.SystemContent + "\n\n必须输出 JSON，且仅返回符合 schema 的控件列表。",
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": buildAnthropicUserContent(prompt),
			},
		},
	}
	endpoint := normalizeAnthropicEndpointForTest(baseURL)
	headers := map[string]string{"api-key": apiKey}
	body, status, err := postJSONWithBearerAuth(endpoint, apiKey, payload, headers)
	if err != nil {
		return nil, err
	}
	t.Logf("%s status=%d\n%s", fileName, status, prettyLLMJSON(body))
	if status < 200 || status >= 300 {
		return nil, errUnexpectedStatus("anthropic messages", status, body)
	}
	text, err := extractAnthropicTextForTest(body)
	if err != nil {
		return nil, err
	}
	var output pagecontrol.Response
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return nil, fmt.Errorf("解析 anthropic 控件检测输出失败: %w", err)
	}
	return pagecontrol.ParseCandidates(output), nil
}

func postJSONWithBearerAuth(endpoint string, apiKey string, payload any, extraHeaders map[string]string) ([]byte, int, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(endpoint), bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	for key, value := range extraHeaders {
		if strings.TrimSpace(key) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func buildOpenAIUserContent(prompt pagecontrol.Prompt) []map[string]any {
	userContent := []map[string]any{
		{"type": "text", "text": prompt.UserContent},
	}
	if len(prompt.Screenshot) > 0 {
		userContent = append(userContent, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url":    "data:" + prompt.ScreenshotMediaType + ";base64," + prompt.ScreenshotBase64(),
				"detail": "high",
			},
		})
	}
	return userContent
}

func buildAnthropicUserContent(prompt pagecontrol.Prompt) []map[string]any {
	content := []map[string]any{
		{"type": "text", "text": prompt.UserContent},
	}
	if len(prompt.Screenshot) > 0 {
		content = append(content, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": prompt.ScreenshotMediaType,
				"data":       prompt.ScreenshotBase64(),
			},
		})
	}
	return content
}

func normalizeOpenAIChatEndpointForTest(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "https://api.openai.com/v1/chat/completions"
	}
	switch {
	case strings.HasSuffix(trimmed, "/chat/completions"):
		return trimmed
	case strings.HasSuffix(trimmed, "/responses"):
		return strings.TrimSuffix(trimmed, "/responses") + "/chat/completions"
	case strings.HasSuffix(trimmed, "/v1"):
		return trimmed + "/chat/completions"
	default:
		return strings.TrimRight(trimmed, "/") + "/chat/completions"
	}
}

func normalizeAnthropicEndpointForTest(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return "https://api.anthropic.com/v1/messages"
	}
	switch {
	case strings.HasSuffix(trimmed, "/v1/messages"):
		return trimmed
	case strings.HasSuffix(trimmed, "/anthropic"):
		return strings.TrimRight(trimmed, "/") + "/v1/messages"
	default:
		return strings.TrimRight(trimmed, "/") + "/messages"
	}
}

func extractChatContentForTest(body []byte) (string, error) {
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

func extractAnthropicTextForTest(body []byte) (string, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析 anthropic 响应失败: %w", err)
	}
	for _, item := range response.Content {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			return strings.TrimSpace(item.Text), nil
		}
	}
	return "", fmt.Errorf("anthropic 响应中缺少 text content")
}

func errUnexpectedStatus(provider string, status int, body []byte) error {
	return &unexpectedStatusError{
		provider: provider,
		status:   status,
		body:     truncateForError(body, 512),
	}
}

type unexpectedStatusError struct {
	provider string
	status   int
	body     string
}

func (e *unexpectedStatusError) Error() string {
	return e.provider + " 请求失败: status=" + strconv.Itoa(e.status) + " body=" + e.body
}

func truncateForError(body []byte, max int) string {
	text := string(body)
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}

func prettyLLMJSON(body []byte) string {
	formatted := body
	var pretty bytes.Buffer
	if json.Valid(body) && json.Indent(&pretty, body, "", "  ") == nil {
		formatted = pretty.Bytes()
	}
	return string(formatted)
}

func assertLLMCandidatesHitExpectedControls(
	t *testing.T,
	items []perception.Candidate,
	expected []llmFixtureExpectedControl,
	textExtractor func(perception.Candidate) string,
) {
	t.Helper()
	const marginX = 0.03
	const marginY = 0.04

	for _, control := range expected {
		item, text, ok := findLLMCandidateByAliases(items, control.aliases, textExtractor)
		require.Truef(t, ok, "未找到控件 %q，对应候选如下:\n%s", control.name, formatLLMCandidates(items, textExtractor))
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

func findLLMCandidateByAliases(
	items []perception.Candidate,
	aliases []string,
	textExtractor func(perception.Candidate) string,
) (perception.Candidate, string, bool) {
	for _, item := range items {
		text := normalizeLLMFixtureText(textExtractor(item))
		if text == "" {
			continue
		}
		for _, alias := range aliases {
			normalizedAlias := normalizeLLMFixtureText(alias)
			if normalizedAlias != "" && text == normalizedAlias {
				return item, text, true
			}
		}
	}
	for _, item := range items {
		text := normalizeLLMFixtureText(textExtractor(item))
		if text == "" {
			continue
		}
		for _, alias := range aliases {
			normalizedAlias := normalizeLLMFixtureText(alias)
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

func normalizeLLMFixtureText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func formatLLMCandidates(items []perception.Candidate, textExtractor func(perception.Candidate) string) string {
	if len(items) == 0 {
		return "(空)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		label := normalizeLLMFixtureText(textExtractor(item))
		if item.Command == nil {
			lines = append(lines, "- text="+label+" command=nil")
			continue
		}
		lines = append(lines,
			"- text="+label+
				" bounds=["+
				formatLLMFloat(item.Command.Pos.Left)+","+
				formatLLMFloat(item.Command.Pos.Top)+","+
				formatLLMFloat(item.Command.Pos.Right)+","+
				formatLLMFloat(item.Command.Pos.Bottom)+"]")
	}
	return strings.Join(lines, "\n")
}

func formatLLMFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 4, 64), "0"), ".")
}
