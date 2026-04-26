package traversal

import (
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
)

// AlgorithmProvider 将 TraversalAlgorithm 包装为 CandidateProvider，
// 使算法能在 Recovery 候选流程中作为候选来源。
//
// 它实现 recovery.CandidateProvider 接口，在 BuildCandidates 时
// 调用内嵌算法的 ProposeCandidates 并返回结果。
type AlgorithmProvider struct {
	algorithm TraversalAlgorithm
}

// NewAlgorithmProvider 创建算法层候选提供者。
func NewAlgorithmProvider(algorithm TraversalAlgorithm) *AlgorithmProvider {
	return &AlgorithmProvider{algorithm: algorithm}
}

// BuildCandidates 调用内嵌算法的 ProposeCandidates，
// 将算法视角的候选动作转换为统一 Candidate 列表。
func (p *AlgorithmProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	if p.algorithm == nil {
		return nil, nil
	}
	candidates, err := p.algorithm.ProposeCandidates(ctx)
	if err != nil {
		return nil, err
	}
	return candidates, nil
}