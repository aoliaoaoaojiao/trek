package monkey

import (
	"sort"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	"trek/logger"
	"trek/pkg/session"
)

func (r *Runner) recordCandidateEnhancementOutcome(step int, cmd *types.ActionCommand, outcome traversal.ActionOutcome) {
	if r == nil || cmd == nil || r.lastEnhancementAttempt == nil {
		return
	}
	attempt := r.lastEnhancementAttempt
	if attempt.step != step {
		return
	}
	defer func() {
		r.lastEnhancementAttempt = nil
	}()
	if attempt.candidate.Command == nil || attempt.candidate.Command.ToJSON() != cmd.ToJSON() {
		return
	}
	writer, ok := r.decider.(RecoveryMemoryWriter)
	if !ok || writer == nil {
		return
	}
	improved := outcome == traversal.OutcomeNewState || outcome == traversal.OutcomeEscapeBlock
	if err := writer.RecordCandidateEnhancementOutcome(attempt.ctx, attempt.candidate, improved); err != nil {
		logger.Warnf("record candidate enhancement outcome failed: improved=%t err=%v", improved, err)
	}
}

func (r *Runner) tryEnhanceCandidates(step int, beforePage session.PageSnapshot, baseCmd *types.ActionCommand, weighted []WeightedCandidate) (*types.ActionCommand, error) {
	if r == nil || baseCmd == nil {
		return nil, nil
	}
	if !r.shouldEnableExploreLLMEnhancement() {
		return nil, nil
	}
	ctx := r.buildTraversalContext(step, beforePage, nil, nil)
	ctx.LocalCandidates = summarizeWeightedCandidates(weighted, baseCmd)
	knownFailed, err := r.collectKnownFailedRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	knownSuccess, err := r.collectKnownSuccessfulRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	ctx.KnownFailedActions = actionKeyList(knownFailed)
	ctx.KnownSuccessActions = actionKeyList(knownSuccess)
	if !r.shouldTriggerCandidateEnhancement(ctx, step, baseCmd, weighted) {
		return nil, nil
	}

	llmProvider, ok := r.decider.(RecoveryCandidateProvider)
	if !ok || llmProvider == nil {
		return nil, nil
	}
	selector, ok := r.decider.(RecoveryCandidateSelector)
	if !ok || selector == nil {
		return nil, nil
	}
	if !r.allowCandidateEnhancementLLM(ctx) {
		r.enhanceLLMDeniedCount++
		return nil, nil
	}

	llmItems, err := llmProvider.BuildLLMRecoveryCandidates(ctx)
	if err != nil {
		return nil, err
	}
	r.enhancementCallCount++
	r.recordCandidateEnhancementLLMCall(ctx, step)
	if len(llmItems) == 0 {
		return nil, nil
	}

	items := make([]candidate.Candidate, 0, len(llmItems)+1)
	items = append(items, candidateFromCommand(baseCmd, candidate.SourceAlgorithm))
	items = append(items, llmItems...)
	fused := candidate.FuseCandidates(items, candidate.FusionOptions{
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
	if selected == nil {
		return nil, nil
	}
	if selected.ToJSON() == baseCmd.ToJSON() {
		return nil, nil
	}
	if chosen := findCandidateByCommand(fused, selected); chosen != nil {
		r.lastEnhancementAttempt = &enhancementAttempt{ctx: ctx, candidate: *chosen, step: step}
	} else {
		r.lastEnhancementAttempt = &enhancementAttempt{
			ctx:       ctx,
			candidate: candidateFromCommand(selected, candidate.SourceLLM),
			step:      step,
		}
	}
	r.enhancementHitCount++
	logger.Infof("candidate enhancement selected action: base=%s enhanced=%s", baseCmd.DetailLogString(), selected.DetailLogString())
	return selected, nil
}

func findCandidateByCommand(items []candidate.Candidate, cmd *types.ActionCommand) *candidate.Candidate {
	if cmd == nil {
		return nil
	}
	key := cmd.ToJSON()
	for _, item := range items {
		if item.Command == nil {
			continue
		}
		if item.Command.ToJSON() == key {
			copyItem := item
			return &copyItem
		}
	}
	return nil
}

func (r *Runner) shouldEnableExploreLLMEnhancement() bool {
	if r == nil || r.cfg.EnableExploreLLMEnhancement == nil {
		return false
	}
	return *r.cfg.EnableExploreLLMEnhancement
}

func (r *Runner) shouldTriggerCandidateEnhancement(ctx enginestate.TraversalContext, step int, baseCmd *types.ActionCommand, weighted []WeightedCandidate) bool {
	if r == nil || baseCmd == nil {
		return false
	}
	if ctx.Mode == TraversalModeRecover || ctx.Mode == TraversalModeCooldown {
		return false
	}
	if isAppRestartAction(baseCmd.Act) || baseCmd.Act == types.NOP {
		return false
	}
	if step-r.lastEnhancementStep < r.cfg.CandidateEnhancementMinStepGap {
		return false
	}
	if !r.isHighValuePage(ctx) {
		return false
	}
	return r.hasLowCandidateDistinction(ctx, weighted)
}

func (r *Runner) isHighValuePage(ctx enginestate.TraversalContext) bool {
	if ctx.Mode == TraversalModeSuspectBlocked {
		return true
	}
	visitCount := 0
	if r != nil && r.pageVisitCount != nil {
		visitCount = r.pageVisitCount[ctx.PageName]
	}
	return visitCount > 0 && visitCount <= r.cfg.HighValuePageVisitLimit
}

func (r *Runner) hasLowCandidateDistinction(ctx enginestate.TraversalContext, weighted []WeightedCandidate) bool {
	if len(weighted) == 0 {
		return ctx.Mode == TraversalModeSuspectBlocked
	}
	positive := make([]float64, 0, len(weighted))
	for _, item := range weighted {
		if item.Command == nil || item.Weight <= 0 {
			continue
		}
		positive = append(positive, item.Weight)
	}
	if len(positive) < 2 {
		return false
	}
	sort.SliceStable(positive, func(i, j int) bool { return positive[i] > positive[j] })
	total := 0.0
	for _, w := range positive {
		total += w
	}
	if total <= 0 {
		return false
	}
	top1 := positive[0] / total
	top2 := positive[1] / total
	if len(positive) >= 3 && top1 <= 0.5 {
		return true
	}
	return (top1 - top2) <= r.cfg.CandidateAmbiguityTopGapThreshold
}

func (r *Runner) allowCandidateEnhancementLLM(ctx enginestate.TraversalContext) bool {
	if r == nil {
		return false
	}
	if r.candidateEnhanceBudget == nil {
		return true
	}
	return r.candidateEnhanceBudget.Allow(ctx)
}

func (r *Runner) recordCandidateEnhancementLLMCall(ctx enginestate.TraversalContext, step int) {
	if r == nil {
		return
	}
	r.lastEnhancementStep = step
	if r.candidateEnhanceBudget != nil {
		r.candidateEnhanceBudget.Record(ctx)
	}
}
