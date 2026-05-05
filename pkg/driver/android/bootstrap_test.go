package android

import "testing"

func TestResolvePageSourceType(t *testing.T) {
	got, err := ResolvePageSourceType("")
	if err != nil {
		t.Fatalf("默认页面源解析失败: %v", err)
	}
	if got != "uia" {
		t.Fatalf("默认页面源应为 uia，实际: %s", got)
	}
}

func TestResolveTouchMode(t *testing.T) {
	mode, touchType, err := ResolveTouchMode("")
	if err != nil {
		t.Fatalf("默认触控模式解析失败: %v", err)
	}
	if mode != "motion" || touchType != TouchTypeMotion {
		t.Fatalf("默认触控模式错误: mode=%s type=%s", mode, touchType)
	}
}

func TestBuildDriverOptionsRejectsMissingPocoEngine(t *testing.T) {
	_, err := BuildDriverOptions(DriverBootstrapConfig{}, "poco", TouchTypeMotion)
	if err == nil {
		t.Fatalf("缺少 poco engine 时应返回错误")
	}
}

func TestParsePocoEngineAlias(t *testing.T) {
	engine, err := ParsePocoEngine("unity")
	if err != nil {
		t.Fatalf("解析 poco engine 失败: %v", err)
	}
	if engine == "" {
		t.Fatalf("解析后的 poco engine 不应为空")
	}
}
