package decision

import (
	types2 "trek/internal/engine/decision/shared/types"
)

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
