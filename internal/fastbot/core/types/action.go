package types

import (
	"Trek/internal/fastbot/tool"
	"Trek/log"
	"fmt"
	"sort"
	"sync/atomic"
	"time"
)

var _ IAction = (*Action)(nil)

// Action 基础Action类
type Action struct {
	Node
	PriorityNodeImpl
	Hashcode   uintptr
	ActionType ActionType
	QValue     float64
}

// NewAction 创建指定类型的Action
func NewAction(actionType ActionType) *Action {
	return &Action{
		Node:             *NewNode(),
		PriorityNodeImpl: *NewPriorityNode(),
		Hashcode:         0,
		ActionType:       actionType,
		QValue:           0,
	}
}

// GetEnabled 获取是否启用
func (a *Action) GetEnabled() bool {
	return true
}

// GetActionType 获取动作类型
func (a *Action) GetActionType() ActionType {
	return a.ActionType
}

// SetPriority 设置优先级
func (a *Action) SetPriority(priority int32) {
	a.PriorityNodeImpl.SetPriority(priority)
}

// GetPriorityByActionType 根据动作类型获取优先级
func (a *Action) GetPriorityByActionType() int32 {
	switch a.ActionType {
	case CLICK:
		return 4
	case LONG_CLICK, SCROLL_TOP_DOWN, SCROLL_BOTTOM_UP, SCROLL_LEFT_RIGHT, SCROLL_RIGHT_LEFT:
		return 2
	default:
		return 1
	}
}

// IsBack 是否为返回动作
func (a *Action) IsBack() bool {
	return a.ActionType == BACK
}

// IsClick 是否为点击动作
func (a *Action) IsClick() bool {
	return a.ActionType == CLICK
}

// IsNop 是否为无操作
func (a *Action) IsNop() bool {
	return a.ActionType == NOP
}

// IsValid 是否有效
func (a *Action) IsValid() bool {
	return true
}

// ToOperate 转换为操作
func (a *Action) ToOperate() *DeviceOperateWrapper {
	opt := NewDeviceOperateWrapper()
	opt.Act = a.ActionType
	opt.Aid = a.GetId()
	if a.GetVisitedCount() <= 1 {
		opt.Throttle = float32(tool.RandomInt(10, getThrottle()))
	}

	return opt
}

// Hash 计算哈希值
func (a *Action) Hash() uintptr {
	return a.Hashcode
}

