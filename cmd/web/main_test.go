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
