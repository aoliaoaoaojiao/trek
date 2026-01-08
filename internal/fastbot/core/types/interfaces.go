package types

import (
	"time"
)

// QValueNode 具有Q值的节点接口
type QValueNode interface {
	GetQValue() float64
	SetQValue(qValue float64)
}

// EnableNode 可启用接口
type EnableNode interface {
	GetEnabled() bool
}

type IGraphListener interface {
	OnAddNode(node IState)
}

type IAgent interface {
	IGraphListener
	GetCurrentStateBlockTimes() int
	ResolveNewAction() IAction
	SelectNewAction() IAction
	UpdateStrategy()
	MoveForward(nextState IState)
	GetAlgorithmType() string
	CreateState(pageName string, element *Element) IState
}

type HashNode interface {
	Visit(timestamp time.Time)
	Hash() uintptr
}

type Serializable interface {
	String() string
}

type IAction interface {
	HashNode
	QValueNode
	Serializable
	EnableNode
	GetActionType() ActionType
	GetPriority() int32
	SetPriority(priority int32)
	GetPriorityByActionType() int32
	GetVisitedCount() int32
	IsVisited() bool
	Visit(timestamp time.Time)
	GetId() string
	IsModelAct() bool
}

type IStatefulActionFilter interface {
	Include(action *StatefulAction) bool
	GetPriority(action *StatefulAction) int32
}

type IState interface {
	HashNode
	Serializable

	// 基本属性
	SetId(id int32)
	GetId() string
	GetPageNameString() string
	GetActions() StatefulActionList
	GetBackAction() *StatefulAction
	GetMergedWidgets() WidgetListMap
	GetVisitedCount() int32
	GetWidgets() WidgetList

	// 动作选择
	RandomPickUnvisitedAction() IAction
	GreedyPickAction(filter IStatefulActionFilter) IAction
	RandomPickAction(filter IStatefulActionFilter, includeBack bool) IAction

	// 工具方法
	TargetActions() StatefulActionList
	ResolveAt(action *StatefulAction, t time.Time) *StatefulAction
	SetPriority(priority int32)
	GetPriority() int32
	HasDetail() bool
	FillDetails(state IState)
	ClearDetails()
	Equals(state IState) bool
	IsSaturated(action *StatefulAction) bool
}
