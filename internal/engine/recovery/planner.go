package recovery

import (
	"sync"

	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

// Planner 负责按统一顺序组装恢复候选。
type Planner struct {
	config PlannerConfig
}

// NewPlanner 创建恢复规划器。
func NewPlanner(config PlannerConfig) *Planner {
	if config.HighConfidenceThreshold <= 0 {
		config.HighConfidenceThreshold = defaultHighConfidenceThreshold
	}
	return &Planner{config: config}
}

// BuildRecoveryCandidates 按 Memory + Heuristic（并发）-> LLM 顺序汇总恢复候选。
func (p *Planner) BuildRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	items := make([]perception.Candidate, 0, 8)

	// 并发获取 Memory 和 Heuristic 候选
	var (
		memoryItems     []perception.Candidate
		heuristicItems  []perception.Candidate
		memoryErr       error
		heuristicErr    error
		wg              sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		memoryItems, memoryErr = p.buildFromProvider(p.config.Memory, ctx)
	}()
	go func() {
		defer wg.Done()
		heuristicItems, heuristicErr = p.buildFromProvider(p.config.Heuristic, ctx)
	}()
	wg.Wait()

	if memoryErr != nil {
		return nil, memoryErr
	}
	if heuristicErr != nil {
		return nil, heuristicErr
	}
	items = append(items, memoryItems...)
	items = append(items, heuristicItems...)

	if p.hasHighConfidenceMemory(memoryItems) {
		return items, nil
	}

	if !p.allowLLM(ctx) {
		return items, nil
	}
	llmCtx := enrichLLMContext(ctx, items)
	llmItems, err := p.buildFromProvider(p.config.LLM, llmCtx)
	if err != nil {
		return nil, err
	}
	p.recordLLMCall(llmCtx)
	items = append(items, llmItems...)
	return items, nil
}

func (p *Planner) buildFromProvider(provider CandidateProvider, ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if provider == nil {
		return nil, nil
	}
	return provider.BuildCandidates(ctx)
}

func (p *Planner) hasHighConfidenceMemory(items []perception.Candidate) bool {
	for _, item := range items {
		if item.Source == perception.SourceMemory && item.Confidence >= p.config.HighConfidenceThreshold {
			return true
		}
	}
	return false
}

func (p *Planner) allowLLM(ctx enginestate.TraversalContext) bool {
	if p == nil || p.config.LLM == nil {
		return false
	}
	if p.config.LLMBudget == nil {
		return true
	}
	allowed := p.config.LLMBudget.Allow(ctx)
	if !allowed && p.config.OnLLMBudgetDenied != nil {
		p.config.OnLLMBudgetDenied(ctx)
	}
	return allowed
}

func (p *Planner) recordLLMCall(ctx enginestate.TraversalContext) {
	if p != nil && p.config.OnLLMCall != nil {
		p.config.OnLLMCall(ctx)
	}
	if p == nil || p.config.LLMBudget == nil {
		return
	}
	p.config.LLMBudget.Record(ctx)
}

func enrichLLMContext(ctx enginestate.TraversalContext, localCandidates []perception.Candidate) enginestate.TraversalContext {
	next := ctx
	if len(localCandidates) == 0 {
		return next
	}
	summaries := make([]enginestate.CandidateSummary, 0, len(ctx.LocalCandidates)+len(localCandidates))
	if len(ctx.LocalCandidates) > 0 {
		summaries = append(summaries, ctx.LocalCandidates...)
	}
	for _, item := range localCandidates {
		if item.Command == nil {
			continue
		}
		summaries = append(summaries, enginestate.CandidateSummary{
			ActionKey:   item.Command.ToJSON(),
			ActionType:  item.Command.Act.String(),
			Source:      item.Source,
			Intent:      item.Intent,
			Confidence:  item.Confidence,
			EscapeScore: item.EscapeScore,
			RiskScore:   item.RiskScore,
		})
	}
	next.LocalCandidates = summaries
	return next
}
