package decision

import "trek/internal/engine/core/types"

// PerceptionInput 表示一次感知阶段的原始输入。
type PerceptionInput struct {
	PageName   string
	XMLDesc    string
	Screenshot []byte
}

// Observation 表示一次决策所需的统一感知输入。
type Observation struct {
	PageName   string
	XMLDesc    string
	Screenshot []byte
	Element    types.IElement
}

// CandidateAction 表示策略层给出的候选动作。
type CandidateAction struct {
	Operate *types.DeviceOperateWrapper
	Source  string
}

// ExecutionPlan 表示规划层输出的执行计划。
type ExecutionPlan struct {
	Operate  *types.DeviceOperateWrapper
	Strategy string
}
