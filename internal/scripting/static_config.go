package scripting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	coretypes "trek/internal/engine/core/types"
)

type BlackRect struct {
	PageName string
	Bounds   [4]int
}

type StaticConfig struct {
	BlackRects                        []BlackRect
	SkipAll                           bool
	PageSource                        string
	TouchMode                         string
	PageNameStrategy                  string
	ImageFingerprintRegions           []StaticTouchRange
	ImageSimilaritySSIMThreshold      coretypes.Optional[float64]
	PageControlStrategy               string
	PageControlCacheFile              string
	PageControlCacheTTLSeconds        coretypes.Optional[int]
	Algorithm                         string
	Plugins                           []string
	CaptureScreenshot                 coretypes.Optional[bool]
	KeepStepRecords                   coretypes.Optional[bool]
	ExploreOCRTimeoutMs               coretypes.Optional[int]
	LLMTimeoutMs                      coretypes.Optional[int]
	RecoveryCooldownSteps             coretypes.Optional[int]
	RecoveryTwoStateLoopThreshold     coretypes.Optional[int]
	RecoveryHighVisitThreshold        coretypes.Optional[int]
	RecoveryLowRewardWindow           coretypes.Optional[int]
	CandidateAmbiguityTopGapThreshold coretypes.Optional[float64]
	HighValuePageVisitLimit           coretypes.Optional[int]
	CandidateRiskDropThreshold        coretypes.Optional[float64]
	CandidateMinFusionScore           coretypes.Optional[float64]
	ScrollInferThreshold              int
	UIA                               StaticUIAConfig
	Poco                              StaticPocoConfig
	Log                               StaticLogConfig
	EffectiveTouchArea                *StaticEffectiveTouchArea
	UCTBandit                         StaticUCTBanditConfig
	Reuse                             StaticReuseConfig
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
	TwoStateLoopPenalty    coretypes.Optional[float64]
	EdgeRepeatPenalty      coretypes.Optional[float64]
	EdgeRepeatThreshold    coretypes.Optional[int]
	ActionCooldownPenalty  coretypes.Optional[float64]
	RecentActionWindow     coretypes.Optional[int]
	LoopEscapeExploreBoost coretypes.Optional[float64]
}