// IsModelAct 是否为模型动作
func (a *Action) IsModelAct() bool {
	return a.ActionType >= BACK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

// RequireTarget 是否需要目标
func (a *Action) RequireTarget() bool {
	return a.ActionType >= CLICK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

// CanStartTestApp 是否可以启动测试应用
func (a *Action) CanStartTestApp() bool {
	return a.ActionType == START || a.ActionType == RESTART || a.ActionType == CLEAN_RESTART
}

// Equal 判断是否相等
func (a *Action) Equal(other *Action) bool {
	if other == nil {
		return false
	}
	return a.ActionType == other.ActionType
}

// SetQValue 设置Q值
func (a *Action) SetQValue(value float64) {
	a.QValue = value
}

// GetQValue 获取Q值
func (a *Action) GetQValue() float64 {
	return a.QValue
}

// GetId 获取ID
func (a *Action) GetId() string {
	return "g0a" + a.Node.GetId()
}

// Visit 更新访问计数
func (a *Action) Visit(timestamp time.Time) {
	atomic.AddInt32(&a.Node.VisitedCount, 1)

}

// String 返回字符串表示
func (a *Action) String() string {
	return fmt.Sprintf("{id: %s, act: %s, value: %.2f}",
		a.GetId(), a.ActionType.String(), a.QValue)
}

// 全局Action实例
var (
	NOPAction      = NewAction(NOP)
	ACTIVATEAction = NewAction(ACTIVATE)
	RESTARTAction  = NewAction(RESTART)
)

// getThrottle 获取节流值
func getThrottle() int {
	return 100
}

// StatefulAction 嵌入动作与整个活动状态、目标Widget和要执行的动作
type StatefulAction struct {
	Action
	// 表示动作的起始状态节点，构建状态转移图的关键连接点
	State    *State
	Target   *Widget
	Hashcode uintptr
}

// NewStatefulAction 创建新的NewStatefulAction
func NewStatefulAction(state *State, targetWidget *Widget, actionType ActionType) *StatefulAction {
	asa := &StatefulAction{
		Action:   *NewAction(actionType),
		State:    state,
		Target:   targetWidget,
		Hashcode: 0, // 初始化为0，将在下面计算
	}

	// 计算哈希码
	hashcode := tool.HashInt(int(asa.GetActionType()))
	var stateHash uintptr
	if asa.State != nil {
		stateHash = asa.State.Hash()
	} else {
		stateHash = 0x1
	}

	var targetHash uintptr
	if asa.Target != nil {
		targetHash = asa.Target.Hash()
	} else {
		targetHash = 0x1
	}

	asa.Hashcode = 0x9e3779b9 + (hashcode << 2) ^ (((stateHash << 4) ^ (targetHash << 3)) << 1)

	log.Debugf("page state action created hashcode:%d stateHash:%d targetHash:%d", asa.Hashcode, stateHash, targetHash)

	return asa
}

// GetState 获取状态
func (asa *StatefulAction) GetState() *State {
	return asa.State
}

// GetTarget 获取目标
func (asa *StatefulAction) GetTarget() *Widget {
	return asa.Target
}

// GetEnabled 获取是否启用
func (asa *StatefulAction) GetEnabled() bool {
	if asa.Target == nil {
		return true
	}
	return asa.Target.GetEnabled()
}

// IsValid 是否有效
func (asa *StatefulAction) IsValid() bool {
	if asa.Target == nil {
		return true
	}
	return !asa.Target.GetBounds().IsEmpty()
}

// SetTarget 设置目标
func (asa *StatefulAction) SetTarget(widget *Widget) {
	asa.Target = widget
}

// ToOperate 转换为操作
func (asa *StatefulAction) ToOperate() *DeviceOperateWrapper {
	opt := asa.Action.ToOperate()
	if asa.State != nil {
		opt.Sid = asa.State.GetId()
	}
	if asa.Target != nil {
		opt.Pos = *asa.Target.GetBounds()
		opt.Editable = asa.Target.IsEditable()

	}
	return opt
}

// IsTargetEmpty 目标是否为空
func (asa *StatefulAction) IsTargetEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

// IsEmpty 判断是否为空
func (asa *StatefulAction) IsEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

// Hash 计算哈希值
func (asa *StatefulAction) Hash() uintptr {
	return asa.Hashcode
}

// Equal 判断是否相等
func (asa *StatefulAction) Equal(other *StatefulAction) bool {
	if other == nil {
		return false
	}
	return asa.Hash() == other.Hash()
}

// Less 比较大小
func (asa *StatefulAction) Less(other *StatefulAction) bool {
	return asa.Hash() < other.Hash()
}

// String 返回字符串表示
func (asa *StatefulAction) String() string {
	stateID := ""
	if asa.State != nil {
		stateID = asa.State.GetId()
	}

	targetStr := ""
	if asa.Target != nil {
		targetStr = asa.Target.String()
	}

	return fmt.Sprintf("{%s, state: %s, node: %s}",
		asa.Action.String(), stateID, targetStr)
}

// NetActionParam 网络动作参数
type NetActionParam struct {
	Throttle        int    `json:"throttle"`
	NetActionTaskID int    `json:"net_action_taskid"`
	AlgorithmString string `json:"algorithm_string"`
	PackageName     string `json:"package_name"`
	Token           string `json:"token"`
	DeviceID        string `json:"device_id"`
}

// StatefulActionList PageNameStateAction指针切片
type StatefulActionList []*StatefulAction

// StatefulActionSet PageNameStateAction指针集合
type StatefulActionSet map[uintptr]*StatefulAction

// Add 添加到集合
func (s StatefulActionSet) Add(action *StatefulAction) {
	if action != nil {
		s[action.Hash()] = action
	}
}

// Remove 从集合中移除
func (s StatefulActionSet) Remove(action *StatefulAction) {
	if action != nil {
		delete(s, action.Hash())
	}
}

// Contains 检查是否包含
func (s StatefulActionSet) Contains(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	_, exists := s[action.Hash()]
	return exists
}

// ToSlice 转换为切片
func (s StatefulActionSet) ToSlice() StatefulActionList {
	result := make(StatefulActionList, 0, len(s))
	for _, action := range s {
		result = append(result, action)
	}
	return result
}

// SortByPriority 按优先级排序
func (actions StatefulActionList) SortByPriority() {
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].GetPriority() < actions[j].GetPriority()
	})
}

// FilterByQValue 按Q值过滤
func (actions StatefulActionList) FilterByQValue(minQValue float64) StatefulActionList {
	result := make(StatefulActionList, 0)
	for _, action := range actions {
		if action.GetQValue() >= minQValue {
			result = append(result, action)
		}
	}
	return result
}

// GetMaxQValueAction 获取最大Q值的动作
func (actions StatefulActionList) GetMaxQValueAction() *StatefulAction {
	if len(actions) == 0 {
		return nil
	}

	maxAction := actions[0]
	maxQValue := maxAction.GetQValue()

	for _, action := range actions[1:] {
		if action.GetQValue() > maxQValue {
			maxAction = action
			maxQValue = action.GetQValue()
		}
	}

	return maxAction
}

// GetRandomUnvisitedAction 获取随机未访问的动作
func (actions StatefulActionList) GetRandomUnvisitedAction() *StatefulAction {
	unvisited := make(StatefulActionList, 0)
	for _, action := range actions {
		if !action.IsVisited() {
			unvisited = append(unvisited, action)
		}
	}

	if len(unvisited) == 0 {
		return nil
	}

	index := tool.RandomInt(0, len(unvisited))
	return unvisited[index]
}
