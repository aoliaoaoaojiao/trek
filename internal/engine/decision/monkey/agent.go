package monkey

import (
	"math/rand"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	sharedgraph "trek/internal/engine/decision/shared/graph"
	"trek/logger"
)

// MonkeyAgent 实现纯随机动作选择策略，即传统 Monkey 测试。
// 不学习、不记忆，每次从当前状态的可选动作中均匀随机选择。
type MonkeyAgent struct {
	model         *sharedgraph.Model
	currentState  types.IState
	newState      types.IState
	lastAction    *types.StatefulAction
	currentAction *types.StatefulAction
}

var _ types.IAgent = (*MonkeyAgent)(nil)

// NewMonkeyAgent 创建 Monkey Agent。
func NewMonkeyAgent(model *sharedgraph.Model) *MonkeyAgent {
	return &MonkeyAgent{
		model: model,
	}
}

// CreateState 从页面元素树构造状态。
func (a *MonkeyAgent) CreateState(pageName string, element types.IElement) types.IState {
	return types.Create(element, pageName)
}

// OnAddNode 图添加新节点时的回调。
func (a *MonkeyAgent) OnAddNode(node types.IState) {
	a.newState = node
}

// ResolveNewAction 从当前状态的可选动作中随机选择一个。
func (a *MonkeyAgent) ResolveNewAction() types.IAction {
	if a.currentState == nil {
		logger.Warn("Monkey: 当前状态为空，返回 NOP")
		return types.NOPAction
	}

	actions := a.currentState.TargetActions()
	if len(actions) == 0 {
		logger.Warn("Monkey: 当前状态无可用动作，返回 NOP")
		return types.NOPAction
	}

	// 过滤掉 nil 和 disabled 的动作
	validActions := make([]*types.StatefulAction, 0, len(actions))
	for _, act := range actions {
		if act != nil && act.GetEnabled() {
			validActions = append(validActions, act)
		}
	}

	if len(validActions) == 0 {
		logger.Warn("Monkey: 当前状态无有效动作，返回 NOP")
		return types.NOPAction
	}

	selected := validActions[rand.Intn(len(validActions))]
	logger.Debugf("Monkey: 随机选择动作 %s (共 %d 个候选)", selected.GetActionType(), len(validActions))
	return selected
}

// SelectNewAction 委托给 ResolveNewAction。
func (a *MonkeyAgent) SelectNewAction() types.IAction {
	return a.ResolveNewAction()
}

// UpdateStrategy 无学习，空实现。
func (a *MonkeyAgent) UpdateStrategy() {}

// MoveForward 状态推进。
func (a *MonkeyAgent) MoveForward(nextState types.IState) {
	a.lastAction = a.currentAction
	a.currentState = nextState
}

// GetAlgorithmType 返回算法标识。
func (a *MonkeyAgent) GetAlgorithmType() string {
	return decision.AlgorithmRandom.String()
}

// Stop 清理资源。
func (a *MonkeyAgent) Stop() {}

// GetCurrentState 返回当前状态，供 traversal adapter 使用。
func (a *MonkeyAgent) GetCurrentState() types.IState {
	return a.currentState
}
