package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResourceMappingBlackRects(t *testing.T) {
	m := GetInstance()
	m.blackRects = map[string][][4]int{}

	p := filepath.Join(t.TempDir(), "mix.json")
	content := `{"black_rects":{"LoginActivity":[[0,0,100,100],[200,200,300,300]]}}`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	if err := m.LoadResourceMapping(p); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if !m.CheckPointIsInBlackRects("LoginActivity", 50, 50) {
		t.Fatalf("点位应命中 black_rects")
	}
	if m.CheckPointIsInBlackRects("LoginActivity", 150, 150) {
		t.Fatalf("点位不应命中 black_rects")
	}
}

func TestLoadResourceMappingRejectInvalidRect(t *testing.T) {
	m := GetInstance()
	m.blackRects = map[string][][4]int{}

	p := filepath.Join(t.TempDir(), "mix.json")
	content := `{"black_rects":{"LoginActivity":[[0,0,100]]}}`
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	if err := m.LoadResourceMapping(p); err == nil {
		t.Fatalf("非法矩形应返回错误")
	}
}
