/*
Copyright © 2026 Trek
*/
package webserver

import (
	"strings"
	"testing"
)

func ptr[T any](v T) *T {
	return &v
}

func TestBuildConfigJS_Default(t *testing.T) {
	js, err := BuildConfigJS(ConfigPayload{})
	if err != nil {
		t.Fatalf("buildConfigJS 默认配置失败: %v", err)
	}
	if !strings.Contains(js, `page_source: "uia"`) {
		t.Fatalf("默认 page_source 不正确: %s", js)
	}
	if !strings.Contains(js, `touch_mode: "motion"`) {
		t.Fatalf("默认 touch_mode 不正确: %s", js)
	}
}

func TestBuildConfigJS_PocoDefaultEngine(t *testing.T) {
	cfg := ConfigPayload{
		PageSource: "poco",
		TouchMode:  "motion",
	}
	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS poco 默认引擎失败: %v", err)
	}
	if !strings.Contains(js, `engine: "UNITY_3D"`) {
		t.Fatalf("poco 默认引擎未写入: %s", js)
	}
}

func TestBuildConfigJS_WithPageNameStrategy(t *testing.T) {
	cfg := ConfigPayload{
		PageSource:       "poco",
		TouchMode:        "motion",
		PageNameStrategy: "structure_fingerprint",
	}
	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS 页面名策略失败: %v", err)
	}
	if !strings.Contains(js, `page_name_strategy: "structure_fingerprint"`) {
		t.Fatalf("未输出 page_name_strategy: %s", js)
	}
}

func TestResolvePreviewPageNameUsesSelectedStrategy(t *testing.T) {
	cfg := ConfigPayload{
		PageSource:       "poco",
		PageNameStrategy: "structure_fingerprint",
	}
	pageName := resolvePreviewPageName(cfg, "poco", `<hierarchy><node widget="button"/></hierarchy>`, nil, "com.unity3d.player")
	if strings.HasPrefix(pageName, "com.unity3d") {
		t.Fatalf("预览页面名不应绕过结构指纹策略返回 Activity: %s", pageName)
	}
	if !strings.HasPrefix(pageName, "XMLPage:") {
		t.Fatalf("预期结构指纹页面名，实际: %s", pageName)
	}
}

func TestBuildConfigJS_ScreenshotPageSourceForcesCaptureScreenshot(t *testing.T) {
	cfg := ConfigPayload{
		PageSource: "screenshot",
		TouchMode:  "motion",
	}
	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS screenshot 页面源失败: %v", err)
	}
	for _, expected := range []string{
		`page_source: "screenshot"`,
		`capture_screenshot: true`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("未输出字段 %q: %s", expected, js)
		}
	}
}

func TestBuildConfigJS_ScreenshotPageSourceForcesOCRPageControl(t *testing.T) {
	cfg := ConfigPayload{
		PageSource: "screenshot",
		TouchMode:  "motion",
	}
	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS screenshot 页面源失败: %v", err)
	}
	if !strings.Contains(js, `page_control_strategy: "ocr"`) {
		t.Fatalf("screenshot 页面源应默认导出 ocr 页面理解策略: %s", js)
	}
}

func TestBuildConfigJS_InvalidTouchMode(t *testing.T) {
	cfg := ConfigPayload{
		PageSource: "uia",
		TouchMode:  "invalid",
	}
	_, err := BuildConfigJS(cfg)
	if err == nil {
		t.Fatalf("预期触控模式校验失败，但返回成功")
	}
}

func TestBuildConfigJS_WithEffectiveTouchArea(t *testing.T) {
	cfg := ConfigPayload{
		PageSource: "uia",
		TouchMode:  "motion",
	}
	cfg.EffectiveTouchArea.Serial = "192.168.2.198:5555"
	cfg.EffectiveTouchArea.PackageName = "com.NetEase"
	cfg.EffectiveTouchArea.Range.Left = 0.043
	cfg.EffectiveTouchArea.Range.Top = 0
	cfg.EffectiveTouchArea.Range.Right = 1
	cfg.EffectiveTouchArea.Range.Bottom = 1

	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS effective_touch_area 失败: %v", err)
	}
	if !strings.Contains(js, `effective_touch_area`) {
		t.Fatalf("未输出 effective_touch_area: %s", js)
	}
	if !strings.Contains(js, `serial: "192.168.2.198:5555"`) {
		t.Fatalf("未输出 serial: %s", js)
	}
	if !strings.Contains(js, `package_name: "com.NetEase"`) {
		t.Fatalf("未输出 package_name: %s", js)
	}
	if !strings.Contains(js, `left: 0.043`) {
		t.Fatalf("未输出 left: %s", js)
	}
}

func TestBuildConfigJS_WithAdvancedFields(t *testing.T) {
	cfg := ConfigPayload{
		PageSource:                   "uia",
		TouchMode:                    "motion",
		PageControl:                  "ocr",
		Algorithm:                    "uctbandit",
		CaptureScreenshot:            ptr(true),
		KeepStepRecords:              ptr(false),
		ScrollInferThreshold:         ptr(5),
		ImageSimilaritySSIMThreshold: ptr(0.985),
		RecoveryCooldownSteps:        ptr(2),
		LLMTimeoutMs:                 ptr(15000),
	}
	cfg.UCTBandit.ActionCooldownPenalty = ptr(0.75)
	cfg.UCTBandit.RecentActionWindow = ptr(8)

	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS 高级字段失败: %v", err)
	}
	for _, expected := range []string{
		`page_control_strategy: "ocr"`,
		`algorithm: "uctbandit"`,
		`capture_screenshot: true`,
		`keep_step_records: false`,
		`scroll_infer_threshold: 5`,
		`image_similarity_ssim_threshold: 0.985`,
		`recovery_cooldown_steps: 2`,
		`llm_timeout_ms: 15000`,
		`uct_bandit: {`,
		`action_cooldown_penalty: 0.75`,
		`recent_action_window: 8`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("未输出字段 %q: %s", expected, js)
		}
	}
}

func TestBuildConfigJS_WithReuseFields(t *testing.T) {
	cfg := ConfigPayload{
		PageSource: "uia",
		TouchMode:  "motion",
		Algorithm:  "reuse",
	}
	cfg.Reuse.Epsilon = ptr(0.08)
	cfg.Reuse.Gamma = ptr(0.9)
	cfg.Reuse.NStep = ptr(7)
	cfg.Reuse.ModelSavePath = "./data/demo_reuse.model"
	cfg.Reuse.EnableModelPersistence = ptr(true)
	cfg.Reuse.ResetModelOnStart = ptr(false)

	js, err := BuildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS reuse 字段失败: %v", err)
	}
	for _, expected := range []string{
		`algorithm: "reuse"`,
		`reuse: {`,
		`epsilon: 0.08`,
		`gamma: 0.9`,
		`n_step: 7`,
		`model_save_path: "./data/demo_reuse.model"`,
		`enable_model_persistence: true`,
		`reset_model_on_start: false`,
	} {
		if !strings.Contains(js, expected) {
			t.Fatalf("未输出字段 %q: %s", expected, js)
		}
	}
}
