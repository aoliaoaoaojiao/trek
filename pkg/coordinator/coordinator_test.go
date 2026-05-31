package coordinator

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	"trek/internal/engine/memory"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
)

func mustPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("生成测试截图失败: %v", err)
	}
	return buf.Bytes()
}

func closeSessionMemoryStore(s *Coordinator) {
	if s == nil || s.memoryStore == nil {
		if s != nil && s.pageControlStore != nil {
			_ = s.pageControlStore.Close()
		}
		return
	}
	_ = s.memoryStore.Close()
	if s.pageControlStore != nil {
		_ = s.pageControlStore.Close()
	}
}

func TestCoordinatorOwnsRuntimeFacade(t *testing.T) {
	coord, err := New(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建 Coordinator 失败: %v", err)
	}
	if coord.runtime == nil {
		t.Fatalf("Coordinator 应持有 runtime 门面")
	}
}

type mockTraversalAlgorithm struct {
	proposeFn func(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
	selectFn  func(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error)
	observeFn func(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error
}

type mockCandidateProvider struct {
	buildFn func(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
}

func (m *mockCandidateProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if m != nil && m.buildFn != nil {
		return m.buildFn(ctx)
	}
	return nil, nil
}

type mockPageControlProvider struct {
	mockCandidateProvider
	detectFn func(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
}

func (m *mockPageControlProvider) DetectPageControls(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if m != nil && m.detectFn != nil {
		return m.detectFn(ctx)
	}
	return nil, nil
}

func (m *mockTraversalAlgorithm) Name() string { return "mock" }

func (m *mockTraversalAlgorithm) ProposeCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if m != nil && m.proposeFn != nil {
		return m.proposeFn(ctx)
	}
	return nil, nil
}

func (m *mockTraversalAlgorithm) SelectAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error) {
	if m != nil && m.selectFn != nil {
		return m.selectFn(ctx, candidates)
	}
	return nil, nil
}

func (m *mockTraversalAlgorithm) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
	if m != nil && m.observeFn != nil {
		return m.observeFn(ctx, action, outcome)
	}
	return nil
}

func TestSessionNextAction(t *testing.T) {
	session, err := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	action, err := session.NextAction("LoginActivity", `
<hierarchy>
	<node class="android.widget.FrameLayout" resource-id="" content-desc="" text="" clickable="false" long-clickable="false" checkable="false" enabled="true" bounds="[0,0][1080,1920]">
		<node class="android.widget.Button" resource-id="com.demo:id/login" content-desc="鐧诲綍" text="鐧诲綍" clickable="true" long-clickable="false" checkable="false" enabled="true" bounds="[10,20][110,120]"/>
	</node>
</hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇涓嬩竴姝ュ姩浣滃け锟? %v", err)
	}

	if action == nil {
		t.Fatalf("棰勬湡杩斿洖闈炵┖鍔ㄤ綔")
	}
}

func TestSessionCheckPointInBlackRects(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "mix.js")
	configContent := `const config = {
  excluded_touch_areas: [
    { page_name: "LoginActivity", bounds: [0, 0, 100, 100] }
  ]
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	if !session.CheckPointInBlackRects("LoginActivity", types.Point{X: 50, Y: 50}) {
		t.Fatalf("鐐逛綅搴斿懡涓粦鍚嶅崟鍖哄煙")
	}
}

func TestSessionTransformPageInfoWithInput(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "mix.js")
	configContent := `const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.page_name + "_v2",
      xml: ctx.page.xml.replace("foo", "bar"),
    }
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	info, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{XMLDescOfGuiTree: `<node text="foo"/>`})
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 澶辫触: %v", err)
	}
	if info.PageName != "MainActivity_v2" {
		t.Fatalf("椤甸潰鍚嶆敼閫犵粨鏋滀笉绗﹀悎棰勬湡: %s", info.PageName)
	}
	if info.XML != `<node text="bar"/>` {
		t.Fatalf("xml 鏀归€犵粨鏋滀笉绗﹀悎棰勬湡: %s", info.XML)
	}
}

func TestSessionTransformPageInfoWithOCRPageControlStrategy(t *testing.T) {
	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "ocr",
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	session.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)},
				perception.SourceOCR,
				"ocr_click:登录",
				map[string]string{"ocr_text": "登录"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	info, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot:       mustPNG(t, 200, 400),
	})
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 失败: %v", err)
	}
	if !strings.Contains(info.XML, `resource-id="visual_1"`) {
		t.Fatalf("预期生成伪控件节点，实际 XML: %s", info.XML)
	}
	if !strings.Contains(info.XML, `text="登录"`) {
		t.Fatalf("预期生成 OCR 文本，实际 XML: %s", info.XML)
	}
	if !strings.Contains(info.XML, `bounds="[0.100000,0.200000][0.300000,0.400000]"`) {
		t.Fatalf("预期归一化 bounds，实际 XML: %s", info.XML)
	}
}

func TestSessionTransformPageInfoWithOCRPageControlStrategyUsesFingerprintCache(t *testing.T) {
	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "ocr",
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	callCount := 0
	session.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			callCount++
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.15, 0.2, 0.45, 0.4)},
				perception.SourceOCR,
				"ocr_click:缓存登录",
				map[string]string{"ocr_text": "缓存登录"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	input := ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot:       mustPNG(t, 240, 360),
	}
	first, err := session.TransformPageInfoWithInput("MainActivity", input)
	if err != nil {
		t.Fatalf("首次 TransformPageInfoWithInput 失败: %v", err)
	}
	second, err := session.TransformPageInfoWithInput("MainActivity", input)
	if err != nil {
		t.Fatalf("二次 TransformPageInfoWithInput 失败: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("预期 OCR provider 仅调用 1 次，实际: %d", callCount)
	}
	if first.XML != second.XML {
		t.Fatalf("缓存命中后应复用同一份控件树，首次=%s 二次=%s", first.XML, second.XML)
	}
	if !strings.Contains(second.XML, `text="缓存登录"`) {
		t.Fatalf("缓存结果缺少 OCR 控件信息，实际 XML: %s", second.XML)
	}
}

func TestSessionTransformPageInfoWithOCRPageControlStrategyRefreshesAfterTTLExpires(t *testing.T) {
	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "ocr",
		PageControlCacheTTL: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	callCount := 0
	session.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			callCount++
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.2, 0.2, 0.4, 0.35)},
				perception.SourceOCR,
				"ocr_click:重复刷新",
				map[string]string{"ocr_text": "重复刷新"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	input := ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot:       mustPNG(t, 240, 360),
	}
	if _, err := session.TransformPageInfoWithInput("MainActivity", input); err != nil {
		t.Fatalf("首次 TransformPageInfoWithInput 失败: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := session.TransformPageInfoWithInput("MainActivity", input); err != nil {
		t.Fatalf("TTL 过期后二次 TransformPageInfoWithInput 失败: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("预期缓存 TTL 到期后重新调用 OCR provider，实际调用次数: %d", callCount)
	}
}

func TestSessionTransformPageInfoWithOCRPageControlStrategyLoadsPersistentCacheAcrossSessions(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "page_control_cache.sqlite")

	firstSession, err := NewSession(Config{
		PackageName:          "com.demo",
		PageControlStrategy:  "ocr",
		PageControlCacheFile: cachePath,
	})
	if err != nil {
		t.Fatalf("创建首次会话失败: %v", err)
	}
	defer closeSessionMemoryStore(firstSession)

	firstCallCount := 0
	firstSession.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			firstCallCount++
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.15, 0.2, 0.45, 0.4)},
				perception.SourceOCR,
				"ocr_click:持久化缓存登录",
				map[string]string{"ocr_text": "持久化缓存登录"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	input := ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot:       mustPNG(t, 240, 360),
	}
	first, err := firstSession.TransformPageInfoWithInput("MainActivity", input)
	if err != nil {
		t.Fatalf("首次 TransformPageInfoWithInput 失败: %v", err)
	}
	if firstCallCount != 1 {
		t.Fatalf("预期首次 provider 调用 1 次，实际: %d", firstCallCount)
	}

	secondSession, err := NewSession(Config{
		PackageName:          "com.demo",
		PageControlStrategy:  "ocr",
		PageControlCacheFile: cachePath,
	})
	if err != nil {
		t.Fatalf("创建二次会话失败: %v", err)
	}
	defer closeSessionMemoryStore(secondSession)

	secondCallCount := 0
	secondSession.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			secondCallCount++
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.3, 0.3, 0.5, 0.45)},
				perception.SourceOCR,
				"ocr_click:不应触发",
				map[string]string{"ocr_text": "不应触发"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	second, err := secondSession.TransformPageInfoWithInput("MainActivity", input)
	if err != nil {
		t.Fatalf("二次 TransformPageInfoWithInput 失败: %v", err)
	}
	if secondCallCount != 0 {
		t.Fatalf("预期二次会话直接命中持久化缓存，实际 provider 调用次数: %d", secondCallCount)
	}
	if first.XML != second.XML {
		t.Fatalf("跨会话持久化缓存应复用同一份控件树，首次=%s 二次=%s", first.XML, second.XML)
	}
	if !strings.Contains(second.XML, `text="持久化缓存登录"`) {
		t.Fatalf("持久化缓存结果缺少 OCR 控件信息，实际 XML: %s", second.XML)
	}
}

func TestSessionTransformPageInfoWithLLMPageControlStrategy(t *testing.T) {
	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "llm",
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	session.llmProvider = &mockPageControlProvider{
		detectFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.25, 0.25, 0.5, 0.5)},
				perception.SourceLLM,
				"点击确认",
				map[string]string{"llm_control_text": "确认按钮", "llm_control_type": "dialog_action", "llm_action_type": "click"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	info, err := session.TransformPageInfoWithInput("DialogActivity", ActionInput{
		Screenshot: mustPNG(t, 400, 400),
	})
	if err != nil {
		t.Fatalf("TransformPageInfoWithInput 失败: %v", err)
	}
	if !strings.Contains(info.XML, `text="确认按钮"`) {
		t.Fatalf("预期生成 LLM 控件提示，实际 XML: %s", info.XML)
	}
	if !strings.Contains(info.XML, `trek-scroll-infer-disabled="true"`) {
		t.Fatalf("LLM 合成 XML 应显式禁用滚动推断，实际 XML: %s", info.XML)
	}
}

func TestSessionNextBlockRecoveryActionForcesPageControlRefresh(t *testing.T) {
	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "ocr",
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	callCount := 0
	session.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			callCount++
			item := perception.NewCandidate(
				&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.3, 0.2)},
				perception.SourceOCR,
				"ocr_click:阻塞刷新",
				map[string]string{"ocr_text": "阻塞刷新"},
			)
			return []perception.Candidate{item}, nil
		},
	}

	screenshot := mustPNG(t, 300, 300)
	if _, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot:       screenshot,
	}); err != nil {
		t.Fatalf("预热缓存失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery) {
      return trek.action.back()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	action, err := session.NextBlockRecoveryAction("MainActivity", ActionInput{
		Screenshot: screenshot,
	})
	if err != nil {
		t.Fatalf("获取阻塞恢复动作失败: %v", err)
	}
	if action == nil || action.Act != types.BACK {
		t.Fatalf("预期阻塞恢复返回 BACK，实际: %+v", action)
	}
	if callCount != 2 {
		t.Fatalf("阻塞恢复路径应强制重新调用 OCR provider，实际调用次数: %d", callCount)
	}
}

func TestSessionBeforeDecideUsesGojaPluginAction(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    return trek.action.back()
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇鍔ㄤ綔澶辫触: %v", err)
	}
	if action.Act != types.BACK {
		t.Fatalf("鎻掍欢搴旇鐩栦负 BACK锛屽疄锟? %s", action.Act.String())
	}
}

func TestSessionOnStepResultFeedsGojaPluginState(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  onStepResult(ctx) {
    if (ctx.result.crash && ctx.result.after.xml.indexOf("After") >= 0) {
      trek.store.set("should_back", true)
    }
  },
  beforeDecide(ctx) {
    if (trek.store.get("should_back")) return trek.action.back()
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("鍐欏叆娴嬭瘯鏂囦欢澶辫触: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("鍔犺浇閰嶇疆澶辫触: %v", err)
	}

	after := PageSnapshot{PageName: "After", XML: `<hierarchy text="After"/>`, Screenshot: []byte{1, 2, 3}}
	if err := session.OnStepResult(StepResultInput{
		Step:    1,
		Action:  &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0, 0, 10, 10)},
		Success: false,
		Crash:   true,
		Before:  PageSnapshot{PageName: "Before", XML: `<hierarchy/>`},
		After:   &after,
	}); err != nil {
		t.Fatalf("OnStepResult 澶辫触: %v", err)
	}

	action, err := session.NextAction("MainActivity", `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`)
	if err != nil {
		t.Fatalf("鑾峰彇鍔ㄤ綔澶辫触: %v", err)
	}
	if action.Act != types.BACK {
		t.Fatalf("onStepResult 鐘舵€佸簲椹卞姩涓嬩竴锟?BACK锛屽疄锟? %s", action.Act.String())
	}
}

func TestSessionNextBlockRecoveryActionUsesPluginBlockRecoveryContext(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery) {
      return trek.action.back()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	action, err := session.NextBlockRecoveryAction("MainActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`,
	})
	if err != nil {
		t.Fatalf("获取阻塞恢复动作失败: %v", err)
	}
	if action == nil || action.Act != types.BACK {
		t.Fatalf("预期阻塞恢复返回 BACK，实际: %+v", action)
	}
}

