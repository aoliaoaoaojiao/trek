package monkey

import (
	"fmt"
	"sort"
	"strings"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

func normalizeConfig(cfg Config) Config {
	cfg.DeviceSerial = strings.TrimSpace(cfg.DeviceSerial)
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = defaultMaxSteps
	}
	if cfg.MaxDuration <= 0 {
		cfg.MaxDuration = defaultMaxDuration
	}
	if cfg.StepInterval < 0 {
		cfg.StepInterval = 0
	} else if cfg.StepInterval == 0 {
		cfg.StepInterval = defaultStepInterval
	}
	if cfg.MaxConsecutiveFailures <= 0 {
		cfg.MaxConsecutiveFailures = defaultMaxConsecutiveFailures
	}
	if cfg.FailureRecoveryInterval <= 0 {
		cfg.FailureRecoveryInterval = defaultFailureRecoveryInterval
	}
	if strings.TrimSpace(cfg.PageSourceType) == "" {
		cfg.PageSourceType = defaultPageSourceType
	}
	if cfg.LongClickDuration <= 0 {
		cfg.LongClickDuration = defaultLongClickDuration
	}
	if cfg.ScrollDuration <= 0 {
		cfg.ScrollDuration = defaultScrollDuration
	}
	if cfg.ScrollSteps <= 0 {
		cfg.ScrollSteps = defaultScrollSteps
	}
	if cfg.ScrollRepeat <= 0 {
		cfg.ScrollRepeat = defaultScrollRepeat
	}
	if cfg.PageNameResolver == nil {
		cfg.PageNameResolver = defaultPageNameResolver
	}
	if cfg.ForegroundMonitorInterval <= 0 {
		cfg.ForegroundMonitorInterval = defaultForegroundMonitorInterval
	}
	if cfg.HealthSignalMonitorInterval <= 0 {
		cfg.HealthSignalMonitorInterval = defaultHealthSignalMonitorInterval
	}
	if cfg.OrientationMonitorInterval <= 0 {
		cfg.OrientationMonitorInterval = defaultOrientationMonitorInterval
	}
	if !cfg.StopOnCrash && !cfg.StopOnANR {
		cfg.StopOnCrash = true
		cfg.StopOnANR = true
	}
	if cfg.BlockNoChangeThreshold <= 0 {
		cfg.BlockNoChangeThreshold = defaultBlockNoChangeThreshold
	}
	if cfg.RecoveryCooldownSteps <= 0 {
		cfg.RecoveryCooldownSteps = defaultRecoveryCooldownSteps
	}
	if cfg.LLMBudgetMaxCalls < 0 {
		cfg.LLMBudgetMaxCalls = 0
	}
	if cfg.LLMBudgetWindowStep < 0 {
		cfg.LLMBudgetWindowStep = 0
	}
	if cfg.CandidateEnhancementMinStepGap <= 0 {
		cfg.CandidateEnhancementMinStepGap = defaultCandidateEnhancementMinStepGap
	}
	if cfg.CandidateAmbiguityTopGapThreshold <= 0 {
		cfg.CandidateAmbiguityTopGapThreshold = defaultCandidateAmbiguityTopGapThreshold
	}
	if cfg.HighValuePageVisitLimit <= 0 {
		cfg.HighValuePageVisitLimit = defaultHighValuePageVisitLimit
	}
	if cfg.TwoStateLoopThreshold <= 0 {
		cfg.TwoStateLoopThreshold = defaultTwoStateLoopThreshold
	}
	if cfg.HighVisitThreshold <= 0 {
		cfg.HighVisitThreshold = defaultHighVisitThreshold
	}
	if cfg.LowRewardWindow <= 0 {
		cfg.LowRewardWindow = defaultLowRewardWindow
	}
	if cfg.CandidateRiskDropThreshold <= 0 {
		cfg.CandidateRiskDropThreshold = defaultCandidateRiskDropThreshold
	}
	if cfg.CandidateMinFusionScore == 0 {
		cfg.CandidateMinFusionScore = defaultCandidateMinFusionScore
	}
	if len(cfg.EffectiveTouchAreas) > 0 {
		filtered := make([]EffectiveTouchArea, 0, len(cfg.EffectiveTouchAreas))
		for _, area := range cfg.EffectiveTouchAreas {
			area.Serial = strings.TrimSpace(area.Serial)
			area.PackageName = strings.TrimSpace(area.PackageName)
			if area.Range.Right <= area.Range.Left || area.Range.Bottom <= area.Range.Top {
				continue
			}
			if len(area.Orientations) == 0 {
				continue
			}
			filtered = append(filtered, area)
		}
		cfg.EffectiveTouchAreas = filtered
	}
	return cfg
}

func firstCandidateWithCommand(items []perception.Candidate) *perception.Candidate {
	for _, item := range items {
		if item.Command != nil {
			copyItem := item
			return &copyItem
		}
	}
	return nil
}

func candidateFromCommand(cmd *types.ActionCommand, source string) perception.Candidate {
	if cmd == nil {
		return perception.Candidate{Source: source}
	}
	cmdCopy := *cmd
	return perception.Candidate{
		Command: &cmdCopy,
		Source:  source,
	}
}

