package memory

import (
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

const candidateEnhancementConfidenceBoost = 0.2

// Provider 将 memory 记录转换为恢复候选。
type Provider struct {
	store *Store
}

// NewProvider 创建 memory provider。
func NewProvider(store *Store) *Provider {
	return &Provider{store: store}
}

// BuildCandidates 从 store 中检索并转换候选。
func (p *Provider) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if p == nil || p.store == nil {
		return nil, nil
	}
	records := p.store.Find(ctx)
	items := make([]perception.Candidate, 0, len(records))
	for _, record := range records {
		item := cloneCandidate(record.Item)
		item.Source = perception.SourceMemory
		item.Confidence = providerConfidence(record, ctx)
		item.EscapeScore = record.EscapeScore
		if item.Metadata == nil {
			item.Metadata = make(map[string]string, 4)
		}
		item.Metadata["memory_key"] = record.MemoryKey
		item.Metadata["memory_outcome"] = record.Outcome
		if shouldBoostCandidateEnhancement(record, ctx) {
			item.Metadata["memory_weight_tag"] = BlockReasonCandidateEnhancement
		}
		items = append(items, item)
	}
	return items, nil
}

func providerConfidence(record RecoveryMemoryRecord, ctx enginestate.TraversalContext) float64 {
	base := successRate(record)
	if shouldBoostCandidateEnhancement(record, ctx) {
		base += candidateEnhancementConfidenceBoost
	}
	if base > 1 {
		return 1
	}
	return base
}

func shouldBoostCandidateEnhancement(record RecoveryMemoryRecord, ctx enginestate.TraversalContext) bool {
	if !equalFold(record.BlockReason, BlockReasonCandidateEnhancement) {
		return false
	}
	return ctx.Mode == enginestate.ModeExplore || ctx.Mode == enginestate.ModeSuspectBlocked
}
