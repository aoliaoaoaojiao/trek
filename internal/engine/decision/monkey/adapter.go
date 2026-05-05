package monkey

import (
	"math/rand"

	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
)

// MonkeyAdapter 将 Monkey Agent 包装为 TraversalAlgorithm 接口实现。
// 纯随机选择，不学习。
type MonkeyAdapter struct {
	agent         types.IAgent
	stateProvider traversal.StateProvider
}

// NewMonkeyAdapter 创建 Monkey 算法适配器。
func NewMonkeyAdapter(agent types.IAgent, stateProvider traversal.StateProvider) *MonkeyAdapter {
	return &MonkeyAdapter{
		agent:         agent,
		stateProvider: stateProvider,
	}
}

// Name 返回算法名称。
func (a *MonkeyAdapter) Name() string {
	return "monkey"
}

// ProposeCandidates 从当前状态提取所有可选动作作为候选。
func (a *MonkeyAdapter) ProposeCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	if a.stateProvider == nil {
		return nil, nil
	}

	state := a.stateProvider.CurrentState()
	if state == nil {
		return nil, nil
	}

	actions := state.GetActions()
	if len(actions) == 0 {
		return nil, nil
	}

	candidates := make([]perception.Candidate, 0, len(actions))
	for _, action := range actions {
		if action == nil {
			continue
		}
		cmd := action.ToOperate()
		if !cmd.IsValid() {
			continue
		}
		candidates = append(candidates, perception.NewCandidate(
			cmd,
			perception.SourceAlgorithm,
			action.GetActionType().String(),
			map[string]string{
				"algorithm":   "monkey",
				"action_type": cmd.Act.String(),
			},
		))
	}
	return candidates, nil
}

// SelectAction 从候选集中随机选择一个有效动作。
func (a *MonkeyAdapter) SelectAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 过滤有效候选
	valid := make([]perception.Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Command != nil && c.Command.IsValid() {
			valid = append(valid, c)
		}
	}

	if len(valid) == 0 {
		return nil, nil
	}

	selected := valid[rand.Intn(len(valid))]
	return selected.Command, nil
}

// ObserveOutcome 无学习，空实现。
func (a *MonkeyAdapter) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
	return nil
}
