package types

import (
	"fmt"
	"sort"
	"sync/atomic"
	"time"
	"trek/internal/engine/core/tool"
	"trek/logger"
)

var _ IAction = (*Action)(nil)

type Action struct {
	Node
	PriorityNodeImpl
	Hashcode   uintptr
	ActionType ActionType
}

func NewAction(actionType ActionType) *Action {
	return &Action{
		Node:             *NewNode(),
		PriorityNodeImpl: *NewPriorityNode(),
		Hashcode:         0,
		ActionType:       actionType,
	}
}

func (a *Action) GetEnabled() bool {
	return true
}

func (a *Action) GetActionType() ActionType {
	return a.ActionType
}

func (a *Action) SetPriority(priority int32) {
	a.PriorityNodeImpl.SetPriority(priority)
}

func (a *Action) GetPriorityByActionType() int32 {
	switch a.ActionType {
	case CLICK:
		return 4
	case INPUT:
		return 4
	case LONG_CLICK, SCROLL_TOP_DOWN, SCROLL_BOTTOM_UP, SCROLL_LEFT_RIGHT, SCROLL_RIGHT_LEFT:
		return 2
	default:
		return 1
	}
}

func (a *Action) IsBack() bool {
	return a.ActionType == BACK
}

func (a *Action) IsClick() bool {
	return a.ActionType == CLICK
}

func (a *Action) IsNop() bool {
	return a.ActionType == NOP
}

func (a *Action) IsValid() bool {
	return true
}

func (a *Action) ToOperate() *ActionCommand {
	opt := NewActionCommand()
	opt.Act = a.ActionType
	opt.Aid = a.GetId()
	if a.GetVisitedCount() <= 1 {
		opt.Throttle = float32(tool.RandomInt(10, getThrottle()))
	}

	return opt
}

func (a *Action) Hash() uintptr {
	return a.Hashcode
}

func (a *Action) IsModelAct() bool {
	return a.ActionType >= BACK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

func (a *Action) RequireTarget() bool {
	return a.ActionType >= CLICK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

func (a *Action) CanStartTestApp() bool {
	return a.ActionType == START || a.ActionType == RESTART || a.ActionType == CLEAN_RESTART
}

func (a *Action) Equal(other *Action) bool {
	if other == nil {
		return false
	}
	return a.ActionType == other.ActionType
}

func (a *Action) GetId() string {
	return "g0a" + a.Node.GetId()
}

func (a *Action) Visit(timestamp time.Time) {
	atomic.AddInt32(&a.Node.VisitedCount, 1)

}

func (a *Action) String() string {
	return fmt.Sprintf("{id: %s, act: %s}",
		a.GetId(), a.ActionType.String())
}

var (
	NOPAction      = NewAction(NOP)
	ACTIVATEAction = NewAction(ACTIVATE)
	INPUTAction    = NewAction(INPUT)
	RESTARTAction  = NewAction(RESTART)
)

func getThrottle() int {
	return 100
}

type StatefulAction struct {
	Action
	State    *State
	Target   IWidget
	Hashcode uintptr
}

func NewStatefulAction(state *State, targetWidget IWidget, actionType ActionType) *StatefulAction {
	asa := &StatefulAction{
		Action:   *NewAction(actionType),
		State:    state,
		Target:   targetWidget,
		Hashcode: 0,
	}

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

	logger.Debugf("page state action created hashcode:%d stateHash:%d targetHash:%d", asa.Hashcode, stateHash, targetHash)

	return asa
}

func (asa *StatefulAction) GetState() *State {
	return asa.State
}

func (asa *StatefulAction) GetTarget() IWidget {
	return asa.Target
}

func (asa *StatefulAction) GetEnabled() bool {
	if asa.Target == nil {
		return true
	}
	return asa.Target.GetEnabled()
}

func (asa *StatefulAction) IsValid() bool {
	if asa.Target == nil {
		return true
	}
	return !asa.Target.GetBounds().IsEmpty()
}

func (asa *StatefulAction) SetTarget(widget IWidget) {
	asa.Target = widget
}

func (asa *StatefulAction) ToOperate() *ActionCommand {
	opt := asa.Action.ToOperate()
	if asa.State != nil {
		opt.Sid = asa.State.GetId()
	}
	if asa.Target != nil {
		opt.Pos = *asa.Target.GetBounds()
		opt.Editable = asa.Target.IsEditable()
		opt.WidgetInfo = asa.Target.String()
	}
	return opt
}

func (asa *StatefulAction) IsTargetEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

func (asa *StatefulAction) IsEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

func (asa *StatefulAction) Hash() uintptr {
	return asa.Hashcode
}

func (asa *StatefulAction) Equal(other *StatefulAction) bool {
	if other == nil {
		return false
	}
	return asa.Hash() == other.Hash()
}

func (asa *StatefulAction) Less(other *StatefulAction) bool {
	return asa.Hash() < other.Hash()
}

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

type NetActionParam struct {
	Throttle        int    `json:"throttle"`
	NetActionTaskID int    `json:"net_action_taskid"`
	AlgorithmString string `json:"algorithm_string"`
	PackageName     string `json:"package_name"`
	Token           string `json:"token"`
	DeviceID        string `json:"device_id"`
}

type StatefulActionList []*StatefulAction

type StatefulActionSet map[uintptr]*StatefulAction

func (s StatefulActionSet) Add(action *StatefulAction) {
	if action != nil {
		s[action.Hash()] = action
	}
}

func (s StatefulActionSet) Remove(action *StatefulAction) {
	if action != nil {
		delete(s, action.Hash())
	}
}

func (s StatefulActionSet) Contains(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	_, exists := s[action.Hash()]
	return exists
}

func (s StatefulActionSet) ToSlice() StatefulActionList {
	result := make(StatefulActionList, 0, len(s))
	for _, action := range s {
		result = append(result, action)
	}
	return result
}

func (actions StatefulActionList) SortByPriority() {
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].GetPriority() < actions[j].GetPriority()
	})
}

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
