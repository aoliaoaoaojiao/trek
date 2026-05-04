package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// FixtureGameNavigation 是导航页示例截图，供 OCR / LLM / gocv 相关测试复用。
	FixtureGameNavigation = "game_navigation.png"
)

// ReadRootFixture 读取仓库根目录 testdata 下的测试资源。
func ReadRootFixture(t *testing.T, fixtureName string) []byte {
	t.Helper()
	path := RootFixturePath(t, fixtureName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取测试资源失败: fixture=%s path=%s err=%v", fixtureName, path, err)
	}
	if len(data) == 0 {
		t.Fatalf("测试资源不能为空: fixture=%s path=%s", fixtureName, path)
	}
	return data
}

// RootFixturePath 返回仓库根目录 testdata 下的测试资源绝对路径。
func RootFixturePath(t *testing.T, fixtureName string) string {
	t.Helper()
	fixtureName = strings.TrimSpace(fixtureName)
	if fixtureName == "" {
		t.Fatal("fixture 名称不能为空")
	}
	rootDir := requireProjectRoot(t)
	return filepath.Join(rootDir, "testdata", fixtureName)
}

func requireProjectRoot(t *testing.T) string {
	t.Helper()
	startDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前工作目录失败: %v", err)
	}
	rootDir, err := findProjectRootForFixture(startDir)
	if err != nil {
		t.Fatalf("定位项目根目录失败: %v", err)
	}
	return rootDir
}

func findProjectRootForFixture(startDir string) (string, error) {
	current := filepath.Clean(startDir)
	for {
		if info, err := os.Stat(filepath.Join(current, "go.mod")); err == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("从 %s 向上未找到 go.mod", startDir)
		}
		current = parent
	}
}
