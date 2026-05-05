package monkey

import (
	"strings"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	"trek/internal/engine/recovery"
	enginestate "trek/internal/engine/state"
	"trek/logger"
	"trek/pkg/coordinator"
)

func (r *Runner) nextCommandWithRecovery(step int, beforePage coordinator.PageSnapshot, pageName string, input coordinator.ActionInput) (*types.ActionCommand, error) {
	r.lastEnhancementAttempt = nil
	if !r.pendingBlockRecovery {
		cmd, weighted, err := r.nextCommandWithCandidates(pageName, input)
		if err != nil || cmd == nil {
			return cmd, err
		}
		ctx := r.buildTraversalContext(step, beforePage, nil, nil)
		cmd, err = r.trySelectFromTraversalCandidates(ctx, cmd, weighted)
		if err != nil {
			logger.Warnf("select traversal candidate failed, fallback to base action: %v", err)
		}
		enhanced, enhanceErr := r.tryEnhanceCandidates(step, beforePage, cmd, weighted)
		if enhanceErr != nil {
			logger.Warnf("enhance candidates failed, fallback to base action: %v", enhanceErr)
			return cmd, nil
		}
		if enhanced != nil {
			return enhanced, nil
		}
		return cmd, nil
	}
	r.pendingBlockRecovery = false
	r.lastRecoveryAttempt = nil
	cmd, err := r.nextBlockRecoveryCommand(pageName, input)
	if err != nil {
		logger.Warnf("build block recovery command failed, fallback to normal command: %v", err)
		return r.nextCommand(pageName, input)
	}
	if cmd == nil {
		logger.Warnf("block recovery command is nil, fallback to normal command")
		return r.nextCommand(pageName, input)
	}
	logger.Infof("block recovery command selected: %s", cmd.DetailLogString())
	return cmd, nil
}

func (r *Runner) trySelectFromTraversalCandidates(
	ctx enginestate.TraversalContext,
	baseCmd *types.ActionCommand,
	weighted []WeightedCandidate,
) (*types.ActionCommand, error) {
	if r == nil || baseCmd == nil {
		return baseCmd, nil
	}
	provider, ok := r.decider.(AlgorithmCandidateProvider)
	if !ok || provider == nil {
		return baseCmd, nil
	}
	selector, ok := r.decider.(RecoveryCandidateSelector)
	if !ok || selector == nil {
		return baseCmd, nil
	}

	items, err := provider.BuildAlgorithmCandidates(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return baseCmd, nil
	}
	baseItems := weightedCandidatesToAlgorithmCandidates(weighted)
	if len(baseItems) == 0 {
		baseItems = []perception.Candidate{candidateFromCommand(baseCmd, perception.SourceAlgorithm)}
	}
	items = append(items, baseItems...)

	knownFailed, err := r.collectKnownFailedRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	fused := perception.FuseCandidates(items, perception.FusionOptions{
		KnownFailedActions:   knownFailed,
		RiskDropThreshold:    r.cfg.CandidateRiskDropThreshold,
		EnableMinScoreFilter: true,
		MinScoreThreshold:    r.cfg.CandidateMinFusionScore,
		KeepTopOnFiltered:    true,
	})
	selected, err := selector.SelectRecoveryAction(ctx, fused)
	if err != nil {
		return nil, err
	}
	if selected == nil || !selected.IsValid() {
		return baseCmd, nil
	}
	return selected, nil
}

func (r *Runner) handleBlockDetected(reason string) {
	if r == nil {
		return
	}
	if r.recoveryState == nil {
		r.recoveryState = newRecoveryStateMachineWithCooldown(r.cfg.RecoveryCooldownSteps)
	}
	beforeMode := r.recoveryState.Mode()
	r.recoveryState.OnBlockDetected(reason)
	r.pendingBlockRecovery = r.recoveryState.Mode() == TraversalModeRecover
	logger.Infof("recovery state transition on block: from=%s to=%s reason=%s",
		beforeMode, r.recoveryState.Mode(), r.recoveryState.BlockReason())
}

func (r *Runner) handleProgress(escaped bool) {
	if r == nil || r.recoveryState == nil {
		return
	}
	beforeMode := r.recoveryState.Mode()
	r.recoveryState.OnProgress(escaped)
	if beforeMode != TraversalModeCooldown && r.recoveryState.Mode() == TraversalModeCooldown {
		r.cooldownEnterCount++
	}
	if beforeMode != r.recoveryState.Mode() {
		logger.Infof("recovery state transition on progress: from=%s to=%s",
			beforeMode, r.recoveryState.Mode())
	}
}

func (r *Runner) advanceRecoveryStateOnStep() {
	if r == nil || r.recoveryState == nil {
		return
	}
	beforeMode := r.recoveryState.Mode()
	if beforeMode == TraversalModeCooldown {
		r.cooldownStepCount++
	}
	r.recoveryState.OnStepAdvance()
	if beforeMode != r.recoveryState.Mode() {
		logger.Infof("recovery state transition on step: from=%s to=%s",
			beforeMode, r.recoveryState.Mode())
	}
}

