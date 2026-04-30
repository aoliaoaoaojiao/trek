package types

import (
	"time"
)

// EnableNode 可启用接口
type EnableNode interface {
	GetEnabled() bool
}

type IGraphListener interface {
	OnAddNode(node IState)
}

type IAgent interface {
	IGraphListener
	ResolveNewAction() IAction
	SelectNewAction() IAction
	UpdateStrategy()
	MoveForward(nextState IState)
	GetAlgorithmType() string
	CreateState(pageName string, element IElement) IState
	Stop()
}

// StateBlockAwareAgent 是可选能力接口；仅在策略实现支持"状态阻塞检测"时提供。
type StateBlockAwareAgent interface {
	GetCurrentStateBlockTimes() int
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
	GetMergedWidgets() WidgetListMap
	GetVisitedCount() int32
	GetWidgets() WidgetList

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

type IElement interface {
	GetIdentifierHash() uintptr

	GetParent() IElement
	GetPath() string
	GetXPath() string

	GetText() string
	SetText(text string)

	GetClickable() bool
	SetClickable(clickable bool)

	GetLongClickable() bool
	SetLongClickable(longClickable bool)

	GetCheckBoxable() bool
	SetCheckBoxable(checkBoxable bool)

	GetEnable() bool
	SetEnable(enable bool)

	GetEditable() bool
	SetEditable(editable bool)

	GetScrollType() ScrollType
	SetScrollType(scrollType string)

	GetBounds() *Rect
	SetBounds(rect *Rect)

	GetAttr(key string) interface{}
	SetAttr(key string, value interface{})

	GetChildren() []IElement
	SetChildren(childList []IElement)
	String() string

	DeleteElement(xpath string) bool
	Query(xpath string) []IElement
}

// IWidget Widget接口，定义了Widget的基本操作和属性访问方法
type IWidget interface {
	GetText() string
	GetBounds() *Rect
	GetEnabled() bool
	GetPath() string
	GetXPath() string

	// 操作和动作相关方法
	HasOperate(opt OperateType) bool
	HasAction() bool
	GetActions() []ActionType
	IsEditable() bool

	// 层级关系方法
	GetParent() IWidget

	// 标识和哈希方法
	Hash() uintptr

	// 字符串表示
	String() string

	ClearDetails()
}
