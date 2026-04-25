package scripting

import "testing"

func TestManagerTransformBeforeAfterAndState(t *testing.T) {
	manager, err := LoadScript(`const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.name + "_patched",
      xml: trek.page.patchText(ctx.page.xml, "old", "new"),
    }
  },
  beforeDecide(ctx) {
    if (trek.state.inc("decide_count") === 1) {
      return trek.action.click([1, 2, 3, 4])
    }
    return null
  },
  afterDecide(ctx, action) {
    if (action.action === "CLICK") {
      return trek.action.back()
    }
    return action
  },
}`)
	if err != nil {
		t.Fatalf("加载插件失败: %v", err)
	}

	ctx := PluginContext{
		Page: PageSnapshot{Name: "Login", XML: `<node text="old"/>`},
		Runtime: RuntimeContext{
			Step:           1,
			PackageName:    "com.demo",
			PageSourceType: "uia",
		},
	}
	page, err := manager.TransformPage(ctx)
	if err != nil {
		t.Fatalf("页面改造失败: %v", err)
	}
	if page.Name != "Login_patched" || page.XML != `<node text="new"/>` {
		t.Fatalf("页面改造结果不符合预期: %+v", page)
	}

	action, handled, err := manager.BeforeDecide(ctx)
	if err != nil {
		t.Fatalf("beforeDecide 失败: %v", err)
	}
	if !handled || action == nil || action.Type != ActionClick {
		t.Fatalf("beforeDecide 应返回 click 动作: handled=%v action=%+v", handled, action)
	}
	if action.Bounds != [4]float64{1, 2, 3, 4} {
		t.Fatalf("click bounds 不符合预期: %+v", action.Bounds)
	}

	afterAction, handled, err := manager.AfterDecide(ctx, action)
	if err != nil {
		t.Fatalf("afterDecide 失败: %v", err)
	}
	if !handled || afterAction == nil || afterAction.Type != ActionBack {
		t.Fatalf("afterDecide 应替换为 back 动作: handled=%v action=%+v", handled, afterAction)
	}

	if got := manager.StateGet("decide_count"); got != int64(1) {
		t.Fatalf("脚本状态未按预期持久化: %v", got)
	}
}

func TestManagerExposesScreenshotBytesToScript(t *testing.T) {
	manager, err := LoadScript(`const plugin = {
  beforeDecide(ctx) {
    const bytes = trek.page.screenshotBytes(ctx.page)
    if (bytes && bytes.length === 3 && bytes[0] === 7) {
      return trek.action.back()
    }
    return null
  },
}`)
	if err != nil {
		t.Fatalf("加载插件失败: %v", err)
	}

	action, handled, err := manager.BeforeDecide(PluginContext{
		Page: PageSnapshot{
			Name: "Main",
			XML:  `<node/>`,
			Screenshot: &Screenshot{
				Bytes: []byte{7, 8, 9},
				MIME:  "image/png",
			},
		},
	})
	if err != nil {
		t.Fatalf("beforeDecide 失败: %v", err)
	}
	if !handled || action == nil || action.Type != ActionBack {
		t.Fatalf("截图字节未正确暴露给脚本: handled=%v action=%+v", handled, action)
	}
}

func TestManagerOnStepResultSeesCrashANRAndBeforeAfterPages(t *testing.T) {
	manager, err := LoadScript(`const plugin = {
  onStepResult(ctx) {
    if (ctx.result.crash) trek.state.set("crash_page", ctx.result.before.name)
    if (ctx.result.anr) trek.state.set("anr_page", ctx.result.after.name)
    trek.state.set("after_xml", ctx.result.after.xml)
  },
}`)
	if err != nil {
		t.Fatalf("加载插件失败: %v", err)
	}

	err = manager.OnStepResult(StepResultContext{
		PluginContext: PluginContext{Page: PageSnapshot{Name: "Before", XML: `<before/>`}},
		Result: StepResult{
			Step:    1,
			Action:  Action{Type: ActionClick, Bounds: [4]float64{0, 0, 10, 10}},
			Success: false,
			Crash:   true,
			ANR:     true,
			Before:  PageSnapshot{Name: "Before", XML: `<before/>`},
			After:   &PageSnapshot{Name: "After", XML: `<after/>`},
		},
	})
	if err != nil {
		t.Fatalf("onStepResult 失败: %v", err)
	}
	if got := manager.StateGet("crash_page"); got != "Before" {
		t.Fatalf("crash page 不符合预期: %v", got)
	}
	if got := manager.StateGet("anr_page"); got != "After" {
		t.Fatalf("anr page 不符合预期: %v", got)
	}
	if got := manager.StateGet("after_xml"); got != `<after/>` {
		t.Fatalf("after xml 不符合预期: %v", got)
	}
}

