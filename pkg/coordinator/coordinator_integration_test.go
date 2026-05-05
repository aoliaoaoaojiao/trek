//go:build integration

package coordinator

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	"trek/internal/testutil"
	"trek/pkg/driver/android"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionIntegration_NextActionWithRealXML(t *testing.T) {
	driver := testutil.RequireDevice(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)
	xml, _ := testutil.CapturePageSnapshot(t, driver)

	session, err := NewSession(Config{
		PackageName: pkgName,
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	require.NoError(t, err, "创建 Session 失败")
	defer closeSessionMemoryStore(session)

	action, err := session.NextAction("IntegrationTestPage", xml)
	require.NoError(t, err, "NextAction 调用失败")
	require.NotNil(t, action, "返回动作不应为 nil")
	assert.True(t, action.IsValid(), "返回动作应有效")
	t.Logf("NextAction 返回: act=%s pos=[%v,%v,%v,%v]",
		action.Act,
		action.Pos.Left, action.Pos.Top,
		action.Pos.Right, action.Pos.Bottom)
}

func TestSessionIntegration_WithOCRProvider(t *testing.T) {
	driver := testutil.RequireDevice(t)
	endpoint, apiKey := testutil.RequireOCREnv(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)
	xml, _ := testutil.CapturePageSnapshot(t, driver)

	session, err := NewSession(Config{
		PackageName:        pkgName,
		Algorithm:          decision.AlgorithmReuse,
		DeviceType:         types.Phone,
		ExploreOCREndpoint: endpoint,
		ExploreOCRAPIKey:   apiKey,
	})
	require.NoError(t, err, "创建 Session 失败")
	defer closeSessionMemoryStore(session)

	// 截图用于 OCR 增强
	screenshot := captureDeviceScreenshot(t, driver)
	action, err := session.NextActionWithInput("IntegrationTestPage", ActionInput{
		XMLDescOfGuiTree: xml,
		Screenshot:       screenshot,
	})
	require.NoError(t, err, "NextActionWithInput 调用失败")
	require.NotNil(t, action, "返回动作不应为 nil")
	t.Logf("OCR 增强后 NextAction: act=%s pos=[%v,%v,%v,%v]",
		action.Act,
		action.Pos.Left, action.Pos.Top,
		action.Pos.Right, action.Pos.Bottom)
}

func TestSessionIntegration_WithLLMPageControl(t *testing.T) {
	driver := testutil.RequireDevice(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)
	xml, screenshot := testutil.CapturePageSnapshot(t, driver)

	// 尝试 LLM HTTP 网关
	endpoint := strings.TrimSpace(os.Getenv("LLM_API_URL"))
	apiKey := strings.TrimSpace(os.Getenv("LLM_API_KEY"))
	model := strings.TrimSpace(os.Getenv("LLM_MODEL"))

	// 尝试 OpenAI
	openAIModel := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	openAIKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	openAIBaseURL := strings.TrimSpace(os.Getenv("OPENAI_API_URL"))

	if endpoint == "" && openAIModel == "" {
		t.Skip("跳过 LLM 控件检测集成测试：未设置 LLM_API_URL 或 OPENAI_MODEL")
	}

	cfg := Config{
		PackageName:         pkgName,
		Algorithm:           decision.AlgorithmReuse,
		DeviceType:          types.Phone,
		PageControlStrategy: "llm",
	}
	if endpoint != "" {
		cfg.RecoveryLLMEndpoint = endpoint
		cfg.RecoveryLLMAPIKey = apiKey
		cfg.RecoveryLLMModel = model
	} else {
		cfg.RecoveryLLMOpenAIModel = openAIModel
		cfg.RecoveryLLMOpenAIAPIKey = openAIKey
		cfg.RecoveryLLMOpenAIBaseURL = openAIBaseURL
	}

	session, err := NewSession(cfg)
	require.NoError(t, err, "创建 Session 失败")
	defer closeSessionMemoryStore(session)

	action, err := session.NextActionWithInput("IntegrationTestPage", ActionInput{
		XMLDescOfGuiTree: xml,
		Screenshot:       screenshot,
	})
	require.NoError(t, err, "NextActionWithInput 调用失败")
	require.NotNil(t, action, "返回动作不应为 nil")
	t.Logf("LLM 控件检测后 NextAction: act=%s pos=[%v,%v,%v,%v]",
		action.Act,
		action.Pos.Left, action.Pos.Top,
		action.Pos.Right, action.Pos.Bottom)
}

func TestSessionIntegration_MultipleSteps(t *testing.T) {
	driver := testutil.RequireDevice(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)
	xml, _ := testutil.CapturePageSnapshot(t, driver)

	session, err := NewSession(Config{
		PackageName: pkgName,
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	require.NoError(t, err)
	defer closeSessionMemoryStore(session)

	for i := 0; i < 3; i++ {
		action, err := session.NextAction("IntegrationTestPage", xml)
		require.NoError(t, err, "第 %d 步 NextAction 失败", i)
		require.NotNil(t, action, "第 %d 步返回动作为 nil", i)
		t.Logf("步骤 %d: act=%s", i, action.Act)

		// 报告步骤结果，模拟成功执行
		err = session.OnStepResult(StepResultInput{
			Step:    i,
			Action:  action,
			Success: true,
			Before: PageSnapshot{
				PageName: "IntegrationTestPage",
				XML:      xml,
			},
		})
		assert.NoError(t, err, "第 %d 步 OnStepResult 失败", i)
	}
}

func TestSessionIntegration_ObserveTraversalOutcome(t *testing.T) {
	driver := testutil.RequireDevice(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)
	xml, _ := testutil.CapturePageSnapshot(t, driver)

	session, err := NewSession(Config{
		PackageName: pkgName,
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	require.NoError(t, err)
	defer closeSessionMemoryStore(session)

	action, err := session.NextAction("IntegrationTestPage", xml)
	require.NoError(t, err)
	require.NotNil(t, action)

	// 观察遍历结果
	err = session.ObserveTraversalOutcome(
		enginestate.TraversalContext{
			Step:          1,
			Mode:          enginestate.ModeExplore,
			PageName:      "IntegrationTestPage",
			PageSignature: "test_sig",
		},
		action,
		traversal.OutcomeNewState,
	)
	assert.NoError(t, err, "ObserveTraversalOutcome 调用失败")
}

func captureDeviceScreenshot(t *testing.T, driver *android.AndroidDriver) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := driver.Screenshot(ctx)
	if err != nil {
		t.Fatalf("获取截图失败: %v", err)
	}
	return data
}
