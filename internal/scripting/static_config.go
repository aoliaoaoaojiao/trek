package scripting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

type StaticConfig struct {
	ResMapping                           map[string]string
	BlackRects                           map[string][][4]int
	SkipAll                              bool
	PageSource                           string
	TouchMode                            string
	PageNameStrategy                     string
	Algorithm                            string
	Plugins                              []string
	CaptureScreenshot                    bool
	HasCaptureScreenshot                 bool
	KeepStepRecords                      bool
	HasKeepStepRecords                   bool
	ExploreOCRTimeoutMs                  int
	HasExploreOCRTimeout                 bool
	RecoveryLLMTimeoutMs                 int
	HasRecoveryLLMTimeout                bool
	RecoveryCooldownSteps                int
	HasRecoveryCooldownSteps             bool
	RecoveryLLMMaxCalls                  int
	HasRecoveryLLMMaxCalls               bool
	RecoveryLLMWindowSteps               int
	HasRecoveryLLMWindowSteps            bool
	RecoveryTwoStateLoopThreshold        int
	HasRecoveryTwoStateLoopThreshold     bool
	RecoveryHighVisitThreshold           int
	HasRecoveryHighVisitThreshold        bool
	RecoveryLowRewardWindow              int
	HasRecoveryLowRewardWindow           bool
	CandidateAmbiguityTopGapThreshold    float64
	HasCandidateAmbiguityTopGapThreshold bool
	HighValuePageVisitLimit              int
	HasHighValuePageVisitLimit           bool
	CandidateRiskDropThreshold           float64
	HasCandidateRiskDropThreshold        bool
	CandidateMinFusionScore              float64
	HasCandidateMinFusionScore           bool
	ScrollInferThreshold                 int
	UIA                                  StaticUIAConfig
	Poco                                 StaticPocoConfig
	Log                                  StaticLogConfig
	EffectiveTouchArea                   *StaticEffectiveTouchArea
	UCTBandit                            StaticUCTBanditConfig
}

type StaticLogConfig struct {
	FileLevel string
}

type StaticUIAConfig struct {
	ServerPort int
}

type StaticPocoConfig struct {
	Engine string
	Port   int
}

type StaticEffectiveTouchArea struct {
	Serial      string
	PackageName string
	Range       StaticTouchRange
}

type StaticTouchRange struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

type StaticUCTBanditConfig struct {
	TwoStateLoopPenalty       float64
	HasTwoStateLoopPenalty    bool
	EdgeRepeatPenalty         float64
	HasEdgeRepeatPenalty      bool
	EdgeRepeatThreshold       int
	HasEdgeRepeatThreshold    bool
	ActionCooldownPenalty     float64
	HasActionCooldownPenalty  bool
	RecentActionWindow        int
	HasRecentActionWindow     bool
	LoopEscapeExploreBoost    float64
	HasLoopEscapeExploreBoost bool
}

