package scripting

import "testing"

func TestManagerTransformBeforeAfterAndState(t *testing.T) {
	manager, err := LoadScript(`const plugin = {
  transformPage(ctx) {
    return {
      page_name: ctx.page.page_name + "_patched",
      xml: trek.page.replaceText(ctx.page.xml, "old", "new"),
    }
  },
  beforeDecide(ctx) {
    if (trek.store.increment("decide_count") === 1) {
      return trek.action.click([1, 2, 3, 4])
    }
    return null
  },
  afterDecide(ctx, action) {
    if (action.type === "CLICK") {
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
    if (ctx.result.crash) trek.store.set("crash_page", ctx.result.before.page_name)
    if (ctx.result.anr) trek.store.set("anr_page", ctx.result.after.page_name)
    trek.store.set("after_xml", ctx.result.after.xml)
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

func TestLoadStaticConfigReadsPlugins(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  plugins: ["./plugins/a.plugin.js", "./plugins/b.plugin.js", ""]
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if len(cfg.Plugins) != 2 {
		t.Fatalf("plugins 数量不符合预期: %+v", cfg.Plugins)
	}
	if cfg.Plugins[0] != "./plugins/a.plugin.js" || cfg.Plugins[1] != "./plugins/b.plugin.js" {
		t.Fatalf("plugins 内容不符合预期: %+v", cfg.Plugins)
	}
}

func TestLoadStaticConfigReadsCaptureScreenshotAndKeepStepRecords(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  capture_screenshot: true,
  keep_step_records: false
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if !cfg.CaptureScreenshot.IsSet() || !cfg.CaptureScreenshot.Get() {
		t.Fatalf("capture_screenshot 不符合预期: %+v", cfg)
	}
	if !cfg.KeepStepRecords.IsSet() || cfg.KeepStepRecords.Get() {
		t.Fatalf("keep_step_records 不符合预期: %+v", cfg)
	}
}

func TestLoadStaticConfigReadsCamelCaseCaptureScreenshotAndKeepStepRecords(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  captureScreenshot: false,
  keepStepRecords: true
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if !cfg.CaptureScreenshot.IsSet() || cfg.CaptureScreenshot.Get() {
		t.Fatalf("captureScreenshot 不符合预期: %+v", cfg)
	}
	if !cfg.KeepStepRecords.IsSet() || !cfg.KeepStepRecords.Get() {
		t.Fatalf("keepStepRecords 不符合预期: %+v", cfg)
	}
}

func TestLoadStaticConfigReadsRecoveryAndCandidateTuningSettings(t *testing.T) {
	cfg, err := LoadStaticConfig(`const config = {
  explore_ocr_timeout_ms: 12000,
  llm_timeout_ms: 23000,
  recovery_cooldown_steps: 3,
  llm_max_calls: 5,
  llm_window_steps: 40,
  recovery_two_state_loop_threshold: 4,
  recovery_high_visit_threshold: 9,
  recovery_low_reward_window: 7,
  candidate_ambiguity_top_gap_threshold: 0.12,
  high_value_page_visit_limit: 3,
  candidate_risk_drop_threshold: 1.9,
  candidate_min_fusion_score: -0.2
}`)
	if err != nil {
		t.Fatalf("加载静态配置失败: %v", err)
	}
	if !cfg.ExploreOCRTimeoutMs.IsSet() || cfg.ExploreOCRTimeoutMs.Get() != 12000 {
		t.Fatalf("explore_ocr_timeout_ms 不符合预期: %+v", cfg)
	}
	if !cfg.LLMTimeoutMs.IsSet() || cfg.LLMTimeoutMs.Get() != 23000 {
		t.Fatalf("llm_timeout_ms 不符合预期: %+v", cfg)
	}
	if !cfg.RecoveryCooldownSteps.IsSet() || cfg.RecoveryCooldownSteps.Get() != 3 {
		t.Fatalf("recovery_cooldown_steps 不符合预期: %+v", cfg)
	}
	if !cfg.LLMMaxCalls.IsSet() || cfg.LLMMaxCalls.Get() != 5 {
		t.Fatalf("llm_max_calls 不符合预期: %+v", cfg)
	}
	if !cfg.LLMWindowSteps.IsSet() || cfg.LLMWindowSteps.Get() != 40 {
		t.Fatalf("llm_window_steps 不符合预期: %+v", cfg)
	}
	if !cfg.RecoveryTwoStateLoopThreshold.IsSet() || cfg.RecoveryTwoStateLoopThreshold.Get() != 4 {
		t.Fatalf("recovery_two_state_loop_threshold 不符合预期: %+v", cfg)
	}
	if !cfg.RecoveryHighVisitThreshold.IsSet() || cfg.RecoveryHighVisitThreshold.Get() != 9 {
		t.Fatalf("recovery_high_visit_threshold 不符合预期: %+v", cfg)
	}
	if !cfg.RecoveryLowRewardWindow.IsSet() || cfg.RecoveryLowRewardWindow.Get() != 7 {
		t.Fatalf("recovery_low_reward_window 不符合预期: %+v", cfg)
	}
	if !cfg.CandidateAmbiguityTopGapThreshold.IsSet() || cfg.CandidateAmbiguityTopGapThreshold.Get() != 0.12 {
		t.Fatalf("candidate_ambiguity_top_gap_threshold 不符合预期: %+v", cfg)
	}
	if !cfg.HighValuePageVisitLimit.IsSet() || cfg.HighValuePageVisitLimit.Get() != 3 {
		t.Fatalf("high_value_page_visit_limit 不符合预期: %+v", cfg)
	}
	if !cfg.CandidateRiskDropThreshold.IsSet() || cfg.CandidateRiskDropThreshold.Get() != 1.9 {
		t.Fatalf("candidate_risk_drop_threshold 不符合预期: %+v", cfg)
	}
	if !cfg.CandidateMinFusionScore.IsSet() || cfg.CandidateMinFusionScore.Get() != -0.2 {
		t.Fatalf("candidate_min_fusion_score 不符合预期: %+v", cfg)
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
	if !cfg.UCTBandit.TwoStateLoopPenalty.IsSet() || cfg.UCTBandit.TwoStateLoopPenalty.Get() != -4.5 {
		t.Fatalf("two_state_loop_penalty 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.EdgeRepeatPenalty.IsSet() || cfg.UCTBandit.EdgeRepeatPenalty.Get() != -1.2 {
		t.Fatalf("edge_repeat_penalty 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.EdgeRepeatThreshold.IsSet() || cfg.UCTBandit.EdgeRepeatThreshold.Get() != 3 {
		t.Fatalf("edge_repeat_threshold 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.ActionCooldownPenalty.IsSet() || cfg.UCTBandit.ActionCooldownPenalty.Get() != 0.75 {
		t.Fatalf("action_cooldown_penalty 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.RecentActionWindow.IsSet() || cfg.UCTBandit.RecentActionWindow.Get() != 8 {
		t.Fatalf("recent_action_window 不符合预期: %+v", cfg.UCTBandit)
	}
	if !cfg.UCTBandit.LoopEscapeExploreBoost.IsSet() || cfg.UCTBandit.LoopEscapeExploreBoost.Get() != 0.2 {
		t.Fatalf("loop_escape_explore_boost 不符合预期: %+v", cfg.UCTBandit)
	}
}
