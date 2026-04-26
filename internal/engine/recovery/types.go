package recovery

import (
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
)

const defaultHighConfidenceThreshold = 0.9

// CandidateProvider 表示恢复阶段的统一候选提供者。
type CandidateProvider interface {
	BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// LLMBudget 控制恢复阶段 LLM provider 的调用预算。
type LLMBudget interface {
	Allow(ctx enginestate.TraversalContext) bool
	Record(ctx enginestate.TraversalContext)
}

// RecoveryPlanner 是恢复规划器的统一接口。
//
// 将 Planner 从具体类型抽象为接口，便于替换不同的恢复规划实现
// （如基于规则的规划器、基于上下文的规划器等），
// 同时保持 Runner 对恢复规划器的依赖解耦。
type RecoveryPlanner interface {
	BuildRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// PlannerConfig 是恢复规划器的最小配置骨架。
type PlannerConfig struct {
	Memory                  CandidateProvider
	Heuristic               CandidateProvider
	LLM                     CandidateProvider
	LLMBudget               LLMBudget
	HighConfidenceThreshold float64
}