func LoadStaticConfigFile(path string) (StaticConfig, error) {
	if strings.ToLower(filepath.Ext(path)) != ".js" {
		return StaticConfig{}, fmt.Errorf("脚本配置仅支持 .js: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StaticConfig{}, err
	}
	return LoadStaticConfig(string(data))
}

func LoadStaticConfig(source string) (StaticConfig, error) {
	cfg := StaticConfig{
		ResMapping: make(map[string]string),
		BlackRects: make(map[string][][4]int),
	}
	vm := goja.New()
	if _, err := vm.RunString(source); err != nil {
		return cfg, fmt.Errorf("执行 goja 配置脚本失败: %w", err)
	}

	value := vm.Get("config")
	if isEmptyJSValue(value) {
		value = vm.Get("CONFIG")
	}
	if isEmptyJSValue(value) {
		return cfg, nil
	}
	obj := value.ToObject(vm)

	if resMappingValue := obj.Get("res_mapping"); !isEmptyJSValue(resMappingValue) {
		resMappingObj := resMappingValue.ToObject(vm)
		for _, key := range resMappingObj.Keys() {
			cfg.ResMapping[key] = resMappingObj.Get(key).String()
		}
	}

	if blackRectsValue := obj.Get("black_rects"); !isEmptyJSValue(blackRectsValue) {
		blackRectsObj := blackRectsValue.ToObject(vm)
		for _, pageName := range blackRectsObj.Keys() {
			rectsObj := blackRectsObj.Get(pageName).ToObject(vm)
			rects := make([][4]int, 0, len(rectsObj.Keys()))
			for _, rectKey := range rectsObj.Keys() {
				rectObj := rectsObj.Get(rectKey).ToObject(vm)
				rectKeys := rectObj.Keys()
				if len(rectKeys) != 4 {
					return cfg, fmt.Errorf("black_rects[%s][%s] 长度必须为4", pageName, rectKey)
				}
				var rect [4]int
				for i, key := range rectKeys {
					val, err := intFromJSValue(rectObj.Get(key))
					if err != nil {
						return cfg, fmt.Errorf("black_rects[%s][%s][%d] 非法: %w", pageName, rectKey, i, err)
					}
					rect[i] = val
				}
				rects = append(rects, rect)
			}
			cfg.BlackRects[pageName] = rects
		}
	}

	if skipValue := obj.Get("skip_all_actions_from_model"); !isEmptyJSValue(skipValue) {
		cfg.SkipAll = skipValue.ToBoolean()
	}
	if pageSourceValue := obj.Get("page_source"); !isEmptyJSValue(pageSourceValue) {
		cfg.PageSource = strings.TrimSpace(pageSourceValue.String())
	}
	if pageSourceValue := obj.Get("pageSource"); cfg.PageSource == "" && !isEmptyJSValue(pageSourceValue) {
		cfg.PageSource = strings.TrimSpace(pageSourceValue.String())
	}
	if touchModeValue := obj.Get("touch_mode"); !isEmptyJSValue(touchModeValue) {
		cfg.TouchMode = strings.TrimSpace(touchModeValue.String())
	}
	if touchModeValue := obj.Get("touchMode"); cfg.TouchMode == "" && !isEmptyJSValue(touchModeValue) {
		cfg.TouchMode = strings.TrimSpace(touchModeValue.String())
	}
	if strategyValue := obj.Get("page_name_strategy"); !isEmptyJSValue(strategyValue) {
		cfg.PageNameStrategy = strings.TrimSpace(strategyValue.String())
	}
	if strategyValue := obj.Get("pageNameStrategy"); cfg.PageNameStrategy == "" && !isEmptyJSValue(strategyValue) {
		cfg.PageNameStrategy = strings.TrimSpace(strategyValue.String())
	}
	if algorithmValue := obj.Get("algorithm"); !isEmptyJSValue(algorithmValue) {
		cfg.Algorithm = strings.TrimSpace(algorithmValue.String())
	}
	if pluginsValue := obj.Get("plugins"); !isEmptyJSValue(pluginsValue) {
		pluginsObj := pluginsValue.ToObject(vm)
		plugins := make([]string, 0, len(pluginsObj.Keys()))
		for _, key := range pluginsObj.Keys() {
			text := strings.TrimSpace(pluginsObj.Get(key).String())
			if text == "" {
				continue
			}
			plugins = append(plugins, text)
		}
		cfg.Plugins = plugins
	}
	if captureValue := obj.Get("capture_screenshot"); !isEmptyJSValue(captureValue) {
		cfg.CaptureScreenshot = captureValue.ToBoolean()
		cfg.HasCaptureScreenshot = true
	}
	if captureValue := obj.Get("captureScreenshot"); !cfg.HasCaptureScreenshot && !isEmptyJSValue(captureValue) {
		cfg.CaptureScreenshot = captureValue.ToBoolean()
		cfg.HasCaptureScreenshot = true
	}
	if keepValue := obj.Get("keep_step_records"); !isEmptyJSValue(keepValue) {
		cfg.KeepStepRecords = keepValue.ToBoolean()
		cfg.HasKeepStepRecords = true
	}
	if keepValue := obj.Get("keepStepRecords"); !cfg.HasKeepStepRecords && !isEmptyJSValue(keepValue) {
		cfg.KeepStepRecords = keepValue.ToBoolean()
		cfg.HasKeepStepRecords = true
	}
	if value := obj.Get("explore_ocr_timeout_ms"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("explore_ocr_timeout_ms 非法: %w", err)
		}
		cfg.ExploreOCRTimeoutMs = parsed
		cfg.HasExploreOCRTimeout = true
	}
	if value := obj.Get("exploreOcrTimeoutMs"); !cfg.HasExploreOCRTimeout && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("exploreOcrTimeoutMs 非法: %w", err)
		}
		cfg.ExploreOCRTimeoutMs = parsed
		cfg.HasExploreOCRTimeout = true
	}
	if value := obj.Get("recovery_llm_timeout_ms"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_llm_timeout_ms 非法: %w", err)
		}
		cfg.RecoveryLLMTimeoutMs = parsed
		cfg.HasRecoveryLLMTimeout = true
	}
	if value := obj.Get("recoveryLlmTimeoutMs"); !cfg.HasRecoveryLLMTimeout && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryLlmTimeoutMs 非法: %w", err)
		}
		cfg.RecoveryLLMTimeoutMs = parsed
		cfg.HasRecoveryLLMTimeout = true
	}
	if value := obj.Get("recovery_cooldown_steps"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_cooldown_steps 非法: %w", err)
		}
		cfg.RecoveryCooldownSteps = parsed
		cfg.HasRecoveryCooldownSteps = true
	}
	if value := obj.Get("recoveryCooldownSteps"); !cfg.HasRecoveryCooldownSteps && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryCooldownSteps 非法: %w", err)
		}
		cfg.RecoveryCooldownSteps = parsed
		cfg.HasRecoveryCooldownSteps = true
	}
	if value := obj.Get("recovery_llm_max_calls"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_llm_max_calls 非法: %w", err)
		}
		cfg.RecoveryLLMMaxCalls = parsed
		cfg.HasRecoveryLLMMaxCalls = true
	}
	if value := obj.Get("recoveryLlmMaxCalls"); !cfg.HasRecoveryLLMMaxCalls && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryLlmMaxCalls 非法: %w", err)
		}
		cfg.RecoveryLLMMaxCalls = parsed
		cfg.HasRecoveryLLMMaxCalls = true
	}
	if value := obj.Get("recovery_llm_window_steps"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_llm_window_steps 非法: %w", err)
		}
		cfg.RecoveryLLMWindowSteps = parsed
		cfg.HasRecoveryLLMWindowSteps = true
	}
	if value := obj.Get("recoveryLlmWindowSteps"); !cfg.HasRecoveryLLMWindowSteps && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryLlmWindowSteps 非法: %w", err)
		}
		cfg.RecoveryLLMWindowSteps = parsed
		cfg.HasRecoveryLLMWindowSteps = true
	}
	if value := obj.Get("recovery_two_state_loop_threshold"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_two_state_loop_threshold 非法: %w", err)
		}
		cfg.RecoveryTwoStateLoopThreshold = parsed
		cfg.HasRecoveryTwoStateLoopThreshold = true
	}
	if value := obj.Get("recoveryTwoStateLoopThreshold"); !cfg.HasRecoveryTwoStateLoopThreshold && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryTwoStateLoopThreshold 非法: %w", err)
		}
		cfg.RecoveryTwoStateLoopThreshold = parsed
		cfg.HasRecoveryTwoStateLoopThreshold = true
	}
	if value := obj.Get("recovery_high_visit_threshold"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_high_visit_threshold 非法: %w", err)
		}
		cfg.RecoveryHighVisitThreshold = parsed
		cfg.HasRecoveryHighVisitThreshold = true
	}
	if value := obj.Get("recoveryHighVisitThreshold"); !cfg.HasRecoveryHighVisitThreshold && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryHighVisitThreshold 非法: %w", err)
		}
		cfg.RecoveryHighVisitThreshold = parsed
		cfg.HasRecoveryHighVisitThreshold = true
	}
	if value := obj.Get("recovery_low_reward_window"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recovery_low_reward_window 非法: %w", err)
		}
		cfg.RecoveryLowRewardWindow = parsed
		cfg.HasRecoveryLowRewardWindow = true
	}
	if value := obj.Get("recoveryLowRewardWindow"); !cfg.HasRecoveryLowRewardWindow && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("recoveryLowRewardWindow 非法: %w", err)
		}
		cfg.RecoveryLowRewardWindow = parsed
		cfg.HasRecoveryLowRewardWindow = true
	}
	if value := obj.Get("candidate_ambiguity_top_gap_threshold"); !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("candidate_ambiguity_top_gap_threshold 非法: %w", err)
		}
		cfg.CandidateAmbiguityTopGapThreshold = parsed
		cfg.HasCandidateAmbiguityTopGapThreshold = true
	}
	if value := obj.Get("candidateAmbiguityTopGapThreshold"); !cfg.HasCandidateAmbiguityTopGapThreshold && !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("candidateAmbiguityTopGapThreshold 非法: %w", err)
		}
		cfg.CandidateAmbiguityTopGapThreshold = parsed
		cfg.HasCandidateAmbiguityTopGapThreshold = true
	}
	if value := obj.Get("high_value_page_visit_limit"); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("high_value_page_visit_limit 非法: %w", err)
		}
		cfg.HighValuePageVisitLimit = parsed
		cfg.HasHighValuePageVisitLimit = true
	}
	if value := obj.Get("highValuePageVisitLimit"); !cfg.HasHighValuePageVisitLimit && !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("highValuePageVisitLimit 非法: %w", err)
		}
		cfg.HighValuePageVisitLimit = parsed
		cfg.HasHighValuePageVisitLimit = true
	}
	if value := obj.Get("candidate_risk_drop_threshold"); !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("candidate_risk_drop_threshold 非法: %w", err)
		}
		cfg.CandidateRiskDropThreshold = parsed
		cfg.HasCandidateRiskDropThreshold = true
	}
	if value := obj.Get("candidateRiskDropThreshold"); !cfg.HasCandidateRiskDropThreshold && !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("candidateRiskDropThreshold 非法: %w", err)
		}
		cfg.CandidateRiskDropThreshold = parsed
		cfg.HasCandidateRiskDropThreshold = true
	}
	if value := obj.Get("candidate_min_fusion_score"); !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("candidate_min_fusion_score 非法: %w", err)
		}
		cfg.CandidateMinFusionScore = parsed
		cfg.HasCandidateMinFusionScore = true
	}
	if value := obj.Get("candidateMinFusionScore"); !cfg.HasCandidateMinFusionScore && !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return cfg, fmt.Errorf("candidateMinFusionScore 非法: %w", err)
		}
		cfg.CandidateMinFusionScore = parsed
		cfg.HasCandidateMinFusionScore = true
	}
	if scrollInferValue := obj.Get("scroll_infer_threshold"); !isEmptyJSValue(scrollInferValue) {
		threshold, err := intFromJSValue(scrollInferValue)
		if err != nil {
			return cfg, fmt.Errorf("scroll_infer_threshold 非法: %w", err)
		}
		cfg.ScrollInferThreshold = threshold
	}
	if uiaValue := obj.Get("uia"); !isEmptyJSValue(uiaValue) {
		uiaObj := uiaValue.ToObject(vm)
		if serverPortValue := uiaObj.Get("server_port"); !isEmptyJSValue(serverPortValue) {
			serverPort, err := intFromJSValue(serverPortValue)
			if err != nil {
				return cfg, fmt.Errorf("uia.server_port 非法: %w", err)
			}
			cfg.UIA.ServerPort = serverPort
		}
		if serverPortValue := uiaObj.Get("serverPort"); cfg.UIA.ServerPort == 0 && !isEmptyJSValue(serverPortValue) {
			serverPort, err := intFromJSValue(serverPortValue)
			if err != nil {
				return cfg, fmt.Errorf("uia.serverPort 非法: %w", err)
			}
			cfg.UIA.ServerPort = serverPort
		}
	}
	if pocoValue := obj.Get("poco"); !isEmptyJSValue(pocoValue) {
		pocoObj := pocoValue.ToObject(vm)
		if engineValue := pocoObj.Get("engine"); !isEmptyJSValue(engineValue) {
			cfg.Poco.Engine = strings.TrimSpace(engineValue.String())
		}
		if portValue := pocoObj.Get("port"); !isEmptyJSValue(portValue) {
			port, err := intFromJSValue(portValue)
			if err != nil {
				return cfg, fmt.Errorf("poco.port 非法: %w", err)
			}
			cfg.Poco.Port = port
		}
	}
	if logValue := obj.Get("log"); !isEmptyJSValue(logValue) {
		logObj := logValue.ToObject(vm)
		if fileLevelValue := logObj.Get("file_level"); !isEmptyJSValue(fileLevelValue) {
			cfg.Log.FileLevel = strings.TrimSpace(fileLevelValue.String())
		}
		if fileLevelValue := logObj.Get("fileLevel"); cfg.Log.FileLevel == "" && !isEmptyJSValue(fileLevelValue) {
			cfg.Log.FileLevel = strings.TrimSpace(fileLevelValue.String())
		}
	}
	if areaValue := obj.Get("effective_touch_area"); !isEmptyJSValue(areaValue) {
		areaObj := areaValue.ToObject(vm)
		area := &StaticEffectiveTouchArea{
			Range: StaticTouchRange{Left: 0, Top: 0, Right: 1, Bottom: 1},
		}
		if serialValue := areaObj.Get("serial"); !isEmptyJSValue(serialValue) {
			area.Serial = strings.TrimSpace(serialValue.String())
		}
		if packageValue := areaObj.Get("package_name"); !isEmptyJSValue(packageValue) {
			area.PackageName = strings.TrimSpace(packageValue.String())
		}
		if packageValue := areaObj.Get("package"); area.PackageName == "" && !isEmptyJSValue(packageValue) {
			area.PackageName = strings.TrimSpace(packageValue.String())
		}
		if rangeValue := areaObj.Get("range"); !isEmptyJSValue(rangeValue) {
			rangeObj := rangeValue.ToObject(vm)
			left, err := floatFromJSValue(rangeObj.Get("left"))
			if err != nil {
				return cfg, fmt.Errorf("effective_touch_area.range.left 非法: %w", err)
			}
			top, err := floatFromJSValue(rangeObj.Get("top"))
			if err != nil {
				return cfg, fmt.Errorf("effective_touch_area.range.top 非法: %w", err)
			}
			right, err := floatFromJSValue(rangeObj.Get("right"))
			if err != nil {
				return cfg, fmt.Errorf("effective_touch_area.range.right 非法: %w", err)
			}
			bottom, err := floatFromJSValue(rangeObj.Get("bottom"))
			if err != nil {
				return cfg, fmt.Errorf("effective_touch_area.range.bottom 非法: %w", err)
			}
			area.Range = StaticTouchRange{
				Left:   left,
				Top:    top,
				Right:  right,
				Bottom: bottom,
			}
		}
		if area.Range.Left < 0 || area.Range.Top < 0 || area.Range.Right > 1 || area.Range.Bottom > 1 {
			return cfg, fmt.Errorf("effective_touch_area.range 必须在 0~1 范围内")
		}
		if area.Range.Right <= area.Range.Left || area.Range.Bottom <= area.Range.Top {
			return cfg, fmt.Errorf("effective_touch_area.range 要求 right>left 且 bottom>top")
		}
		cfg.EffectiveTouchArea = area
	}
	if uctBanditValue := obj.Get("uct_bandit"); !isEmptyJSValue(uctBanditValue) {
		uctBanditObj := uctBanditValue.ToObject(vm)
		if value := uctBanditObj.Get("two_state_loop_penalty"); !isEmptyJSValue(value) {
			parsed, err := floatFromJSValue(value)
			if err != nil {
				return cfg, fmt.Errorf("uct_bandit.two_state_loop_penalty 非法: %w", err)
			}
			cfg.UCTBandit.TwoStateLoopPenalty = parsed
			cfg.UCTBandit.HasTwoStateLoopPenalty = true
		}
		if value := uctBanditObj.Get("edge_repeat_penalty"); !isEmptyJSValue(value) {
			parsed, err := floatFromJSValue(value)
			if err != nil {
				return cfg, fmt.Errorf("uct_bandit.edge_repeat_penalty 非法: %w", err)
			}
			cfg.UCTBandit.EdgeRepeatPenalty = parsed
			cfg.UCTBandit.HasEdgeRepeatPenalty = true
		}
		if value := uctBanditObj.Get("edge_repeat_threshold"); !isEmptyJSValue(value) {
			parsed, err := intFromJSValue(value)
			if err != nil {
				return cfg, fmt.Errorf("uct_bandit.edge_repeat_threshold 非法: %w", err)
			}
			cfg.UCTBandit.EdgeRepeatThreshold = parsed
			cfg.UCTBandit.HasEdgeRepeatThreshold = true
		}
		if value := uctBanditObj.Get("action_cooldown_penalty"); !isEmptyJSValue(value) {
			parsed, err := floatFromJSValue(value)
			if err != nil {
				return cfg, fmt.Errorf("uct_bandit.action_cooldown_penalty 非法: %w", err)
			}
			cfg.UCTBandit.ActionCooldownPenalty = parsed
			cfg.UCTBandit.HasActionCooldownPenalty = true
		}
		if value := uctBanditObj.Get("recent_action_window"); !isEmptyJSValue(value) {
			parsed, err := intFromJSValue(value)
			if err != nil {
				return cfg, fmt.Errorf("uct_bandit.recent_action_window 非法: %w", err)
			}
			cfg.UCTBandit.RecentActionWindow = parsed
			cfg.UCTBandit.HasRecentActionWindow = true
		}
		if value := uctBanditObj.Get("loop_escape_explore_boost"); !isEmptyJSValue(value) {
			parsed, err := floatFromJSValue(value)
			if err != nil {
				return cfg, fmt.Errorf("uct_bandit.loop_escape_explore_boost 非法: %w", err)
			}
			cfg.UCTBandit.LoopEscapeExploreBoost = parsed
			cfg.UCTBandit.HasLoopEscapeExploreBoost = true
		}
	}
	return cfg, nil
}

func intFromJSValue(v goja.Value) (int, error) {
	if isEmptyJSValue(v) {
		return 0, errors.New("值不能为空")
	}
	f := v.ToFloat()
	if f != float64(int(f)) {
		return 0, fmt.Errorf("值必须是整数: %v", f)
	}
	return int(f), nil
}

func floatFromJSValue(v goja.Value) (float64, error) {
	if isEmptyJSValue(v) {
		return 0, errors.New("值不能为空")
	}
	f := v.ToFloat()
	if f != f {
		return 0, errors.New("值不能为 NaN")
	}
	return f, nil
}
