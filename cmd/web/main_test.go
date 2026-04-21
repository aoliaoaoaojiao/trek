package web

import (
	"strings"
	"testing"
)

func TestBuildConfigJS_Default(t *testing.T) {
	js, err := buildConfigJS(webConfigPayload{})
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
	cfg := webConfigPayload{
		PageSource: "poco",
		TouchMode:  "motion",
	}
	js, err := buildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS poco 默认引擎失败: %v", err)
	}
	if !strings.Contains(js, `engine: "UNITY_3D"`) {
		t.Fatalf("poco 默认引擎未写入: %s", js)
	}
}

func TestBuildConfigJS_WithPageNameStrategy(t *testing.T) {
	cfg := webConfigPayload{
		PageSource:       "poco",
		TouchMode:        "motion",
		PageNameStrategy: "structure_fingerprint",
	}
	js, err := buildConfigJS(cfg)
	if err != nil {
		t.Fatalf("buildConfigJS 页面名策略失败: %v", err)
	}
	if !strings.Contains(js, `page_name_strategy: "structure_fingerprint"`) {
		t.Fatalf("未输出 page_name_strategy: %s", js)
	}
}

func TestResolvePreviewPageNameUsesSelectedStrategy(t *testing.T) {
	cfg := webConfigPayload{
		PageSource:       "poco",
		PageNameStrategy: "structure_fingerprint",
	}
	pageName := resolvePreviewPageName(cfg, "poco", `<hierarchy><node widget="button"/></hierarchy>`, "com.unity3d.player")
	if strings.HasPrefix(pageName, "com.unity3d") {
		t.Fatalf("预览页面名不应绕过结构指纹策略返回 Activity: %s", pageName)
	}
	if !strings.HasPrefix(pageName, "XMLPage:") {
		t.Fatalf("预期结构指纹页面名，实际: %s", pageName)
	}
}

func TestBuildConfigJS_InvalidTouchMode(t *testing.T) {
	cfg := webConfigPayload{
		PageSource: "uia",
		TouchMode:  "invalid",
	}
	_, err := buildConfigJS(cfg)
	if err == nil {
		t.Fatalf("预期触控模式校验失败，但返回成功")
	}
}

func TestBuildConfigJS_WithEffectiveTouchArea(t *testing.T) {
	cfg := webConfigPayload{
		PageSource: "uia",
		TouchMode:  "motion",
	}
	cfg.EffectiveTouchArea.Serial = "192.168.2.198:5555"
	cfg.EffectiveTouchArea.PackageName = "com.NetEase"
	cfg.EffectiveTouchArea.Range.Left = 0.043
	cfg.EffectiveTouchArea.Range.Top = 0
	cfg.EffectiveTouchArea.Range.Right = 1
	cfg.EffectiveTouchArea.Range.Bottom = 1

	js, err := buildConfigJS(cfg)
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
