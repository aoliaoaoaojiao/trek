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

// Action 鍩虹Action绫?

type Action struct {
	Node
	PriorityNodeImpl
	Hashcode   uintptr
	ActionType ActionType
	QValue     float64
}

// NewAction 鍒涘缓鎸囧畾绫诲瀷鐨凙ction
func NewAction(actionType ActionType) *Action {
	return &Action{
		Node:             *NewNode(),
		PriorityNodeImpl: *NewPriorityNode(),
		Hashcode:         0,
		ActionType:       actionType,
		QValue:           0,
	}
}

// GetEnabled 鑾峰彇鏄惁鍚敤
func (a *Action) GetEnabled() bool {
	return true
}

// GetActionType 鑾峰彇鍔ㄤ綔绫诲瀷
func (a *Action) GetActionType() ActionType {
	return a.ActionType
}

// SetPriority 璁剧疆浼樺厛绾?

func (a *Action) SetPriority(priority int32) {
	a.PriorityNodeImpl.SetPriority(priority)
}

// GetPriorityByActionType 鏍规嵁鍔ㄤ綔绫诲瀷鑾峰彇浼樺厛绾?

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

// IsBack 鏄惁涓鸿繑鍥炲姩浣?

func (a *Action) IsBack() bool {
	return a.ActionType == BACK
}

// IsClick 鏄惁涓虹偣鍑诲姩浣?

func (a *Action) IsClick() bool {
	return a.ActionType == CLICK
}

// IsNop 鏄惁涓烘棤鎿嶄綔
func (a *Action) IsNop() bool {
	return a.ActionType == NOP
}

// IsValid 鏄惁鏈夋晥
func (a *Action) IsValid() bool {
	return true
}

// ToOperate 杞崲涓烘搷浣?

func (a *Action) ToOperate() *DeviceOperateWrapper {
	opt := NewDeviceOperateWrapper()
	opt.Act = a.ActionType
	opt.Aid = a.GetId()
	if a.GetVisitedCount() <= 1 {
		opt.Throttle = float32(tool.RandomInt(10, getThrottle()))
	}

	return opt
}

// Hash 璁＄畻鍝堝笇鍊?

func (a *Action) Hash() uintptr {
	return a.Hashcode
}

// IsModelAct 鏄惁涓烘ā鍨嬪姩浣?