func TestLoadStaticConfigReadsLogFileLevel(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  log: {
    file_level: "warn"
  }
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if cfg.Log.FileLevel != "warn" {
		t.Fatalf("文件日志级别不符合预期: %q", cfg.Log.FileLevel)
	}
}

func TestLoadStaticConfigReadsPageSourceAndTouchMode(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  page_source: "uia",
  touch_mode: "motion"
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if cfg.PageSource != "uia" {
		t.Fatalf("页面源配置不符合预期: %q", cfg.PageSource)
	}
	if cfg.TouchMode != "motion" {
		t.Fatalf("触控模式配置不符合预期: %q", cfg.TouchMode)
	}
}

func TestLoadStaticConfigReadsCamelCasePageSourceAndTouchMode(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  pageSource: "poco",
  touchMode: "uia"
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if cfg.PageSource != "poco" {
		t.Fatalf("页面源配置不符合预期: %q", cfg.PageSource)
	}
	if cfg.TouchMode != "uia" {
		t.Fatalf("触控模式配置不符合预期: %q", cfg.TouchMode)
	}
}

func TestLoadStaticConfigReadsPageNameStrategy(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  page_name_strategy: "xml_only"
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if cfg.PageNameStrategy != "xml_only" {
		t.Fatalf("页面名策略配置不符合预期: %q", cfg.PageNameStrategy)
	}
}

func TestLoadStaticConfigReadsUIAAndPocoSettings(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  uia: {
    server_port: 7900,
  },
  poco: {
    engine: "UNITY_3D",
    port: 5101,
  }
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if cfg.UIA.ServerPort != 7900 {
		t.Fatalf("uia 配置不符合预期: %+v", cfg.UIA)
	}
	if cfg.Poco.Engine != "UNITY_3D" || cfg.Poco.Port != 5101 {
		t.Fatalf("poco 配置不符合预期: %+v", cfg.Poco)
	}
}

func TestLoadStaticConfigReadsEffectiveTouchAreaBySerialAndPackage(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  effective_touch_area: {
    serial: "192.168.2.198:5555",
    package_name: "com.NetEase",
    range: {
      left: 0.043,
      top: 0,
      right: 1,
      bottom: 1
    }
  }
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if cfg.EffectiveTouchArea == nil {
		t.Fatalf("预期解析到 effective_touch_area")
	}
	if cfg.EffectiveTouchArea.Serial != "192.168.2.198:5555" {
		t.Fatalf("serial 不符合预期: %q", cfg.EffectiveTouchArea.Serial)
	}
	if cfg.EffectiveTouchArea.PackageName != "com.NetEase" {
		t.Fatalf("package_name 不符合预期: %q", cfg.EffectiveTouchArea.PackageName)
	}
	if cfg.EffectiveTouchArea.Range.Left != 0.043 || cfg.EffectiveTouchArea.Range.Right != 1 {
		t.Fatalf("range 不符合预期: %+v", cfg.EffectiveTouchArea.Range)
	}
}

func TestLoadStaticConfigReadsUCTBanditSettings(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  uct_bandit: {
    two_state_loop_penalty: -4.5,
    edge_repeat_penalty: -1.2,
    edge_repeat_threshold: 3,
    action_cooldown_penalty: 0.75,
    recent_action_window: 8,
    loop_escape_explore_boost: 0.2
  }
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if !cfg.UCTBandit.HasTwoStateLoopPenalty || cfg.UCTBandit.TwoStateLoopPenalty != -4.5 {
		t.Fatalf("two_state_loop_penalty 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.HasEdgeRepeatPenalty || cfg.UCTBandit.EdgeRepeatPenalty != -1.2 {
		t.Fatalf("edge_repeat_penalty 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.HasEdgeRepeatThreshold || cfg.UCTBandit.EdgeRepeatThreshold != 3 {
		t.Fatalf("edge_repeat_threshold 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.HasActionCooldownPenalty || cfg.UCTBandit.ActionCooldownPenalty != 0.75 {
		t.Fatalf("action_cooldown_penalty 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.HasRecentActionWindow || cfg.UCTBandit.RecentActionWindow != 8 {
		t.Fatalf("recent_action_window 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.HasLoopEscapeExploreBoost || cfg.UCTBandit.LoopEscapeExploreBoost != 0.2 {
		t.Fatalf("loop_escape_explore_boost 不符合预期: %+v", cfg.UCTBandit)
	}
}
