package memory

import (
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
)

// Provider 将 memory 记录转换为恢复候选。
type Provider struct {
	store *Store
}

// NewProvider 创建 memory provider。
func NewProvider(store *Store) *Provider {
	return &Provider{store: store}
}

// BuildCandidates 从 store 中检索并转换候选。
func (p *Provider) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if p == nil || p.store == nil {
		return nil, nil
	}
	records := p.store.Find(ctx)
	items := make([]candidate.Candidate, 0, len(records))
	for _, record := range records {
		item := cloneCandidate(record.Candidate)
		item.Source = candidate.SourceMemory
		item.Confidence = successRate(record)
		item.EscapeScore = record.EscapeScore
		if item.Metadata == nil {
			item.Metadata = make(map[string]string, 2)
		}
		item.Metadata["memory_key"] = record.MemoryKey
		item.Metadata["memory_outcome"] = record.Outcome
		items = append(items, item)
	}
	return items, nil
}
