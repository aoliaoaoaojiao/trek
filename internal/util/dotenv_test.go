package util

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadDotEnvFilesLoadsLocalOverrides(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}
	projectRoot := findProjectRootForEnvTest(t, workingDir)

	localPath := filepath.Join(projectRoot, ".env.local")
	backupPath := localPath + ".codex-backup"
	restoreBackup := false

	if _, err := os.Stat(localPath); err == nil {
		if err := os.Rename(localPath, backupPath); err != nil {
			t.Fatalf("备份现有 .env.local 失败: %v", err)
		}
		restoreBackup = true
	}
	t.Cleanup(func() {
		_ = os.Remove(localPath)
		if restoreBackup {
			_ = os.Rename(backupPath, localPath)
		}
		loadDotEnvOnce = sync.Once{}
		_ = os.Unsetenv("PADDLEOCR_API_URL")
	})

	content := "PADDLEOCR_API_URL=http://fixture-ocr.local/test\n"
	if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
		t.Fatalf("写入测试 .env.local 失败: %v", err)
	}

	loadDotEnvOnce = sync.Once{}
	_ = os.Unsetenv("PADDLEOCR_API_URL")
	LoadDotEnvFiles()

	if got := os.Getenv("PADDLEOCR_API_URL"); got != "http://fixture-ocr.local/test" {
		t.Fatalf("未从 .env.local 加载 OCR URL: got=%q", got)
	}
}

func findProjectRootForEnvTest(t *testing.T, start string) string {
	t.Helper()
	current := filepath.Clean(start)
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			t.Fatalf("未找到 go.mod: start=%s", start)
		}
		current = parent
	}
}