func TestSessionNextBlockRecoveryActionRejectsRestartActions(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery) {
      return trek.action.restart()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	action, err := session.NextBlockRecoveryAction("MainActivity", ActionInput{
		XMLDescOfGuiTree: `<hierarchy><node class="android.widget.TextView" bounds="[0,0][10,10]"/></hierarchy>`,
	})
	if err != nil {
		t.Fatalf("获取阻塞恢复动作失败: %v", err)
	}
	if action != nil {
		t.Fatalf("阻塞恢复不应返回重启动作，实际: %+v", action)
	}
}

func TestSessionBuildMemoryRecoveryCandidates(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.sqlite")
	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("初始化 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	record := memory.RecoveryMemoryRecord{
		MemoryKey:        memory.BuildMemoryKey("page.sig", "same_page_no_change", "back", "recover"),
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		TraceSignature:   "back",
		Mode:             "recover",
		Item: perception.NewCandidate(
			&types.ActionCommand{Act: types.BACK},
			perception.SourceAlgorithm,
			"回退上一层",
			map[string]string{"seed": "1"},
		),
		Outcome:      memory.OutcomeEscaped,
		EscapeScore:  0.8,
		SuccessCount: 3,
		FailCount:    1,
		LastUsedAt:   time.Now(),
		CreatedAt:    time.Now(),
	}
	if err := store.Append(record); err != nil {
		t.Fatalf("写入 memory 记录失败: %v", err)
	}

	session, err := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	items, err := session.BuildMemoryRecoveryCandidates(enginestate.TraversalContext{
		Mode:             "recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	})
	if err != nil {
		t.Fatalf("构建 memory 恢复候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("预期命中 1 条候选，实际: %d", len(items))
	}
	if items[0].Source != perception.SourceMemory {
		t.Fatalf("候选来源应为 memory，实际: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types.BACK {
		t.Fatalf("候选动作应为 BACK，实际: %+v", items[0].Command)
	}
	if items[0].Metadata["memory_key"] == "" {
		t.Fatalf("候选缺少 memory_key 元数据")
	}
}

func TestSessionBuildHeuristicRecoveryCandidates(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery) {
      return trek.action.back()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	items, err := session.BuildHeuristicRecoveryCandidates(enginestate.TraversalContext{
		Mode:       "Recover",
		PageName:   "MainActivity",
		XML:        `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
		Screenshot: []byte{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("构建 heuristic 恢复候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("预期 1 条 heuristic 候选，实际: %d", len(items))
	}
	if items[0].Source != perception.SourceHeuristic {
		t.Fatalf("候选来源应为 heuristic，实际: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types.BACK {
		t.Fatalf("候选动作应为 BACK，实际: %+v", items[0].Command)
	}
}

func TestSessionBuildHeuristicRecoveryCandidatesRejectRestart(t *testing.T) {
	session, err := NewSession(Config{PackageName: "com.demo"})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "plugin.js")
	configContent := `const plugin = {
  beforeDecide(ctx) {
    if (ctx.runtime.block_recovery) {
      return trek.action.restart()
    }
    return null
  }
};`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := session.LoadConfigFile(configPath); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	items, err := session.BuildHeuristicRecoveryCandidates(enginestate.TraversalContext{
		Mode:     "Recover",
		PageName: "MainActivity",
		XML:      `<hierarchy><node class="android.widget.TextView"/></hierarchy>`,
	})
	if err != nil {
		t.Fatalf("构建 heuristic 恢复候选失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("重启动作不应进入恢复候选，实际: %+v", items)
	}
}

func TestSessionSelectRecoveryActionPrefersAlgorithmCandidate(t *testing.T) {
	session, err := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	ctx := enginestate.TraversalContext{Mode: "Recover"}
	items := []perception.Candidate{
		{
			Command:    &types.ActionCommand{Act: types.BACK},
			Source:     perception.SourceAlgorithm,
			Confidence: 0.3,
		},
		{
			Command:    &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
			Source:     perception.SourceLLM,
			Confidence: 0.9,
		},
	}
	session.traversalAlgo = &mockTraversalAlgorithm{
		selectFn: func(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error) {
			if len(candidates) == 0 {
				return nil, nil
			}
			return candidates[0].Command, nil
		},
	}
	cmd, err := session.SelectRecoveryAction(ctx, items)
	if err != nil {
		t.Fatalf("SelectRecoveryAction 失败: %v", err)
	}
	if cmd == nil || cmd.Act != types.BACK {
		t.Fatalf("预期优先选择 algorithm 候选 BACK，实际: %+v", cmd)
	}
}

func TestSessionBuildAlgorithmCandidatesDelegatesTraversalAlgorithm(t *testing.T) {
	session, err := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	called := false
	expected := []perception.Candidate{
		{
			Command:    &types.ActionCommand{Act: types.BACK},
			Source:     perception.SourceAlgorithm,
			Confidence: 0.8,
		},
	}
	session.traversalAlgo = &mockTraversalAlgorithm{
		proposeFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			called = true
			return expected, nil
		},
	}

	items, err := session.BuildAlgorithmCandidates(enginestate.TraversalContext{
		Mode:     "Explore",
		PageName: "MainActivity",
	})
	if err != nil {
		t.Fatalf("BuildAlgorithmCandidates 失败: %v", err)
	}
	if !called {
		t.Fatalf("预期调用 traversalAlgo.ProposeCandidates")
	}
	if len(items) != 1 || items[0].Command == nil || items[0].Command.Act != types.BACK {
		t.Fatalf("算法候选返回不符合预期: %+v", items)
	}
}

func TestSessionBuildAlgorithmCandidatesMergesOCRCandidates(t *testing.T) {
	session, err := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	session.traversalAlgo = &mockTraversalAlgorithm{
		proposeFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			return []perception.Candidate{
				{
					Command:    &types.ActionCommand{Act: types.BACK},
					Source:     perception.SourceAlgorithm,
					Confidence: 0.8,
				},
			}, nil
		},
	}
	session.ocrProvider = &mockCandidateProvider{
		buildFn: func(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
			return []perception.Candidate{
				{
					Command:    &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.2, 0.3, 0.4)},
					Source:     perception.SourceOCR,
					Confidence: 0.7,
				},
			}, nil
		},
	}

	items, err := session.BuildAlgorithmCandidates(enginestate.TraversalContext{
		Mode:       "Explore",
		PageName:   "MainActivity",
		Screenshot: []byte{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("BuildAlgorithmCandidates 失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("预期合并 algorithm + ocr 候选，实际: %d", len(items))
	}
	if items[0].Source != perception.SourceAlgorithm || items[1].Source != perception.SourceOCR {
		t.Fatalf("候选来源顺序错误: %+v", items)
	}
}

func TestSessionObserveTraversalOutcomeNoError(t *testing.T) {
	session, err := NewSession(Config{
		PackageName: "com.demo",
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	err = session.ObserveTraversalOutcome(
		enginestate.TraversalContext{Mode: "Explore", PageName: "MainActivity"},
		&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
		traversal.OutcomeNewState,
	)
	if err != nil {
		t.Fatalf("ObserveTraversalOutcome 不应报错: %v", err)
	}
}

func TestSessionRecordRecoveryMemoryOutcome(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.sqlite")
	session, err := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})

	err = session.RecordRecoveryMemoryOutcome(
		enginestate.TraversalContext{
			Mode:             "Recover",
			PageSignature:    "page.sig",
			ClusterSignature: "cluster.sig",
			BlockReason:      "same_page_no_change",
			RecentTrace: []enginestate.ActionTrace{
				{ActionKey: "back"},
				{ActionKey: "click"},
			},
		},
		perception.Candidate{
			Command: &types.ActionCommand{Act: types.BACK},
			Source:  perception.SourceHeuristic,
		},
		true,
	)
	if err != nil {
		t.Fatalf("写回 recovery memory 失败: %v", err)
	}

	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("读取 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("预期写入 1 条记录，实际: %d", len(all))
	}
	if all[0].Outcome != memory.OutcomeEscaped {
		t.Fatalf("预期 outcome=escaped，实际: %s", all[0].Outcome)
	}
	if all[0].SuccessCount != 1 || all[0].FailCount != 0 {
		t.Fatalf("预期成功/失败计数为 1/0，实际: %d/%d", all[0].SuccessCount, all[0].FailCount)
	}
	if all[0].TraceSignature != "back>click" {
		t.Fatalf("预期 trace 签名为 back>click，实际: %s", all[0].TraceSignature)
	}
}

func TestSessionRecordRecoveryMemoryOutcomeAggregatesCounts(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.sqlite")
	session, err := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	}
	item := perception.Candidate{
		Command: &types.ActionCommand{Act: types.BACK},
		Source:  perception.SourceMemory,
	}

	if err := session.RecordRecoveryMemoryOutcome(ctx, item, true); err != nil {
		t.Fatalf("写回成功样本失败: %v", err)
	}
	if err := session.RecordRecoveryMemoryOutcome(ctx, item, false); err != nil {
		t.Fatalf("写回失败样本失败: %v", err)
	}

	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("读取 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("聚合后预期 1 条记录，实际: %d", len(all))
	}
	if all[0].SuccessCount != 1 || all[0].FailCount != 1 {
		t.Fatalf("聚合计数错误: success=%d fail=%d", all[0].SuccessCount, all[0].FailCount)
	}
	if all[0].Outcome != memory.OutcomeFailed {
		t.Fatalf("最新 outcome 应为 failed，实际: %s", all[0].Outcome)
	}
}

func TestSessionRecordCandidateEnhancementOutcome(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.sqlite")
	session, err := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Explore",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "click"},
		},
	}
	item := perception.Candidate{
		Command: &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
		Source:  perception.SourceLLM,
	}
	if err := session.RecordCandidateEnhancementOutcome(ctx, item, true); err != nil {
		t.Fatalf("写回候选增强结果失败: %v", err)
	}

	store, err := memory.NewStore(memoryPath)
	if err != nil {
		t.Fatalf("读取 memory store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("预期写入 1 条记录，实际: %d", len(all))
	}
	if all[0].BlockReason != memory.BlockReasonCandidateEnhancement {
		t.Fatalf("预期 block_reason=%s，实际: %s", memory.BlockReasonCandidateEnhancement, all[0].BlockReason)
	}
	if all[0].Outcome != memory.OutcomeEscaped || all[0].SuccessCount != 1 {
		t.Fatalf("预期写入 improved 正样本，实际: outcome=%s success=%d", all[0].Outcome, all[0].SuccessCount)
	}
}

func TestSessionBuildKnownFailedRecoveryActions(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.sqlite")
	session, err := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	}
	item := perception.Candidate{
		Command: &types.ActionCommand{Act: types.BACK},
		Source:  perception.SourceMemory,
	}
	if err := session.RecordRecoveryMemoryOutcome(ctx, item, false); err != nil {
		t.Fatalf("写回失败样本失败: %v", err)
	}

	known, err := session.BuildKnownFailedRecoveryActions(ctx)
	if err != nil {
		t.Fatalf("提取失败动作失败: %v", err)
	}
	if !known[item.Command.ToJSON()] {
		t.Fatalf("预期包含 BACK 失败动作 key")
	}
}

func TestSessionBuildKnownSuccessfulRecoveryActions(t *testing.T) {
	memoryPath := filepath.Join(t.TempDir(), "recovery.sqlite")
	session, err := NewSession(Config{
		PackageName:        "com.demo",
		RecoveryMemoryFile: memoryPath,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	t.Cleanup(func() {
		closeSessionMemoryStore(session)
	})
	ctx := enginestate.TraversalContext{
		Mode:             "Recover",
		PageSignature:    "page.sig",
		ClusterSignature: "cluster.sig",
		BlockReason:      "same_page_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "back"},
		},
	}
	item := perception.Candidate{
		Command: &types.ActionCommand{Act: types.BACK},
		Source:  perception.SourceMemory,
	}
	if err := session.RecordRecoveryMemoryOutcome(ctx, item, true); err != nil {
		t.Fatalf("写回成功样本失败: %v", err)
	}

	known, err := session.BuildKnownSuccessfulRecoveryActions(ctx)
	if err != nil {
		t.Fatalf("提取成功动作失败: %v", err)
	}
	if !known[item.Command.ToJSON()] {
		t.Fatalf("预期包含 BACK 成功动作 key")
	}
}

func TestSessionBuildLLMRecoveryCandidatesDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{
				{
					"intent":      "返回",
					"action_type": "BACK",
					"confidence":  0.7,
					"reason":      "弹窗疑似遮挡",
				},
			},
		})
	}))
	defer server.Close()

	session, err := NewSession(Config{
		PackageName:         "com.demo",
		RecoveryLLMEndpoint: server.URL,
		RecoveryLLMAPIKey:   "test-key",
		RecoveryLLMModel:    "gpt-x",
		RecoveryLLMTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	items, err := session.BuildLLMRecoveryCandidates(enginestate.TraversalContext{
		Step:        5,
		Mode:        "Recover",
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	if err != nil {
		t.Fatalf("调用已禁用的 llm 恢复候选接口失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("llm 不应再直接参与决策，实际返回: %+v", items)
	}
}

func TestSessionBuildLLMRecoveryCandidatesWithOpenAIResponsesProviderDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": `{"candidates":[{"intent":"返回","action_type":"BACK","confidence":0.8}]}`,
		})
	}))
	defer server.Close()

	session, err := NewSession(Config{
		PackageName:              "com.demo",
		RecoveryLLMOpenAIModel:   "gpt-4.1-mini",
		RecoveryLLMOpenAIAPIKey:  "sk-test",
		RecoveryLLMOpenAIBaseURL: server.URL,
		RecoveryLLMTimeout:       2 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	items, err := session.BuildLLMRecoveryCandidates(enginestate.TraversalContext{
		Step:        6,
		Mode:        "Recover",
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	if err != nil {
		t.Fatalf("openai provider 调用已禁用接口失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("openai provider 不应再直接返回决策候选: %+v", items)
	}
}

func TestSessionInitLLMProviderFromEnvForPageControlOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer env-key" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"controls": []map[string]any{
				{
					"action_type":  "click",
					"control_type": "button",
					"text":         "继续",
					"hint":         "下一步",
					"clickable":    true,
					"confidence":   0.9,
					"bounds": map[string]any{
						"left":   0.1,
						"top":    0.1,
						"right":  0.3,
						"bottom": 0.2,
					},
				},
			},
		})
	}))
	defer server.Close()

	t.Setenv("LLM_API_URL", server.URL)
	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_MODEL", "env-model")

	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "llm",
		RecoveryLLMTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	info, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{
		Screenshot: mustPNG(t, 200, 200),
	})
	if err != nil {
		t.Fatalf("环境变量 LLM provider 控件检测失败: %v", err)
	}
	if !strings.Contains(info.XML, `text="继续"`) {
		t.Fatalf("环境变量 LLM provider 应仅用于控件检测，实际 XML: %s", info.XML)
	}
}

func TestSessionInitOpenAIProviderFromEnvForPageControlOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-env" {
			t.Fatalf("Authorization 错误: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"controls":[{"action_type":"click","control_type":"button","text":"确认","hint":"主按钮","clickable":true,"confidence":0.8,"bounds":{"left":0.2,"top":0.2,"right":0.6,"bottom":0.6}}]}`,
					},
				},
			},
		})
	}))
	defer server.Close()

	t.Setenv("OPENAI_MODEL", "gpt-4.1-mini")
	t.Setenv("OPENAI_API_KEY", "sk-env")
	t.Setenv("OPENAI_API_URL", server.URL)

	session, err := NewSession(Config{
		PackageName:         "com.demo",
		PageControlStrategy: "llm",
		RecoveryLLMTimeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	info, err := session.TransformPageInfoWithInput("MainActivity", ActionInput{
		Screenshot: mustPNG(t, 300, 300),
	})
	if err != nil {
		t.Fatalf("环境变量 OpenAI provider 控件检测失败: %v", err)
	}
	if !strings.Contains(info.XML, `text="确认"`) {
		t.Fatalf("环境变量 OpenAI provider 应仅用于控件检测，实际 XML: %s", info.XML)
	}
}
