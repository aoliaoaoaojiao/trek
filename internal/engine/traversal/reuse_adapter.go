package traversal

import (
	"fmt"

	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
)

// StateProvider 提供当前遍历状态给适配器，用于从算法内部提取候选。
//
// 由于 IAgent 接口不暴露 GetCurrentState 方法，
// 适配器需要通过 StateProvider 获取当前状态的可选动作列表。
// 在实际运行时，Runner 等调用方负责注入状态提供者。
type StateProvider interface {
	// CurrentState 返回算法当前所在的页面状态，用于提取可选动作。
	// 返回 nil 表示当前无可用状态。
	CurrentState() types.IState
}

// ReuseAdapter 将实现了 types.IAgent 接口的 ModelReusableAgent
// 包装为 TraversalAlgorithm 接口实现。
//
// ProposeCandidates: 从注入的 StateProvider 提取可选动作，
// 映射为 Source=algorithm 的统一 Candidate 列表。
// SelectAction: 当融合候选集中存在来源为 algorithm 的候选时，
// 优先从中选择；否则回退到其他来源中按置信度选择。
// ObserveOutcome: 根据 ActionOutcome 映射学习信号（当前为空操作）。
type ReuseAdapter struct {
	agent         types.IAgent
	stateProvider StateProvider
}

// NewReuseAdapter 创建 Reuse 算法适配器。
// agent 参数为 ModelReusableAgent 实例，可为 nil。
// stateProvider 用于获取当前状态的可选动作列表，可为 nil。
func NewReuseAdapter(agent types.IAgent, stateProvider StateProvider) *ReuseAdapter {
	return &ReuseAdapter{
		agent:         agent,
		stateProvider: stateProvider,
	}
}

// Name 返回算法名称。
func (a *ReuseAdapter) Name() string {
	return "reuse_adapter"
}

// ProposeCandidates 从注入的状态提供者提取可选动作，
// 映射为统一 Candidate 列表。
//
// 当 stateProvider 为 nil 或当前状态无可用动作时返回空列表。
func (a *ReuseAdapter) ProposeCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
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

	candidates := make([]candidate.Candidate, 0, len(actions))
	for _, action := range actions {
		if action == nil {
			continue
		}
		cmd := action.ToOperate()
		if !cmd.IsValid() {
			continue
		}
		candidates = append(candidates, candidate.NewCandidate(
			cmd,
			candidate.SourceAlgorithm,
			action.GetActionType().String(),
			map[string]string{
				"algorithm":     "reuse",
				"action_type":   cmd.Act.String(),
				"visited_count": formatInt32(action.GetVisitedCount()),
			},
		))
	}
	return candidates, nil
}

// SelectAction 从融合候选集中选择最终动作。
//
// 优先选择 Source=algorithm 的候选（来自 Reuse 自身产出），
// 若无 algorithm 来源候选，从其他来源中按 Confidence 选择最高者。
func (a *ReuseAdapter) SelectAction(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types.ActionCommand, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 优先选择算法自身的候选
	for _, c := range candidates {
		if c.Source == candidate.SourceAlgorithm && c.Command != nil && c.Command.IsValid() {
			return c.Command, nil
		}
	}

	// 回退：选择置信度最高的有效候选
	best := candidate.Candidate{}
	found := false
	for _, c := range candidates {
		if c.Command == nil || !c.Command.IsValid() {
			continue
		}
		if !found || c.Confidence > best.Confidence {
			best = c
			found = true
		}
	}
	if found {
		return best.Command, nil
	}

	return nil, nil
}

// ObserveOutcome 接收动作执行结果，映射到 Reuse 内部学习信号。
//
// 当前实现为空操作，因为 Reuse 的 UpdateStrategy 已在内部处理。
// 未来可在此桥接外部 Outcome 到 Reuse 的奖励机制。
func (a *ReuseAdapter) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome ActionOutcome) error {
	// Reuse 的学习信号在 agent.UpdateStrategy() 和 agent.MoveForward() 中内部处理，
	// 此处暂为空操作。Phase 4 后续迭代可桥接外部 Outcome 到 Reuse 的奖励机制。
	return nil
}

// formatInt32 将 int32 格式化为字符串。
func formatInt32(v int32) string {
	return fmt.Sprintf("%d", v)
}