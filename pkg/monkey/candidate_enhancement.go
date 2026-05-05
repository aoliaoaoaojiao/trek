package monkey

import (
	"sort"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	"trek/logger"
	"trek/pkg/coordinator"
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
	if attempt.item.Command == nil || !attempt.item.Command.Equal(cmd) {
		return
	}
	writer, ok := r.decider.(RecoveryMemoryWriter)
	if !ok || writer == nil {
		return
	}
	improved := outcome == traversal.OutcomeNewState || outcome == traversal.OutcomeEscapeBlock
	if err := writer.RecordCandidateEnhancementOutcome(attempt.ctx, attempt.item, improved); err != nil {
		logger.Warnf("record candidate enhancement outcome failed: improved=%t err=%v", improved, err)
	}
}

func (r *Runner) tryEnhanceCandidates(step int, beforePage coordinator.PageSnapshot, baseCmd *types.ActionCommand, weighted []WeightedCandidate) (*types.ActionCommand, error) {
	if r == nil || baseCmd == nil {
		return nil, nil
	}
	_ = step
	_ = beforePage
	_ = weighted
	return nil, nil
}

func findCandidateByCommand(items []perception.Candidate, cmd *types.ActionCommand) *perception.Candidate {
	if cmd == nil {
		return nil
	}
	for _, item := range items {
		if item.Command == nil {
			continue
		}
		if item.Command.Equal(cmd) {
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