func (r *Runner) buildTraversalContext(step int, page coordinator.PageSnapshot, pageVisitCount map[string]int, actionCount map[string]int) enginestate.TraversalContext {
	mode := TraversalModeExplore
	blockReason := ""
	if r != nil && r.recoveryState != nil {
		mode = r.recoveryState.Mode()
		blockReason = r.recoveryState.BlockReason()
	}
	if pageVisitCount == nil && r != nil {
		pageVisitCount = r.pageVisitCount
	}
	if actionCount == nil && r != nil {
		actionCount = r.actionCount
	}
	signature := cachedSignature(page)
	return enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:             step,
		Mode:             mode,
		PageName:         page.PageName,
		PageSignature:    signature,
		ClusterSignature: signature,
		XML:              page.XML,
		Screenshot:       page.Screenshot,
		BlockReason:      blockReason,
		RecentTrace:      r.cloneRecentTrace(),
		PageVisitCount:   pageVisitCount,
		ActionCount:      actionCount,
	})
}

func (r *Runner) recordActionTrace(page coordinator.PageSnapshot, cmd *types.ActionCommand) {
	if r == nil || cmd == nil {
		return
	}
	trace := enginestate.ActionTrace{
		PageSignature: cachedSignature(page),
		ActionKey:     commandTraceKey(cmd),
	}
	if strings.TrimSpace(trace.PageSignature) == "" || strings.TrimSpace(trace.ActionKey) == "" {
		return
	}
	r.recentTrace = append(r.recentTrace, trace)
	if len(r.recentTrace) > maxRecentTraceEntries {
		r.recentTrace = r.recentTrace[len(r.recentTrace)-maxRecentTraceEntries:]
	}
}

func (r *Runner) cloneRecentTrace() []enginestate.ActionTrace {
	if r == nil || len(r.recentTrace) == 0 {
		return nil
	}
	result := make([]enginestate.ActionTrace, len(r.recentTrace))
	copy(result, r.recentTrace)
	return result
}

func commandTraceKey(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	return cmd.Act.String()
}

func (r *Runner) nextBlockRecoveryCommand(pageName string, input coordinator.ActionInput) (*types.ActionCommand, error) {
	ctx := r.buildTraversalContext(0, coordinator.PageSnapshot{
		PageName:   pageName,
		XML:        input.XMLDescOfGuiTree,
		Screenshot: input.Screenshot,
	}, nil, nil)
	knownFailed, knownSuccess, knownErr := r.collectBothKnownActions(ctx)
	if knownErr != nil {
		return nil, knownErr
	}
	ctx.KnownFailedActions = actionKeyList(knownFailed)
	ctx.KnownSuccessActions = actionKeyList(knownSuccess)

	if planner := r.getRecoveryPlanner(); planner != nil {
		items, err := planner.BuildRecoveryCandidates(ctx)
		if err != nil {
			return nil, err
		}
		fused := perception.FuseCandidates(items, perception.FusionOptions{
			KnownFailedActions:   knownFailed,
			RiskDropThreshold:    r.cfg.CandidateRiskDropThreshold,
			EnableMinScoreFilter: true,
			MinScoreThreshold:    r.cfg.CandidateMinFusionScore,
			KeepTopOnFiltered:    true,
		})
		if selector, ok := r.decider.(RecoveryCandidateSelector); ok && selector != nil {
			selected, selectErr := selector.SelectRecoveryAction(ctx, fused)
			if selectErr != nil {
				return nil, selectErr
			}
			if selected != nil {
				r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: candidateFromCommand(selected, perception.SourceAlgorithm)}
				return selected, nil
			}
		}
		if item := firstCandidateWithCommand(fused); item != nil {
			r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: *item}
			return item.Command, nil
		}
	}

	if provider, ok := r.decider.(ContextAwareBlockRecoveryDecider); ok && provider != nil {
		cmd, err := provider.NextBlockRecoveryActionWithContext(ctx, input)
		if err != nil {
			return nil, err
		}
		if cmd != nil {
			r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: candidateFromCommand(cmd, perception.SourceHeuristic)}
			return cmd, nil
		}
	}
	fallback := &types.ActionCommand{Act: types.BACK}
	r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: candidateFromCommand(fallback, perception.SourceHeuristic)}
	return fallback, nil
}

func (r *Runner) getRecoveryPlanner() recovery.RecoveryPlanner {
	if r == nil {
		return nil
	}
	if r.recoveryPlanner != nil {
		return r.recoveryPlanner
	}

	config := recovery.PlannerConfig{}
	if provider, ok := r.decider.(RecoveryCandidateProvider); ok && provider != nil {
		config.Memory = recoveryProviderFunc(provider.BuildMemoryRecoveryCandidates)
		config.Heuristic = recoveryProviderFunc(provider.BuildHeuristicRecoveryCandidates)
	}

	if config.Memory == nil && config.Heuristic == nil && config.LLM == nil {
		return nil
	}

	r.recoveryPlanner = recovery.NewPlanner(config)
	return r.recoveryPlanner
}

