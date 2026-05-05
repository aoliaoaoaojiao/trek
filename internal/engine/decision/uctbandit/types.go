package uctbandit

import "trek/internal/engine/core/types"

// Candidate 表示一个候选动作及其各项打分。
type Candidate struct {
	Action       *types.StatefulAction
	ActionKey    string
	ArmKey       string
	UCTScore     float64
	BanditScore  float64
	BonusScore   float64
	PenaltyScore float64
	FinalScore   float64
}

// StepContext 用于在动作执行前后传递结算信息。
type StepContext struct {
	StepIndex       int
	PrevStateID     string
	PrevPageName    string
	PrevPageCluster string
	LastActionKey   string
	LastArmKey      string
	CandidateCount  int
}
