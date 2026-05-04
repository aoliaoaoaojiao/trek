package traversal

import (
	"fmt"

	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

// UCTBanditAdapter 将实现了 types.IAgent 接口的 uctbandit.Agent
// 包装为 TraversalAlgorithm 接口实现。
//
// ProposeCandidates: 从注入的 StateProvider 提取可选动作，
// 映射为 Source=algorithm 的统一 Candidate，UCT 分数作为 Confidence。
// SelectAction: 优先选择算法自身候选，否则从其他来源中按置信度选择。
// ObserveOutcome: 将动作执行结果回写为动作级反馈分，影响后续候选排序。
type UCTBanditAdapter struct {
	agent         types.IAgent
	stateProvider StateProvider
	feedback      *outcomeFeedbackStore
}

// NewUCTBanditAdapter 创建 UCTBandit 算法适配器。
// agent 参数为 uctbandit.Agent 实例，可为 nil。
// stateProvider 用于获取当前状态的可选动作列表，可为 nil。
func NewUCTBanditAdapter(agent types.IAgent, stateProvider StateProvider) *UCTBanditAdapter {
	return &UCTBanditAdapter{
		agent:         agent,
		stateProvider: stateProvider,
		feedback:      newOutcomeFeedbackStore(),
	}
}

// Name 返回算法名称。
func (a *UCTBanditAdapter) Name() string {
	return "uctbandit_adapter"
}

// ProposeCandidates 从 UCTBandit 当前状态提取可选动作，
// 映射为统一 Candidate 列表，UCT 分数作为 Confidence。
func (a *UCTBanditAdapter) ProposeCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
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
		// UCTBandit 优先级通过 GetPriority 表达，
		// 归一化到 [0, 1] 作为 Confidence 的近似。
		priority := action.GetPriority()
		confidence := float64(priority) / 100.0
		if confidence > 1.0 {
			confidence = 1.0
		}
		if confidence < 0 {
			confidence = 0
		}

		c := perception.NewCandidate(
			cmd,
			perception.SourceAlgorithm,
			action.GetActionType().String(),
			map[string]string{
				"algorithm":     "uctbandit",
				"action_type":   cmd.Act.String(),
				"visited_count": fmt.Sprintf("%d", action.GetVisitedCount()),
				"priority":      fmt.Sprintf("%d", priority),
			},
		)
		c.Confidence = confidence
		c.NoveltyScore = noveltyFromVisitCount(action.GetVisitedCount())
		c = applyOutcomeFeedback(c, a.feedback)
		candidates = append(candidates, c)
	}
	return candidates, nil
}

// SelectAction 从融合候选集中选择最终动作。
// 优先选择算法自身的候选，否则从其他来源中按置信度选择。
func (a *UCTBanditAdapter) SelectAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 优先选择算法自身的候选
	for _, c := range candidates {
		if c.Source == perception.SourceAlgorithm && c.Command != nil && c.Command.IsValid() {
			scored := applyOutcomeFeedback(c, a.feedback)
			return scored.Command, nil
		}
	}

	// 回退：选择置信度最高的有效候选
	best := perception.Candidate{}
	found := false
	for _, c := range candidates {
		if c.Command == nil || !c.Command.IsValid() {
			continue
		}
		scored := applyOutcomeFeedback(c, a.feedback)
		if !found || scored.Confidence+scored.EscapeScore-scored.RiskScore > best.Confidence+best.EscapeScore-best.RiskScore {
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
func (a *UCTBanditAdapter) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome ActionOutcome) error {
	if a == nil || action == nil {
		return nil
	}
	a.feedback.observe(action, outcome)
	return nil
}

// noveltyFromVisitCount 将访问计数转换为新颖性分数。
// 未访问的动作新颖性高(1.0)，频繁访问的动作新颖性低(接近0)。
func noveltyFromVisitCount(visitCount int32) float64 {
	if visitCount <= 0 {
		return 1.0
	}
	return 1.0 / (1.0 + float64(visitCount))
}
