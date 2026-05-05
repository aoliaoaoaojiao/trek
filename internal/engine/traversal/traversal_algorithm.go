// Package traversal 提供遍历算法统一接口与适配器。
package traversal

import (
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

// ActionOutcome 表示动作执行后的观测结果，用于回写在线学习信号。
type ActionOutcome string

const (
	// OutcomeNewState 表示动作导致了新状态（新页面）。
	OutcomeNewState ActionOutcome = "new_state"
	// OutcomeSameState 表示动作后状态未变化。
	OutcomeSameState ActionOutcome = "same_state"
	// OutcomeEscapeBlock 表示动作成功逃离了阻塞簇。
	OutcomeEscapeBlock ActionOutcome = "escape_block"
	// OutcomeLoop 表示动作导致了循环（回到之前的状态）。
	OutcomeLoop ActionOutcome = "loop"
	// OutcomeNoOp 表示动作为空操作或无效。
	OutcomeNoOp ActionOutcome = "no_op"
)

// TraversalAlgorithm 是遍历算法的统一接口。
//
// 所有遍历算法（Reuse、UCTBandit、未来新算法）都通过此接口接入
// 候选融合与恢复规划框架。算法层只负责：
//   - 提供本算法视角下的候选动作（ProposeCandidates）
//   - 从融合后的候选集中选择最终动作（SelectAction）
//   - 接收动作执行结果以回写在线学习信号（ObserveOutcome）
//
// 算法不直接调用 LLM，不直接管理经验库，不直接主导恢复状态机。
type TraversalAlgorithm interface {
	// Name 返回算法名称，用于日志与调试。
	Name() string

	// ProposeCandidates 根据当前遍历上下文产出本算法视角的候选动作。
	//
	// 返回的候选 Source 字段应设为 perception.SourceAlgorithm，
	// Intent 字段应描述候选的语义意图（如"点击搜索按钮"），
	// Confidence 应基于算法内部的评估给出 0-1 分数。
	ProposeCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)

	// SelectAction 从融合后的候选集中选择最终执行动作。
	//
	// candidates 由融合层提供，可能包含来自 algorithm/memory/heuristic/llm
	// 多个来源的候选。算法应根据自己的策略（如 UCT 探索/利用、
	// Reuse Q 值等）从中选择最优动作。
	SelectAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error)

	// ObserveOutcome 接收动作执行后的观测结果，用于回写在线学习。
	//
	// 算法应根据 outcome 更新内部统计（如 Q 值、访问计数、奖励等）。
	ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome ActionOutcome) error
}