type StaticReuseConfig struct {
	Epsilon                coretypes.Optional[float64]
	Gamma                  coretypes.Optional[float64]
	NStep                  coretypes.Optional[int]
	ModelSavePath          string
	EnableModelPersistence coretypes.Optional[bool]
	ResetModelOnStart      coretypes.Optional[bool]
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
	cfg := StaticConfig{}
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

	blackRectsValue := obj.Get("excluded_touch_areas")
	if isEmptyJSValue(blackRectsValue) {
		blackRectsValue = obj.Get("black_rects")
	}
	if !isEmptyJSValue(blackRectsValue) {
		arrObj := blackRectsValue.ToObject(vm)
		for _, key := range arrObj.Keys() {
			value := arrObj.Get(key)
			item := value.ToObject(vm)
			if item == nil {
				continue
			}
			pageName := ""
			if pageNameValue := item.Get("page_name"); !isEmptyJSValue(pageNameValue) {
				pageName = strings.TrimSpace(pageNameValue.String())
			}
			if pageName == "" {
				if pageNameValue := item.Get("pageName"); !isEmptyJSValue(pageNameValue) {
					pageName = strings.TrimSpace(pageNameValue.String())
				}
			}
			boundsValue := item.Get("bounds")
			if isEmptyJSValue(boundsValue) && pageName == "" {
				legacyBoundsObj := value.ToObject(vm)
				for _, legacyKey := range legacyBoundsObj.Keys() {
					boundsObj := legacyBoundsObj.Get(legacyKey).ToObject(vm)
					boundsKeys := boundsObj.Keys()
					if len(boundsKeys) == 0 {
						continue
					}
					targetBounds := boundsObj
					rectKeys := boundsKeys
					if len(rectKeys) != 4 {
						targetBounds = boundsObj.Get(boundsKeys[0]).ToObject(vm)
						rectKeys = targetBounds.Keys()
					}
					if len(rectKeys) != 4 {
						return cfg, fmt.Errorf("excluded_touch_areas[%s] bounds 长度必须为4", legacyKey)
					}
					var bounds [4]int
					for i, bk := range rectKeys {
						val, err := intFromJSValue(targetBounds.Get(bk))
						if err != nil {
							return cfg, fmt.Errorf("excluded_touch_areas[%s][%d] 非法: %w", legacyKey, i, err)
						}
						bounds[i] = val
					}
					cfg.BlackRects = append(cfg.BlackRects, BlackRect{PageName: legacyKey, Bounds: bounds})
				}
				continue
			}
			boundsObj := boundsValue.ToObject(vm)
			boundsKeys := boundsObj.Keys()
			if len(boundsKeys) != 4 {
				return cfg, fmt.Errorf("excluded_touch_areas[%s].bounds 长度必须为4", key)
			}
			var bounds [4]int
			for i, bk := range boundsKeys {
				val, err := intFromJSValue(boundsObj.Get(bk))
				if err != nil {
					return cfg, fmt.Errorf("excluded_touch_areas[%s].bounds[%d] 非法: %w", key, i, err)
				}
				bounds[i] = val
			}
			cfg.BlackRects = append(cfg.BlackRects, BlackRect{PageName: pageName, Bounds: bounds})
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
	if regionsValue := obj.Get("image_fingerprint_regions"); !isEmptyJSValue(regionsValue) {
		regions, err := parseStaticTouchRanges(regionsValue, vm, "image_fingerprint_regions")
		if err != nil {
			return cfg, err
		}
		cfg.ImageFingerprintRegions = regions
	}
	if regionsValue := obj.Get("imageFingerprintRegions"); len(cfg.ImageFingerprintRegions) == 0 && !isEmptyJSValue(regionsValue) {
		regions, err := parseStaticTouchRanges(regionsValue, vm, "imageFingerprintRegions")
		if err != nil {
			return cfg, err
		}
		cfg.ImageFingerprintRegions = regions
	}
	cfg.ImageSimilaritySSIMThreshold = optionalFloat(obj, "image_similarity_ssim_threshold", "imageSimilaritySSIMThreshold")
	if strategyValue := obj.Get("page_control_strategy"); !isEmptyJSValue(strategyValue) {
		cfg.PageControlStrategy = strings.TrimSpace(strategyValue.String())
	}
	if strategyValue := obj.Get("pageControlStrategy"); cfg.PageControlStrategy == "" && !isEmptyJSValue(strategyValue) {
		cfg.PageControlStrategy = strings.TrimSpace(strategyValue.String())
	}
	if cachePathValue := obj.Get("page_control_cache_file"); !isEmptyJSValue(cachePathValue) {
		cfg.PageControlCacheFile = strings.TrimSpace(cachePathValue.String())
	}
	if cachePathValue := obj.Get("pageControlCacheFile"); cfg.PageControlCacheFile == "" && !isEmptyJSValue(cachePathValue) {
		cfg.PageControlCacheFile = strings.TrimSpace(cachePathValue.String())
	}
	cfg.PageControlCacheTTLSeconds = optionalInt(obj, "page_control_cache_ttl_seconds", "pageControlCacheTTLSeconds")
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

	// 可选布尔字段
	cfg.CaptureScreenshot = optionalBool(obj, "capture_screenshot", "captureScreenshot")
	cfg.KeepStepRecords = optionalBool(obj, "keep_step_records", "keepStepRecords")

	// 可选整数字段
	cfg.ExploreOCRTimeoutMs = optionalInt(obj, "explore_ocr_timeout_ms", "exploreOcrTimeoutMs")
	cfg.LLMTimeoutMs = optionalInt(obj, "llm_timeout_ms", "recovery_llm_timeout_ms", "llmTimeoutMs", "recoveryLlmTimeoutMs")
	cfg.RecoveryCooldownSteps = optionalInt(obj, "recovery_cooldown_steps", "recoveryCooldownSteps")
	cfg.RecoveryTwoStateLoopThreshold = optionalInt(obj, "recovery_two_state_loop_threshold", "recoveryTwoStateLoopThreshold")
	cfg.RecoveryHighVisitThreshold = optionalInt(obj, "recovery_high_visit_threshold", "recoveryHighVisitThreshold")
	cfg.RecoveryLowRewardWindow = optionalInt(obj, "recovery_low_reward_window", "recoveryLowRewardWindow")
	cfg.HighValuePageVisitLimit = optionalInt(obj, "high_value_page_visit_limit", "highValuePageVisitLimit")

	// 可选浮点字段
	cfg.CandidateAmbiguityTopGapThreshold = optionalFloat(obj, "candidate_ambiguity_top_gap_threshold", "candidateAmbiguityTopGapThreshold")
	cfg.CandidateRiskDropThreshold = optionalFloat(obj, "candidate_risk_drop_threshold", "candidateRiskDropThreshold")
	cfg.CandidateMinFusionScore = optionalFloat(obj, "candidate_min_fusion_score", "candidateMinFusionScore")

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
		cfg.UCTBandit.TwoStateLoopPenalty = optionalFloatFromObj(uctBanditObj, "two_state_loop_penalty")
		cfg.UCTBandit.EdgeRepeatPenalty = optionalFloatFromObj(uctBanditObj, "edge_repeat_penalty")
		cfg.UCTBandit.EdgeRepeatThreshold = optionalIntFromObj(uctBanditObj, "edge_repeat_threshold")
		cfg.UCTBandit.ActionCooldownPenalty = optionalFloatFromObj(uctBanditObj, "action_cooldown_penalty")
		cfg.UCTBandit.RecentActionWindow = optionalIntFromObj(uctBanditObj, "recent_action_window")
		cfg.UCTBandit.LoopEscapeExploreBoost = optionalFloatFromObj(uctBanditObj, "loop_escape_explore_boost")
	}
	if reuseValue := obj.Get("reuse"); !isEmptyJSValue(reuseValue) {
		reuseObj := reuseValue.ToObject(vm)
		cfg.Reuse.Epsilon = optionalFloatFromObj(reuseObj, "epsilon")
		cfg.Reuse.Gamma = optionalFloatFromObj(reuseObj, "gamma")
		cfg.Reuse.NStep = optionalIntFromObj(reuseObj, "n_step")
		cfg.Reuse.EnableModelPersistence = optionalBoolFromObj(reuseObj, "enable_model_persistence")
		cfg.Reuse.ResetModelOnStart = optionalBoolFromObj(reuseObj, "reset_model_on_start")
		if modelSavePathValue := reuseObj.Get("model_save_path"); !isEmptyJSValue(modelSavePathValue) {
			cfg.Reuse.ModelSavePath = strings.TrimSpace(modelSavePathValue.String())
		}
		if modelSavePathValue := reuseObj.Get("modelSavePath"); cfg.Reuse.ModelSavePath == "" && !isEmptyJSValue(modelSavePathValue) {
			cfg.Reuse.ModelSavePath = strings.TrimSpace(modelSavePathValue.String())
		}
	}
	return cfg, nil
}

// optionalBool 从 JS 对象中按优先级查找布尔可选值。
func optionalBool(obj *goja.Object, keys ...string) coretypes.Optional[bool] {
	for _, key := range keys {
		if value := obj.Get(key); !isEmptyJSValue(value) {
			return coretypes.Some(value.ToBoolean())
		}
	}
	return coretypes.NoneOf[bool]()
}

// optionalInt 从 JS 对象中按优先级查找整数可选值。
func optionalInt(obj *goja.Object, keys ...string) coretypes.Optional[int] {
	for _, key := range keys {
		if value := obj.Get(key); !isEmptyJSValue(value) {
			parsed, err := intFromJSValue(value)
			if err != nil {
				continue
			}
			return coretypes.Some(parsed)
		}
	}
	return coretypes.NoneOf[int]()
}

// optionalFloat 从 JS 对象中按优先级查找浮点可选值。
func optionalFloat(obj *goja.Object, keys ...string) coretypes.Optional[float64] {
	for _, key := range keys {
		if value := obj.Get(key); !isEmptyJSValue(value) {
			parsed, err := floatFromJSValue(value)
			if err != nil {
				continue
			}
			return coretypes.Some(parsed)
		}
	}
	return coretypes.NoneOf[float64]()
}

// optionalIntFromObj 从嵌套 JS 对象中解析整数可选值（单键）。
func optionalIntFromObj(obj *goja.Object, key string) coretypes.Optional[int] {
	if value := obj.Get(key); !isEmptyJSValue(value) {
		parsed, err := intFromJSValue(value)
		if err != nil {
			return coretypes.NoneOf[int]()
		}
		return coretypes.Some(parsed)
	}
	return coretypes.NoneOf[int]()
}

// optionalFloatFromObj 从嵌套 JS 对象中解析浮点可选值（单键）。
func optionalFloatFromObj(obj *goja.Object, key string) coretypes.Optional[float64] {
	if value := obj.Get(key); !isEmptyJSValue(value) {
		parsed, err := floatFromJSValue(value)
		if err != nil {
			return coretypes.NoneOf[float64]()
		}
		return coretypes.Some(parsed)
	}
	return coretypes.NoneOf[float64]()
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

// optionalBoolFromObj 从嵌套 JS 对象中解析布尔可选值（单键）。
func optionalBoolFromObj(obj *goja.Object, key string) coretypes.Optional[bool] {
	if value := obj.Get(key); !isEmptyJSValue(value) {
		return coretypes.Some(value.ToBoolean())
	}
	return coretypes.NoneOf[bool]()
}

func parseStaticTouchRanges(value goja.Value, vm *goja.Runtime, fieldName string) ([]StaticTouchRange, error) {
	arrObj := value.ToObject(vm)
	ranges := make([]StaticTouchRange, 0, len(arrObj.Keys()))
	for _, key := range arrObj.Keys() {
		item := arrObj.Get(key)
		if isEmptyJSValue(item) {
			continue
		}
		itemObj := item.ToObject(vm)
		if itemObj == nil {
			continue
		}
		left, err := floatFromField(itemObj, "left")
		if err != nil {
			return nil, fmt.Errorf("%s[%s].left 非法: %w", fieldName, key, err)
		}
		top, err := floatFromField(itemObj, "top")
		if err != nil {
			return nil, fmt.Errorf("%s[%s].top 非法: %w", fieldName, key, err)
		}
		right, err := floatFromField(itemObj, "right")
		if err != nil {
			return nil, fmt.Errorf("%s[%s].right 非法: %w", fieldName, key, err)
		}
		bottom, err := floatFromField(itemObj, "bottom")
		if err != nil {
			return nil, fmt.Errorf("%s[%s].bottom 非法: %w", fieldName, key, err)
		}
		r := StaticTouchRange{Left: left, Top: top, Right: right, Bottom: bottom}
		if r.Left < 0 || r.Left > 1 || r.Top < 0 || r.Top > 1 || r.Right < 0 || r.Right > 1 || r.Bottom < 0 || r.Bottom > 1 {
			return nil, fmt.Errorf("%s[%s] 必须在 0~1 范围内", fieldName, key)
		}
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return nil, fmt.Errorf("%s[%s] 要求 right>left 且 bottom>top", fieldName, key)
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

func floatFromField(obj *goja.Object, key string) (float64, error) {
	value := obj.Get(key)
	if isEmptyJSValue(value) {
		return 0, errors.New("值不能为空")
	}
	return floatFromJSValue(value)
}
