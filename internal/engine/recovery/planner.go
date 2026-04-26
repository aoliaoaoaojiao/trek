package recovery

import (
	"trek/internal/engine/candidate"
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

// BuildRecoveryCandidates 按 Memory -> Heuristic -> LLM 顺序汇总恢复候选。
func (p *Planner) BuildRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	items := make([]candidate.Candidate, 0, 8)

	memoryItems, err := p.buildFromProvider(p.config.Memory, ctx)
	if err != nil {
		return nil, err
	}
	items = append(items, memoryItems...)

	heuristicItems, err := p.buildFromProvider(p.config.Heuristic, ctx)
	if err != nil {
		return nil, err
	}
	items = append(items, heuristicItems...)

	if p.hasHighConfidenceMemory(memoryItems) {
		return items, nil
	}

	if !p.allowLLM(ctx) {
		return items, nil
	}

	llmItems, err := p.buildFromProvider(p.config.LLM, ctx)
	if err != nil {
		return nil, err
	}
	p.recordLLMCall(ctx)
	items = append(items, llmItems...)
	return items, nil
}

func (p *Planner) buildFromProvider(provider CandidateProvider, ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if provider == nil {
		return nil, nil
	}
	return provider.BuildCandidates(ctx)
}

func (p *Planner) hasHighConfidenceMemory(items []candidate.Candidate) bool {
	for _, item := range items {
		if item.Source == candidate.SourceMemory && item.Confidence >= p.config.HighConfidenceThreshold {
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
	return p.config.LLMBudget.Allow(ctx)
}

func (p *Planner) recordLLMCall(ctx enginestate.TraversalContext) {
	if p == nil || p.config.LLMBudget == nil {
		return
	}
	p.config.LLMBudget.Record(ctx)
}
