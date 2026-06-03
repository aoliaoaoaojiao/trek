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

	// 直接 LLM 规划（重复阻塞检测触发）
	if r.pendingDirectLLMPlan {
		r.pendingDirectLLMPlan = false
		r.directLLMUsed = true
		cmd, err := r.nextDirectLLMPlanningCommand(step, beforePage, pageName, input)
		if err != nil {
			logger.Warnf("direct LLM planning failed, fallback to normal recovery: %v", err)
		} else if cmd != nil {
			logger.Infof("direct LLM planning command selected: %s", cmd.DetailLogString())
			return cmd, nil
		}
		// fallback 到正常恢复流程
	}

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
	r.handleBlockDetectedWithPage(reason, nil)
}

func (r *Runner) handleBlockDetectedWithPage(reason string, page *coordinator.PageSnapshot) {
	if r == nil {
		return
	}
	if page != nil && shouldInvalidatePageControlCacheOnBlock(reason) {
		r.invalidatePageControlCache(page.Screenshot)
		// 消费标记：恢复周期内跳过已消费的缓存条目，强制重新识别
		r.markCacheConsumed(page.Screenshot)
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

func shouldInvalidatePageControlCacheOnBlock(reason string) bool {
	switch strings.TrimSpace(reason) {
	case blockReasonSameActionNoChange, blockReasonSamePageNoChange:
		return true
	default:
		return false
	}
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
	// 脱离 Recover 模式时重置 BACK 尝试标记
	if beforeMode == TraversalModeRecover && r.recoveryState.Mode() != TraversalModeRecover {
		r.recoveryTriedBack = false
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

// cloneRecentTrace 返回最近操作轨迹的快照副本。
// 由于 recentTrace 最多保留 maxRecentTraceEntries（8）条，开销很小。
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

	// 阻塞恢复优先尝试返回键：简单、通用，能快速脱离多数卡死页面。
	// 如果返回后页面仍未变化，block detector 会再次触发，下一轮走 planner。
	if !r.recoveryTriedBack {
		r.recoveryTriedBack = true
		fallback := &types.ActionCommand{Act: types.BACK}
		r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: candidateFromCommand(fallback, perception.SourceHeuristic)}
		logger.Infof("block recovery: trying BACK")
		return fallback, nil
	}

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

// nextDirectLLMPlanningCommand 在重复阻塞时综合 Memory + Heuristic + LLM 规划。
// 将最近 2n 步的执行历史注入 TraversalContext，复用 Planner 并发获取 Memory + Heuristic 候选。
func (r *Runner) nextDirectLLMPlanningCommand(step int, beforePage coordinator.PageSnapshot, pageName string, input coordinator.ActionInput) (*types.ActionCommand, error) {
	history := r.collectStepContextHistory(r.directLLMHistorySteps)
	execHistory := r.buildExecutionHistory(history)
	ctx := r.buildTraversalContextWithHistory(step, beforePage, execHistory)

	provider, ok := r.decider.(RecoveryCandidateProvider)
	if !ok || provider == nil {
		return nil, nil
	}

	// 收集已知失败/成功动作
	knownFailed, knownSuccess, knownErr := r.collectBothKnownActions(ctx)
	if knownErr != nil {
		return nil, knownErr
	}
	ctx.KnownFailedActions = actionKeyList(knownFailed)
	ctx.KnownSuccessActions = actionKeyList(knownSuccess)

	// 复用 Planner 并发获取 Memory + Heuristic 候选（不含 LLM）
	planner := r.getRecoveryPlanner()
	if planner == nil {
		return nil, nil
	}
	items, err := planner.BuildRecoveryCandidates(ctx)
	if err != nil {
		return nil, err
	}

	// 手动调用 LLM（Planner 不配置 LLM，由调用方决定是否调用）
	if !hasHighConfidenceCandidate(items) {
		if llmItems, llmErr := provider.BuildLLMRecoveryCandidates(ctx); llmErr == nil {
			items = append(items, llmItems...)
		}
	}

	if len(items) == 0 {
		return nil, nil
	}

	// 融合候选：排除已知失败、降低风险分数
	fused := perception.FuseCandidates(items, perception.FusionOptions{
		KnownFailedActions:   knownFailed,
		RiskDropThreshold:    r.cfg.CandidateRiskDropThreshold,
		EnableMinScoreFilter: true,
		MinScoreThreshold:    r.cfg.CandidateMinFusionScore,
		KeepTopOnFiltered:    true,
	})

	// 通过算法选择最佳候选
	if selector, ok := r.decider.(RecoveryCandidateSelector); ok && selector != nil {
		selected, selectErr := selector.SelectRecoveryAction(ctx, fused)
		if selectErr == nil && selected != nil {
			r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: candidateFromCommand(selected, perception.SourceLLM)}
			return selected, nil
		}
	}
	// 回退到第一个有效候选
	if item := firstCandidateWithCommand(fused); item != nil {
		r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, item: *item}
		return item.Command, nil
	}
	return nil, nil
}

// hasHighConfidenceCandidate 检查候选列表中是否有高置信度的 Memory 候选。
func hasHighConfidenceCandidate(items []perception.Candidate) bool {
	const highConfidenceThreshold = 0.9
	for _, item := range items {
		if item.Source == perception.SourceMemory && item.Confidence >= highConfidenceThreshold {
			return true
		}
	}
	return false
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

	// 恢复失败时失效 LLM 响应缓存，下次同页面+同阻塞原因重新调用 LLM
	if !escaped && attempt.ctx.PageSignature != "" {
		r.invalidatePlanCache(attempt.ctx.PageSignature)
	}

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
// 优先使用 BatchRecoveryActionHistoryProvider 接口单次遍历获取；
// 回退路径合并本地 map 后分别查询，减少 map 分配次数。
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
	// 回退路径：合并本地 map 后分别查询持久层
	knownFailed, err := r.collectKnownActions(r.recoveryFailedAction, ctx, func(p RecoveryActionHistoryProvider, c enginestate.TraversalContext) (map[string]bool, error) {
		return p.BuildKnownFailedRecoveryActions(c)
	})
	if err != nil {
		return nil, nil, err
	}
	knownSuccess, err := r.collectKnownActions(r.recoverySuccessAction, ctx, func(p RecoveryActionHistoryProvider, c enginestate.TraversalContext) (map[string]bool, error) {
		return p.BuildKnownSuccessfulRecoveryActions(c)
	})
	if err != nil {
		return nil, nil, err
	}
	return knownFailed, knownSuccess, nil
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
