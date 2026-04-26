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
// ObserveOutcome: 将动作执行结果回写为动作级反馈分，影响后续候选排序。
type ReuseAdapter struct {
	agent         types.IAgent
	stateProvider StateProvider
	feedback      *outcomeFeedbackStore
}

// NewReuseAdapter 创建 Reuse 算法适配器。
// agent 参数为 ModelReusableAgent 实例，可为 nil。
// stateProvider 用于获取当前状态的可选动作列表，可为 nil。
func NewReuseAdapter(agent types.IAgent, stateProvider StateProvider) *ReuseAdapter {
	return &ReuseAdapter{
		agent:         agent,
		stateProvider: stateProvider,
		feedback:      newOutcomeFeedbackStore(),
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
		last := len(candidates) - 1
		candidates[last] = applyOutcomeFeedback(candidates[last], a.feedback)
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
			scored := applyOutcomeFeedback(c, a.feedback)
			return scored.Command, nil
		}
	}

	// 回退：选择置信度最高的有效候选
	best := candidate.Candidate{}
	found := false
	for _, c := range candidates {
		if c.Command == nil || !c.Command.IsValid() {
			continue
		}
		scored := applyOutcomeFeedback(c, a.feedback)
		if !found || scored.Confidence-scored.RiskScore > best.Confidence-best.RiskScore {
			best = scored
			found = true
		}
	}
	if found {
		return best.Command, nil
	}

	return nil, nil
}

// ObserveOutcome 接收动作执行结果并更新动作级反馈分。
func (a *ReuseAdapter) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome ActionOutcome) error {
	if a == nil || action == nil {
		return nil
	}
	a.feedback.observe(action, outcome)
	return nil
}

// formatInt32 将 int32 格式化为字符串。
func formatInt32(v int32) string {
	return fmt.Sprintf("%d", v)
}