func (r *Runner) recordRecoveryOutcome(escaped bool) {
	if r == nil || r.lastRecoveryAttempt == nil {
		return
	}
	attempt := *r.lastRecoveryAttempt
	defer func() {
		r.lastRecoveryAttempt = nil
	}()
	r.markRecoveryActionOutcome(attempt.item, escaped)

	writer, ok := r.decider.(RecoveryMemoryWriter)
	if !ok || writer == nil {
		return
	}
	if err := writer.RecordRecoveryMemoryOutcome(attempt.ctx, attempt.item, escaped); err != nil {
		logger.Warnf("record recovery memory outcome failed: escaped=%t err=%v", escaped, err)
	}
}

func (r *Runner) markRecoveryActionOutcome(item perception.Candidate, escaped bool) {
	if r == nil || item.Command == nil {
		return
	}
	key := item.Command.ToJSON()
	if strings.TrimSpace(key) == "" {
		return
	}
	if r.recoveryFailedAction == nil {
		r.recoveryFailedAction = make(map[string]bool)
	}
	if r.recoverySuccessAction == nil {
		r.recoverySuccessAction = make(map[string]bool)
	}
	if escaped {
		delete(r.recoveryFailedAction, key)
		r.recoverySuccessAction[key] = true
		return
	}
	r.recoveryFailedAction[key] = true
	delete(r.recoverySuccessAction, key)
}

// BatchRecoveryActionHistoryProvider 支持单次遍历同时返回失败/成功集合的可选接口。
type BatchRecoveryActionHistoryProvider interface {
	BuildKnownRecoveryActions(ctx enginestate.TraversalContext) (failed, success map[string]bool, err error)
}

func (r *Runner) collectKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	return r.collectKnownActions(r.recoveryFailedAction, ctx, func(p RecoveryActionHistoryProvider, c enginestate.TraversalContext) (map[string]bool, error) {
		return p.BuildKnownFailedRecoveryActions(c)
	})
}

func (r *Runner) collectKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	return r.collectKnownActions(r.recoverySuccessAction, ctx, func(p RecoveryActionHistoryProvider, c enginestate.TraversalContext) (map[string]bool, error) {
		return p.BuildKnownSuccessfulRecoveryActions(c)
	})
}

// collectBothKnownActions 单次遍历记忆库同时获取失败/成功集合，避免两次 Find。
func (r *Runner) collectBothKnownActions(ctx enginestate.TraversalContext) (failed, success map[string]bool, err error) {
	if batch, ok := r.decider.(BatchRecoveryActionHistoryProvider); ok && batch != nil {
		persistedFailed, persistedSuccess, err := batch.BuildKnownRecoveryActions(ctx)
		if err != nil {
			return nil, nil, err
		}
		failed = mergeBoolMaps(r.recoveryFailedAction, persistedFailed)
		success = mergeBoolMaps(r.recoverySuccessAction, persistedSuccess)
		return failed, success, nil
	}
	f, err1 := r.collectKnownFailedRecoveryActions(ctx)
	if err1 != nil {
		return nil, nil, err1
	}
	s, err2 := r.collectKnownSuccessfulRecoveryActions(ctx)
	if err2 != nil {
		return nil, nil, err2
	}
	return f, s, nil
}

func mergeBoolMaps(local, persisted map[string]bool) map[string]bool {
	result := make(map[string]bool, len(local)+len(persisted))
	for key, value := range local {
		if value {
			result[key] = true
		}
	}
	for key, value := range persisted {
		if value {
			result[key] = true
		}
	}
	return result
}

func (r *Runner) collectKnownActions(local map[string]bool, ctx enginestate.TraversalContext, fetch func(RecoveryActionHistoryProvider, enginestate.TraversalContext) (map[string]bool, error)) (map[string]bool, error) {
	known := make(map[string]bool, len(local))
	for key, value := range local {
		if value {
			known[key] = true
		}
	}
	provider, ok := r.decider.(RecoveryActionHistoryProvider)
	if !ok || provider == nil {
		return known, nil
	}
	persisted, err := fetch(provider, ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range persisted {
		if value {
			known[key] = true
		}
	}
	return known, nil
}

type recoveryProviderFunc func(ctx enginestate.TraversalContext) ([]perception.Candidate, error)

func (f recoveryProviderFunc) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	return f(ctx)
}

func (r *Runner) pickWeightedCandidate(candidates []WeightedCandidate) *types.ActionCommand {
	if len(candidates) == 0 {
		return nil
	}

	total := 0.0
	for _, c := range candidates {
		if c.Command != nil && c.Weight > 0 {
			total += c.Weight
		}
	}

	if total > 0 && r.rng != nil {
		target := r.rng.Float64() * total
		acc := 0.0
		for _, c := range candidates {
			if c.Command == nil || c.Weight <= 0 {
				continue
			}
			acc += c.Weight
			if acc >= target {
				return c.Command
			}
		}
	}

	for _, c := range candidates {
		if c.Command != nil {
			return c.Command
		}
	}
	return nil
}

func (r *Runner) shouldEnableBlockRecovery() bool {
	if r == nil {
		return false
	}
	if r.cfg.EnableBlockRecovery == nil {
		return true
	}
	return *r.cfg.EnableBlockRecovery
}
