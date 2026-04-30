package decision

import (
	"context"
	types2 "trek/internal/engine/decision/shared/types"
)

// Perceptor 负责把原始输入转换为统一 Observation。
type Perceptor interface {
	Observe(ctx context.Context, input PerceptionInput) (*Observation, error)
}

type PerceptionInput struct {
	PageName   string
	XMLDesc    string
	Screenshot []byte
}

type Observation struct {
	PageName   string
	XMLDesc    string
	Screenshot []byte
	Element    types2.IElement
}

type CandidateAction struct {
	Operate *types2.ActionCommand
	Source  string
}

type ExecutionPlan struct {
	Operate  *types2.ActionCommand
	Strategy string
}
