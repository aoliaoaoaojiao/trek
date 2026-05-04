package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func resetLoggerForTest() {
	if logger != nil {
		_ = logger.Sync()
	}
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
	logger = nil
	sugar = nil
	once = sync.Once{}
	logLevel = zapcore.InfoLevel
	fileLogLevel = zapcore.DebugLevel
	consoleAtomicLevel = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	fileAtomicLevel = zap.NewAtomicLevelAt(zapcore.DebugLevel)
}

func TestFileLogLevelCanKeepDebugWhileConsoleUsesInfo(t *testing.T) {
	resetLoggerForTest()
	logDir := t.TempDir()
	t.Cleanup(resetLoggerForTest)

	oldStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建 stdout 管道失败: %v", err)
	}
	os.Stdout = writePipe
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = readPipe.Close()
	})

	if err := SetLevel("info"); err != nil {
		t.Fatalf("设置控制台日志级别失败: %v", err)
	}
	if err := SetFileLevel("debug"); err != nil {
		t.Fatalf("设置文件日志级别失败: %v", err)
	}
	if err := InitLogger(logDir); err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	Debug("debug-file-only")
	Info("info-visible")
	if err := Sync(); err != nil {
		t.Fatalf("同步日志失败: %v", err)
	}
	_ = writePipe.Close()
	consoleData, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("读取控制台输出失败: %v", err)
	}
	if strings.Contains(string(consoleData), "debug-file-only") {
		t.Fatalf("控制台日志不应包含 debug 内容: %s", string(consoleData))
	}
	if !strings.Contains(string(consoleData), "info-visible") {
		t.Fatalf("控制台日志应包含 info 内容: %s", string(consoleData))
	}
	os.Stdout = oldStdout

	matches, err := filepath.Glob(filepath.Join(logDir, "app_*.log"))
	if err != nil {
		t.Fatalf("匹配日志文件失败: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("日志文件数量不符合预期: %d, files=%v", len(matches), matches)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "debug-file-only") {
		t.Fatalf("文件日志应包含 debug 内容: %s", text)
	}
	if !strings.Contains(text, "info-visible") {
		t.Fatalf("文件日志应包含 info 内容: %s", text)
	}
}

func TestInitLoggerWithPackageUsesPackageAsFilePrefix(t *testing.T) {
	resetLoggerForTest()
	logDir := t.TempDir()
	t.Cleanup(resetLoggerForTest)

	if err := InitLoggerWithPackage(logDir, "com.demo.app"); err != nil {
		t.Fatalf("初始化包级日志失败: %v", err)
	}
	Info("package-log")
	_ = Sync()

	matches, err := filepath.Glob(filepath.Join(logDir, "com.demo.app_*.log"))
	if err != nil {
		t.Fatalf("匹配包级日志文件失败: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("包级日志文件数量不符合预期: %d, files=%v", len(matches), matches)
	}
	if !strings.Contains(filepath.Base(matches[0]), time.Now().Format("2006-01-02_")) {
		t.Fatalf("包级日志文件名缺少日期前缀: %s", filepath.Base(matches[0]))
	}
}

func TestResolveLogDirUsesProjectRootForRelativePath(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前目录失败: %v", err)
	}
	expectedRoot := findRootForTest(t, workingDir)

	got, err := resolveLogDir("log")
	if err != nil {
		t.Fatalf("解析相对日志目录失败: %v", err)
	}
	expected := filepath.Join(expectedRoot, "log")
	if got != expected {
		t.Fatalf("相对日志目录应解析到项目根目录: got=%s want=%s", got, expected)
	}
}

func findRootForTest(t *testing.T, start string) string {
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
