package providers

import (
	"strings"
	"testing"

	enginestate "trek/internal/engine/state"
)

func TestBuildRecoveryPrompt_SystemContent(t *testing.T) {
	ctx := enginestate.TraversalContext{
		Step:     10,
		Mode:     "Recover",
		PageName: "MainActivity",
	}
	prompt := buildRecoveryPrompt(ctx)

	if !strings.Contains(prompt.SystemContent, "Android UI") {
		t.Fatalf("SystemContent 应包含 'Android UI': %s", prompt.SystemContent)
	}
	if !strings.Contains(prompt.SystemContent, "归一化坐标") {
		t.Fatalf("SystemContent 应包含坐标说明: %s", prompt.SystemContent)
	}
	if !strings.Contains(prompt.SystemContent, "JSON") {
		t.Fatalf("SystemContent 应提及 JSON 输出: %s", prompt.SystemContent)
	}
}

func TestBuildRecoveryPrompt_UserContent_Structured(t *testing.T) {
	ctx := enginestate.TraversalContext{
		Step:        42,
		Mode:        "Recover",
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{PageSignature: "sig_home", ActionKey: "click_btn_login"},
			{PageSignature: "sig_login", ActionKey: "scroll_down"},
		},
		VisitStats: enginestate.VisitStats{
			PageVisitCount: map[string]int{"MainActivity": 5, "LoginPage": 2},
			ActionCount:    map[string]int{"click_btn_login": 3, "scroll_down": 1},
		},
		LocalCandidates: []enginestate.CandidateSummary{
			{ActionType: "BACK", Source: "memory", Confidence: 0.9, EscapeScore: 0.8, RiskScore: 0.1, Intent: "返回上一层"},
		},
		KnownFailedActions:  []string{`{"act":"CLICK","x":0.5}`},
		KnownSuccessActions: []string{`{"act":"BACK"}`},
	}
	prompt := buildRecoveryPrompt(ctx)

	if !strings.Contains(prompt.UserContent, "当前步数: 42") {
		t.Fatalf("UserContent 应包含格式化的步数: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "遍历模式: Recover") {
		t.Fatalf("UserContent 应包含模式: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "当前页面: MainActivity") {
		t.Fatalf("UserContent 应包含页面名: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "阻塞原因: same_page_no_change") {
		t.Fatalf("UserContent 应包含阻塞原因: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "页面访问次数") {
		t.Fatalf("UserContent 应包含页面访问次数: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "动作执行次数") {
		t.Fatalf("UserContent 应包含动作执行次数: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "最近操作轨迹") {
		t.Fatalf("UserContent 应包含操作轨迹: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "本地候选摘要") {
		t.Fatalf("UserContent 应包含本地候选摘要: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "已知失败动作") {
		t.Fatalf("UserContent 应包含已知失败动作: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "已知成功动作") {
		t.Fatalf("UserContent 应包含已知成功动作: %s", prompt.UserContent)
	}
}

func TestBuildRecoveryPrompt_UserContent_XMLSnippet(t *testing.T) {
	xml := `<node bounds="[0,0][1080,2400]"><node text="Login" bounds="[100,200][300,400]"/></node>`
	ctx := enginestate.TraversalContext{
		Step: 1,
		Mode: "Recover",
		XML:  xml,
	}
	prompt := buildRecoveryPrompt(ctx)

	if !strings.Contains(prompt.UserContent, "页面结构（XML）") {
		t.Fatalf("UserContent 应包含 XML 部分: %s", prompt.UserContent)
	}
	if !strings.Contains(prompt.UserContent, "Login") {
		t.Fatalf("UserContent 应包含 XML 内容: %s", prompt.UserContent)
	}
}

func TestBuildRecoveryPrompt_XMLTruncation(t *testing.T) {
	longXML := strings.Repeat("<node>", 1000)
	ctx := enginestate.TraversalContext{
		Step: 1,
		Mode: "Recover",
		XML:  longXML,
	}
	prompt := buildRecoveryPrompt(ctx)

	if !strings.Contains(prompt.UserContent, "截断") {
		t.Fatalf("长 XML 应被截断: %s", prompt.UserContent[len(prompt.UserContent)-50:])
	}
}

func TestBuildRecoveryPrompt_Screenshot(t *testing.T) {
	screenshotData := []byte("fake-png-data")
	ctx := enginestate.TraversalContext{
		Step:       1,
		Mode:       "Recover",
		Screenshot: screenshotData,
	}
	prompt := buildRecoveryPrompt(ctx)

	if len(prompt.Screenshot) != len(screenshotData) {
		t.Fatalf("Screenshot 长度应一致: 期望 %d, 实际 %d", len(screenshotData), len(prompt.Screenshot))
	}
	if prompt.ScreenshotMediaType != "image/png" {
		t.Fatalf("ScreenshotMediaType 应为 image/png, 实际: %s", prompt.ScreenshotMediaType)
	}

	// 验证 base64
	b64 := prompt.ScreenshotBase64()
	if b64 == "" {
		t.Fatalf("ScreenshotBase64 不应为空")
	}
}

func TestBuildRecoveryPrompt_NoScreenshot(t *testing.T) {
	ctx := enginestate.TraversalContext{
		Step: 1,
		Mode: "Recover",
	}
	prompt := buildRecoveryPrompt(ctx)

	if prompt.Screenshot != nil {
		t.Fatalf("无截图时 Screenshot 应为 nil")
	}
	if prompt.ScreenshotMediaType != "" {
		t.Fatalf("无截图时 ScreenshotMediaType 应为空, 实际: %s", prompt.ScreenshotMediaType)
	}
	if prompt.ScreenshotBase64() != "" {
		t.Fatalf("无截图时 ScreenshotBase64 应为空")
	}
}

func TestBuildRecoveryPrompt_ScreenshotIsolation(t *testing.T) {
	original := []byte{0x89, 0x50, 0x4E, 0x47}
	ctx := enginestate.TraversalContext{
		Step:       1,
		Mode:       "Recover",
		Screenshot: original,
	}
	prompt := buildRecoveryPrompt(ctx)

	// 修改原始数据不应影响 prompt
	original[0] = 0x00

	if prompt.Screenshot[0] != 0x89 {
		t.Fatalf("Screenshot 应深拷贝，修改原文不应影响")
	}
}

func TestBuildRecoveryPrompt_ContextFieldsPreserved(t *testing.T) {
	ctx := enginestate.TraversalContext{
		Step:             42,
		Mode:             "Recover",
		PageName:         "MainActivity",
		PageSignature:    "sig_abc",
		ClusterSignature: "cluster_xyz",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{PageSignature: "sig_1", ActionKey: "click_btn"},
		},
		VisitStats: enginestate.VisitStats{
			PageVisitCount: map[string]int{"MainActivity": 5},
			ActionCount:    map[string]int{"click_btn": 3},
		},
		LocalCandidates: []enginestate.CandidateSummary{
			{ActionType: "BACK", Source: "memory", Confidence: 0.8},
		},
		KnownFailedActions:  []string{"A", "B"},
		KnownSuccessActions: []string{"C"},
	}
	prompt := buildRecoveryPrompt(ctx)

	if prompt.ContextFields.Step != 42 {
		t.Fatalf("Step 应为 42, 实际: %d", prompt.ContextFields.Step)
	}
	if prompt.ContextFields.Mode != "Recover" {
		t.Fatalf("Mode 应为 Recover, 实际: %s", prompt.ContextFields.Mode)
	}
	if prompt.ContextFields.ClusterSignature != "cluster_xyz" {
		t.Fatalf("ClusterSignature 应为 cluster_xyz, 实际: %s", prompt.ContextFields.ClusterSignature)
	}
	if len(prompt.ContextFields.LocalCandidates) != 1 {
		t.Fatalf("LocalCandidates 应保留，实际: %d", len(prompt.ContextFields.LocalCandidates))
	}
	if len(prompt.ContextFields.KnownFailedActions) != 2 || len(prompt.ContextFields.KnownSuccessActions) != 1 {
		t.Fatalf("Known actions 应保留，实际 failed=%v success=%v", prompt.ContextFields.KnownFailedActions, prompt.ContextFields.KnownSuccessActions)
	}
}

func TestBuildRecoveryPrompt_ResponseSchema_PointCoordinates(t *testing.T) {
	prompt := buildRecoveryPrompt(enginestate.TraversalContext{})
	schema := prompt.ResponseSchema

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema.properties 类型错误")
	}
	candidates, ok := props["candidates"].(map[string]any)
	if !ok {
		t.Fatalf("schema.properties.candidates 类型错误")
	}
	items, ok := candidates["items"].(map[string]any)
	if !ok {
		t.Fatalf("candidates.items 类型错误")
	}
	itemProps, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("items.properties 类型错误")
	}

	// 验证 point 字段替代了 bounds
	point, ok := itemProps["point"].(map[string]any)
	if !ok {
		t.Fatalf("应存在 point 字段, 实际: %v", itemProps)
	}
	if point["type"] != "object" {
		t.Fatalf("point.type 应为 object, 实际: %v", point["type"])
	}
	pointProps, ok := point["properties"].(map[string]any)
	if !ok {
		t.Fatalf("point.properties 类型错误")
	}
	x, ok := pointProps["x"].(map[string]any)
	if !ok {
		t.Fatalf("point.properties.x 类型错误")
	}
	if x["type"] != "number" {
		t.Fatalf("point.x.type 应为 number, 实际: %v", x["type"])
	}
	y, ok := pointProps["y"].(map[string]any)
	if !ok {
		t.Fatalf("point.properties.y 类型错误")
	}
	if y["type"] != "number" {
		t.Fatalf("point.y.type 应为 number, 实际: %v", y["type"])
	}

	// 验证 bounds 字段已移除
	if _, exists := itemProps["bounds"]; exists {
		t.Fatalf("bounds 字段应已移除")
	}
}

func TestBuildRecoveryPrompt_RecentTraceFormatting(t *testing.T) {
	ctx := enginestate.TraversalContext{
		Step: 5,
		Mode: "Recover",
		RecentTrace: []enginestate.ActionTrace{
			{PageSignature: "page_A", ActionKey: "click_ok"},
			{PageSignature: "page_B", ActionKey: "scroll_down"},
			{PageSignature: "page_C", ActionKey: "back"},
		},
	}
	prompt := buildRecoveryPrompt(ctx)

	// 轨迹应倒序显示（最近在前）
	lines := strings.Split(prompt.UserContent, "\n")
	foundClickOK := false
	foundBack := false
	for _, line := range lines {
		if strings.Contains(line, "page_C") && strings.Contains(line, "back") {
			foundBack = true
		}
		if strings.Contains(line, "page_A") && strings.Contains(line, "click_ok") {
			foundClickOK = true
		}
	}
	if !foundClickOK {
		t.Fatalf("应包含最早轨迹 page_A/click_ok: %s", prompt.UserContent)
	}
	if !foundBack {
		t.Fatalf("应包含最近轨迹 page_C/back: %s", prompt.UserContent)
	}
}

func TestBuildRecoveryPrompt_EmptyContext(t *testing.T) {
	ctx := enginestate.TraversalContext{}
	prompt := buildRecoveryPrompt(ctx)

	if prompt.ContextFields.Step != 0 {
		t.Fatalf("空 ctx 的 Step 应为 0, 实际: %d", prompt.ContextFields.Step)
	}
	if prompt.ContextFields.Mode != "" {
		t.Fatalf("空 ctx 的 Mode 应为空, 实际: %s", prompt.ContextFields.Mode)
	}
	if prompt.ContextFields.RecentTrace != nil {
		t.Fatalf("空 ctx 的 RecentTrace 应为 nil, 实际: %v", prompt.ContextFields.RecentTrace)
	}
	if prompt.ContextFields.PageVisitCount != nil {
		t.Fatalf("空 ctx 的 PageVisitCount 应为 nil, 实际: %v", prompt.ContextFields.PageVisitCount)
	}
	if prompt.ContextFields.KnownFailedActions != nil || prompt.ContextFields.KnownSuccessActions != nil {
		t.Fatalf("空 ctx 的 known actions 应为 nil")
	}
}