func (a *Action) IsModelAct() bool {
	return a.ActionType >= BACK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

// RequireTarget 鏄惁闇€瑕佺洰鏍?

func (a *Action) RequireTarget() bool {
	return a.ActionType >= CLICK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

// CanStartTestApp 鏄惁鍙互鍚姩娴嬭瘯搴旂敤
func (a *Action) CanStartTestApp() bool {
	return a.ActionType == START || a.ActionType == RESTART || a.ActionType == CLEAN_RESTART
}

// Equal 鍒ゆ柇鏄惁鐩哥瓑
func (a *Action) Equal(other *Action) bool {
	if other == nil {
		return false
	}
	return a.ActionType == other.ActionType
}

// SetQValue 璁剧疆Q鍊?

func (a *Action) SetQValue(value float64) {
	a.QValue = value
}

// GetQValue 鑾峰彇Q鍊?

func (a *Action) GetQValue() float64 {
	return a.QValue
}

// GetId 鑾峰彇ID
func (a *Action) GetId() string {
	return "g0a" + a.Node.GetId()
}

// Visit 鏇存柊璁块棶璁℃暟
func (a *Action) Visit(timestamp time.Time) {
	atomic.AddInt32(&a.Node.VisitedCount, 1)

}

// String 杩斿洖瀛楃涓茶〃绀?

func (a *Action) String() string {
	return fmt.Sprintf("{id: %s, act: %s, value: %.2f}",
		a.GetId(), a.ActionType.String(), a.QValue)
}

// 鍏ㄥ眬Action瀹炰緥
var (
	NOPAction      = NewAction(NOP)
	ACTIVATEAction = NewAction(ACTIVATE)
	RESTARTAction  = NewAction(RESTART)
)

// getThrottle 鑾峰彇鑺傛祦鍊?

func getThrottle() int {
	return 100
}

// StatefulAction 宓屽叆鍔ㄤ綔涓庢暣涓椿鍔ㄧ姸鎬併€佺洰鏍嘩idget鍜岃鎵ц鐨勫姩浣?

type StatefulAction struct {
	Action
	// 琛ㄧず鍔ㄤ綔鐨勮捣濮嬬姸鎬佽妭鐐癸紝鏋勫缓鐘舵€佽浆绉诲浘鐨勫叧閿繛鎺ョ偣
	State    *State
	Target   IWidget
	Hashcode uintptr
}

// NewStatefulAction 鍒涘缓鏂扮殑NewStatefulAction
func NewStatefulAction(state *State, targetWidget IWidget, actionType ActionType) *StatefulAction {
	asa := &StatefulAction{
		Action:   *NewAction(actionType),
		State:    state,
		Target:   targetWidget,
		Hashcode: 0, // 鍒濆鍖栦负0锛屽皢鍦ㄤ笅闈㈣绠?
	}

	// 璁＄畻鍝堝笇鐮?
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

// GetState 鑾峰彇鐘舵€?

func (asa *StatefulAction) GetState() *State {
	return asa.State
}

// GetTarget 鑾峰彇鐩爣
func (asa *StatefulAction) GetTarget() IWidget {
	return asa.Target
}

// GetEnabled 鑾峰彇鏄惁鍚敤
func (asa *StatefulAction) GetEnabled() bool {
	if asa.Target == nil {
		return true
	}
	return asa.Target.GetEnabled()
}

// IsValid 鏄惁鏈夋晥
func (asa *StatefulAction) IsValid() bool {
	if asa.Target == nil {
		return true
	}
	return !asa.Target.GetBounds().IsEmpty()
}

// SetTarget 璁剧疆鐩爣
func (asa *StatefulAction) SetTarget(widget IWidget) {
	asa.Target = widget
}

// ToOperate 杞崲涓烘搷浣?

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

// IsTargetEmpty 鐩爣鏄惁涓虹┖
func (asa *StatefulAction) IsTargetEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

// IsEmpty 鍒ゆ柇鏄惁涓虹┖
func (asa *StatefulAction) IsEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

// Hash 璁＄畻鍝堝笇鍊?

func (asa *StatefulAction) Hash() uintptr {
	return asa.Hashcode
}

// Equal 鍒ゆ柇鏄惁鐩哥瓑
func (asa *StatefulAction) Equal(other *StatefulAction) bool {
	if other == nil {
		return false
	}
	return asa.Hash() == other.Hash()
}

// Less 姣旇緝澶у皬
func (asa *StatefulAction) Less(other *StatefulAction) bool {
	return asa.Hash() < other.Hash()
}

// String 杩斿洖瀛楃涓茶〃绀?

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

// NetActionParam 缃戠粶鍔ㄤ綔鍙傛暟
type NetActionParam struct {
	Throttle        int    `json:"throttle"`
	NetActionTaskID int    `json:"net_action_taskid"`
	AlgorithmString string `json:"algorithm_string"`
	PackageName     string `json:"package_name"`
	Token           string `json:"token"`
	DeviceID        string `json:"device_id"`
}

// StatefulActionList PageNameStateAction鎸囬拡鍒囩墖
type StatefulActionList []*StatefulAction

// StatefulActionSet PageNameStateAction鎸囬拡闆嗗悎
type StatefulActionSet map[uintptr]*StatefulAction

// Add 娣诲姞鍒伴泦鍚?

func (s StatefulActionSet) Add(action *StatefulAction) {
	if action != nil {
		s[action.Hash()] = action
	}
}

// Remove 浠庨泦鍚堜腑绉婚櫎
func (s StatefulActionSet) Remove(action *StatefulAction) {
	if action != nil {
		delete(s, action.Hash())
	}
}

// Contains 妫€鏌ユ槸鍚﹀寘鍚?

func (s StatefulActionSet) Contains(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	_, exists := s[action.Hash()]
	return exists
}

// ToSlice 杞崲涓哄垏鐗?

func (s StatefulActionSet) ToSlice() StatefulActionList {
	result := make(StatefulActionList, 0, len(s))
	for _, action := range s {
		result = append(result, action)
	}
	return result
}

// SortByPriority 鎸変紭鍏堢骇鎺掑簭
func (actions StatefulActionList) SortByPriority() {
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].GetPriority() < actions[j].GetPriority()
	})
}

// FilterByQValue 鎸塓鍊艰繃婊?

func (actions StatefulActionList) FilterByQValue(minQValue float64) StatefulActionList {
	result := make(StatefulActionList, 0)
	for _, action := range actions {
		if action.GetQValue() >= minQValue {
			result = append(result, action)
		}
	}
	return result
}

// GetMaxQValueAction 鑾峰彇鏈€澶鍊肩殑鍔ㄤ綔
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

// GetRandomUnvisitedAction 鑾峰彇闅忔満鏈闂殑鍔ㄤ綔
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
