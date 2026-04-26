package candidate

import "trek/internal/engine/decision/shared/types"

const (
	SourceAlgorithm = "algorithm"
	SourceMemory    = "memory"
	SourceHeuristic = "heuristic"
	SourceLLM       = "llm"
)

// Candidate 是第一阶段统一候选骨架，后续由融合层和算法层共同消费。
type Candidate struct {
	Command      *types.ActionCommand
	Source       string
	Intent       string
	Confidence   float64
	NoveltyScore float64
	EscapeScore  float64
	RiskScore    float64
	Metadata     map[string]string
}

// NewCandidate 创建带 metadata 快照的统一候选。
func NewCandidate(cmd *types.ActionCommand, source string, intent string, metadata map[string]string) Candidate {
	return Candidate{
		Command:  cmd,
		Source:   source,
		Intent:   intent,
		Metadata: cloneMetadata(metadata),
	}
}

func cloneMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}
