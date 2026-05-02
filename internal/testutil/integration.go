// Package testutil 提供集成测试共享的守卫函数与工具。
package testutil

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
	"trek/pkg/driver/android"
	"trek/pkg/driver/common"
)

func init() {
	rootPath, _ := common.RepoRootFromCurrentFile()
	common.SetPluginDirPath(rootPath)
}

// RequireDevice 检测 ADB 设备是否可用，不可用时跳过测试。
// 返回已连接的 AndroidDriver。
func RequireDevice(t *testing.T) *android.AndroidDriver {
	t.Helper()
	driver, err := android.NewAndroidDriver()
	if err != nil {
		t.Skipf("跳过集成测试：无法连接 ADB 设备: %v", err)
	}
	t.Cleanup(func() { driver.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pkg, err := driver.GetCurrentPackage(ctx)
	if err != nil {
		t.Skipf("跳过集成测试：无法获取设备信息: %v", err)
	}
	t.Logf("已连接设备，当前前台应用: %s", pkg)
	return driver
}

// RequireOCREnv 检查 OCR 环境变量是否配置，未配置时跳过测试。
func RequireOCREnv(t *testing.T) (endpoint, apiKey string) {
	t.Helper()
	endpoint = strings.TrimSpace(os.Getenv("PADDLEOCR_API_URL"))
	apiKey = strings.TrimSpace(os.Getenv("PADDLEOCR_API_KEY"))
	if endpoint == "" {
		t.Skip("跳过 OCR 集成测试：未设置 PADDLEOCR_API_URL")
	}
	return endpoint, apiKey
}

// RequireLLMEnv 检查 LLM HTTP 网关环境变量是否配置，未配置时跳过测试。
func RequireLLMEnv(t *testing.T) (endpoint, apiKey, model string) {
	t.Helper()
	endpoint = strings.TrimSpace(os.Getenv("LLM_API_URL"))
	apiKey = strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	model = strings.TrimSpace(os.Getenv("LLM_MODEL"))
	if endpoint == "" {
		t.Skip("跳过 LLM 集成测试：未设置 LLM_API_URL")
	}
	return endpoint, apiKey, model
}

// RequireOpenAIEnv 检查 OpenAI 环境变量是否配置，未配置时跳过测试。
func RequireOpenAIEnv(t *testing.T) (baseURL, apiKey, model string) {
	t.Helper()
	apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	model = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	baseURL = strings.TrimSpace(os.Getenv("OPENAI_API_URL"))
	if apiKey == "" || model == "" {
		t.Skip("跳过 OpenAI 集成测试：未设置 OPENAI_API_KEY 或 OPENAI_MODEL")
	}
	return baseURL, apiKey, model
}

// DetectForegroundPackage 自动检测设备前台应用包名。
// 优先使用 TREK_TEST_PACKAGE 环境变量，否则从设备获取。
func DetectForegroundPackage(t *testing.T, driver *android.AndroidDriver) string {
	t.Helper()
	if pkg := strings.TrimSpace(os.Getenv("TREK_TEST_PACKAGE")); pkg != "" {
		return pkg
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pkg, err := driver.GetCurrentPackage(ctx)
	if err != nil {
		t.Fatalf("获取前台应用包名失败: %v", err)
	}
	return pkg
}

// CapturePageSnapshot 从真实设备获取页面快照（XML + 截图）。
func CapturePageSnapshot(t *testing.T, driver *android.AndroidDriver) (xml string, screenshot []byte) {
	t.Helper()

	pageSource := driver.GetPageSource(string(android.PageTypeUIA))
	if pageSource == nil {
		t.Fatal("获取 PageSource 失败")
	}
	var err error
	xml, err = pageSource.DumpPageSource()
	if err != nil {
		t.Fatalf("获取页面 XML 失败: %v", err)
	}
	if xml == "" {
		t.Fatal("页面 XML 为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	screenshot, err = driver.Screenshot(ctx)
	if err != nil {
		t.Fatalf("获取截图失败: %v", err)
	}
	if len(screenshot) == 0 {
		t.Fatal("截图数据为空")
	}
	t.Logf("页面快照: XML=%d bytes, Screenshot=%d bytes", len(xml), len(screenshot))
	return xml, screenshot
}
