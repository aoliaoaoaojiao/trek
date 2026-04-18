package decision

import "context"

// Policy 负责根据 Observation 生成候选动作。
type Policy interface {
	Name() string
	Propose(ctx context.Context, obs *Observation) ([]CandidateAction, error)
}

// Planner 负责从候选动作中选择可执行计划。
type Planner interface {
	Select(ctx context.Context, obs *Observation, candidates []CandidateAction) (*ExecutionPlan, error)
}