func weightedCandidatesToAlgorithmCandidates(weighted []WeightedCandidate) []perception.Candidate {
	if len(weighted) == 0 {
		return nil
	}
	total := 0.0
	for _, item := range weighted {
		if item.Command == nil || item.Weight <= 0 {
			continue
		}
		total += item.Weight
	}
	result := make([]perception.Candidate, 0, len(weighted))
	for _, item := range weighted {
		if item.Command == nil {
			continue
		}
		c := candidateFromCommand(item.Command, perception.SourceAlgorithm)
		if total > 0 && item.Weight > 0 {
			c.Confidence = item.Weight / total
		}
		result = append(result, c)
	}
	return result
}

func summarizeWeightedCandidates(weighted []WeightedCandidate, baseCmd *types.ActionCommand) []enginestate.CandidateSummary {
	result := make([]enginestate.CandidateSummary, 0, len(weighted)+1)
	total := 0.0
	for _, item := range weighted {
		if item.Command == nil || item.Weight <= 0 {
			continue
		}
		total += item.Weight
	}
	for _, item := range weighted {
		if item.Command == nil {
			continue
		}
		confidence := 0.0
		if total > 0 && item.Weight > 0 {
			confidence = item.Weight / total
		}
		result = append(result, enginestate.CandidateSummary{
			ActionKey:  item.Command.ToJSON(),
			ActionType: item.Command.Act.String(),
			Source:     perception.SourceAlgorithm,
			Confidence: confidence,
		})
	}
	if len(result) == 0 && baseCmd != nil {
		result = append(result, enginestate.CandidateSummary{
			ActionKey:  baseCmd.ToJSON(),
			ActionType: baseCmd.Act.String(),
			Source:     perception.SourceAlgorithm,
			Confidence: 1,
		})
	}
	return result
}

func actionKeyList(actions map[string]bool) []string {
	if len(actions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(actions))
	for key, value := range actions {
		if value && strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func isAppRestartAction(act types.ActionType) bool {
	return act == types.START || act == types.RESTART || act == types.CLEAN_RESTART
}

// ResolvePageName 使用 Runner 同款逻辑解析页面名，便于调试和外部调用。
func ResolvePageName(xml string, resolver PageNameResolver) string {
	if resolver == nil {
		resolver = defaultPageNameResolver
	}
	return resolver(xml)
}

func centerPoint(rect types.Rect) (types.Point, error) {
	if rect.IsEmpty() {
		return types.Point{}, fmt.Errorf("动作坐标为空")
	}
	p := rect.Center()
	return types.Point{X: p.X, Y: p.Y}, nil
}

func pointByRatio(rect types.Rect, rx, ry float64) types.Point {
	return types.Point{
		X: rect.Left + (rect.Right-rect.Left)*rx,
		Y: rect.Top + (rect.Bottom-rect.Top)*ry,
	}
}

func matchesEffectiveTouchScope(area EffectiveTouchArea, serial string, packageName string, orientation ScreenOrientation) bool {
	areaSerial := strings.TrimSpace(area.Serial)
	if areaSerial != "" && !strings.EqualFold(areaSerial, strings.TrimSpace(serial)) {
		return false
	}
	areaPackage := strings.TrimSpace(area.PackageName)
	if areaPackage != "" && !strings.EqualFold(areaPackage, strings.TrimSpace(packageName)) {
		return false
	}
	for _, allowed := range area.Orientations {
		if allowed == orientation {
			return true
		}
	}
	return false
}

func matchEffectiveTouchArea(areas []EffectiveTouchArea, serial string, packageName string, orientation ScreenOrientation) *EffectiveTouchArea {
	if len(areas) == 0 || orientation == "" {
		return nil
	}
	for idx := range areas {
		if matchesEffectiveTouchScope(areas[idx], serial, packageName, orientation) {
			return &areas[idx]
		}
	}
	return nil
}

func isNormalizedRect(rect types.Rect) bool {
	return rect.Left >= 0 && rect.Top >= 0 && rect.Right <= 1 && rect.Bottom <= 1
}

func mapRectToEffectiveRange(rect types.Rect, area EffectiveTouchRange) (types.Rect, bool) {
	width := area.Right - area.Left
	height := area.Bottom - area.Top
	if width <= 0 || height <= 0 {
		return rect, false
	}
	return types.Rect{
		Left:   area.Left + width*rect.Left,
		Top:    area.Top + height*rect.Top,
		Right:  area.Left + width*rect.Right,
		Bottom: area.Top + height*rect.Bottom,
	}, true
}

func isAutoStartOnRunEnabled(cfg Config) bool {
	if cfg.AutoStartOnRun == nil {
		return true
	}
	return *cfg.AutoStartOnRun
}

func isActionThrottleEnabled(cfg Config) bool {
	if cfg.ActionThrottleEnabled == nil {
		return true
	}
	return *cfg.ActionThrottleEnabled
}

func isFailureRecoveryEnabled(cfg Config) bool {
	if cfg.EnableFailureRecovery == nil {
		return true
	}
	return *cfg.EnableFailureRecovery
}
