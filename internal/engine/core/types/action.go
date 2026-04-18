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

// Action 閸╄櫣顢匒ction缁?

type Action struct {
	Node
	PriorityNodeImpl
	Hashcode   uintptr
	ActionType ActionType
	QValue     float64
}

// NewAction 閸掓稑缂撻幐鍥х暰缁鐎烽惃鍑檆tion
func NewAction(actionType ActionType) *Action {
	return &Action{
		Node:             *NewNode(),
		PriorityNodeImpl: *NewPriorityNode(),
		Hashcode:         0,
		ActionType:       actionType,
		QValue:           0,
	}
}

// GetEnabled 閼惧嘲褰囬弰顖氭儊閸氼垳鏁?
func (a *Action) GetEnabled() bool {
	return true
}

// GetActionType 閼惧嘲褰囬崝銊ょ稊缁鐎?
func (a *Action) GetActionType() ActionType {
	return a.ActionType
}

// SetPriority 鐠佸墽鐤嗘导妯哄帥缁?

func (a *Action) SetPriority(priority int32) {
	a.PriorityNodeImpl.SetPriority(priority)
}

// GetPriorityByActionType 閺嶈宓侀崝銊ょ稊缁鐎烽懢宄板絿娴兼ê鍘涚痪?

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

// IsBack 閺勵垰鎯佹稉楦跨箲閸ョ偛濮╂担?

func (a *Action) IsBack() bool {
	return a.ActionType == BACK
}

// IsClick 閺勵垰鎯佹稉铏瑰仯閸戣濮╂担?

func (a *Action) IsClick() bool {
	return a.ActionType == CLICK
}

// IsNop 閺勵垰鎯佹稉鐑樻￥閹垮秳缍?
func (a *Action) IsNop() bool {
	return a.ActionType == NOP
}

// IsValid 閺勵垰鎯侀張澶嬫櫏
func (a *Action) IsValid() bool {
	return true
}

// ToOperate 鏉烆剚宕叉稉鐑樻惙娴?

func (a *Action) ToOperate() *ActionCommand {
	opt := NewActionCommand()
	opt.Act = a.ActionType
	opt.Aid = a.GetId()
	if a.GetVisitedCount() <= 1 {
		opt.Throttle = float32(tool.RandomInt(10, getThrottle()))
	}

	return opt
}

// Hash 鐠侊紕鐣婚崫鍫濈瑖閸?

func (a *Action) Hash() uintptr {
	return a.Hashcode
}

// IsModelAct 閺勵垰鎯佹稉鐑樐侀崹瀣З娴?

func (a *Action) IsModelAct() bool {
	return a.ActionType >= BACK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

// RequireTarget 閺勵垰鎯侀棁鈧憰浣烘窗閺?

func (a *Action) RequireTarget() bool {
	return a.ActionType >= CLICK && a.ActionType <= SCROLL_BOTTOM_UP_N
}

// CanStartTestApp 閺勵垰鎯侀崣顖欎簰閸氼垰濮╁ù瀣槸鎼存梻鏁?
func (a *Action) CanStartTestApp() bool {
	return a.ActionType == START || a.ActionType == RESTART || a.ActionType == CLEAN_RESTART
}

// Equal 閸掋倖鏌囬弰顖氭儊閻╁摜鐡?
func (a *Action) Equal(other *Action) bool {
	if other == nil {
		return false
	}
	return a.ActionType == other.ActionType
}

// SetQValue 鐠佸墽鐤哘閸?

func (a *Action) SetQValue(value float64) {
	a.QValue = value
}

// GetQValue 閼惧嘲褰嘠閸?

func (a *Action) GetQValue() float64 {
	return a.QValue
}

// GetId 閼惧嘲褰嘔D
func (a *Action) GetId() string {
	return "g0a" + a.Node.GetId()
}

// Visit 閺囧瓨鏌婄拋鍧楁６鐠佲剝鏆?
func (a *Action) Visit(timestamp time.Time) {
	atomic.AddInt32(&a.Node.VisitedCount, 1)

}

// String 鏉╂柨娲栫€涙顑佹稉鑼躲€冪粈?

func (a *Action) String() string {
	return fmt.Sprintf("{id: %s, act: %s, value: %.2f}",
		a.GetId(), a.ActionType.String(), a.QValue)
}

// 閸忋劌鐪珹ction鐎圭偘绶?
var (
	NOPAction      = NewAction(NOP)
	ACTIVATEAction = NewAction(ACTIVATE)
	RESTARTAction  = NewAction(RESTART)
)

// getThrottle 閼惧嘲褰囬懞鍌涚ウ閸?

func getThrottle() int {
	return 100
}

// StatefulAction 瀹撳苯鍙嗛崝銊ょ稊娑撳孩鏆ｆ稉顏呮た閸斻劎濮搁幀浣碘偓浣烘窗閺嶅槱idget閸滃矁顩﹂幍褑顢戦惃鍕З娴?

type StatefulAction struct {
	Action
	// 鐞涖劎銇氶崝銊ょ稊閻ㄥ嫯鎹ｆ慨瀣Ц閹浇濡悙鐧哥礉閺嬪嫬缂撻悩鑸碘偓浣芥祮缁夎娴橀惃鍕彠闁款喛绻涢幒銉у仯
	State    *State
	Target   IWidget
	Hashcode uintptr
}

// NewStatefulAction 閸掓稑缂撻弬鎵畱NewStatefulAction
func NewStatefulAction(state *State, targetWidget IWidget, actionType ActionType) *StatefulAction {
	asa := &StatefulAction{
		Action:   *NewAction(actionType),
		State:    state,
		Target:   targetWidget,
		Hashcode: 0, // 閸掓繂顫愰崠鏍﹁礋0閿涘苯鐨㈤崷銊ょ瑓闂堛垼顓哥粻?
	}

	// 鐠侊紕鐣婚崫鍫濈瑖閻?
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

// GetState 閼惧嘲褰囬悩鑸碘偓?

func (asa *StatefulAction) GetState() *State {
	return asa.State
}

// GetTarget 閼惧嘲褰囬惄顔界垼
func (asa *StatefulAction) GetTarget() IWidget {
	return asa.Target
}

// GetEnabled 閼惧嘲褰囬弰顖氭儊閸氼垳鏁?
func (asa *StatefulAction) GetEnabled() bool {
	if asa.Target == nil {
		return true
	}
	return asa.Target.GetEnabled()
}

// IsValid 閺勵垰鎯侀張澶嬫櫏
func (asa *StatefulAction) IsValid() bool {
	if asa.Target == nil {
		return true
	}
	return !asa.Target.GetBounds().IsEmpty()
}

// SetTarget 鐠佸墽鐤嗛惄顔界垼
func (asa *StatefulAction) SetTarget(widget IWidget) {
	asa.Target = widget
}

// ToOperate 鏉烆剚宕叉稉鐑樻惙娴?

func (asa *StatefulAction) ToOperate() *ActionCommand {
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

// IsTargetEmpty 閻╊喗鐖ｉ弰顖氭儊娑撹櫣鈹?
func (asa *StatefulAction) IsTargetEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

// IsEmpty 閸掋倖鏌囬弰顖氭儊娑撹櫣鈹?
func (asa *StatefulAction) IsEmpty() bool {
	if asa.Target == nil {
		return true
	}
	rect := asa.Target.GetBounds()
	return rect.IsEmpty()
}

// Hash 鐠侊紕鐣婚崫鍫濈瑖閸?

func (asa *StatefulAction) Hash() uintptr {
	return asa.Hashcode
}

// Equal 閸掋倖鏌囬弰顖氭儊閻╁摜鐡?
func (asa *StatefulAction) Equal(other *StatefulAction) bool {
	if other == nil {
		return false
	}
	return asa.Hash() == other.Hash()
}

// Less 濮ｆ棁绶濇径褍鐨?
func (asa *StatefulAction) Less(other *StatefulAction) bool {
	return asa.Hash() < other.Hash()
}

// String 鏉╂柨娲栫€涙顑佹稉鑼躲€冪粈?

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

// NetActionParam 缂冩垹绮堕崝銊ょ稊閸欏倹鏆?
type NetActionParam struct {
	Throttle        int    `json:"throttle"`
	NetActionTaskID int    `json:"net_action_taskid"`
	AlgorithmString string `json:"algorithm_string"`
	PackageName     string `json:"package_name"`
	Token           string `json:"token"`
	DeviceID        string `json:"device_id"`
}

// StatefulActionList PageNameStateAction閹稿洭鎷￠崚鍥╁
type StatefulActionList []*StatefulAction

// StatefulActionSet PageNameStateAction閹稿洭鎷￠梿鍡楁値
type StatefulActionSet map[uintptr]*StatefulAction

// Add 濞ｈ濮為崚浼存肠閸?

func (s StatefulActionSet) Add(action *StatefulAction) {
	if action != nil {
		s[action.Hash()] = action
	}
}

// Remove 娴犲酣娉﹂崥鍫滆厬缁夊娅?
func (s StatefulActionSet) Remove(action *StatefulAction) {
	if action != nil {
		delete(s, action.Hash())
	}
}

// Contains 濡偓閺屻儲妲搁崥锕€瀵橀崥?

func (s StatefulActionSet) Contains(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	_, exists := s[action.Hash()]
	return exists
}

// ToSlice 鏉烆剚宕叉稉鍝勫瀼閻?

func (s StatefulActionSet) ToSlice() StatefulActionList {
	result := make(StatefulActionList, 0, len(s))
	for _, action := range s {
		result = append(result, action)
	}
	return result
}

// SortByPriority 閹稿绱崗鍫㈤獓閹烘帒绨?
func (actions StatefulActionList) SortByPriority() {
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].GetPriority() < actions[j].GetPriority()
	})
}

// FilterByQValue 閹稿閸婅壈绻冨?

func (actions StatefulActionList) FilterByQValue(minQValue float64) StatefulActionList {
	result := make(StatefulActionList, 0)
	for _, action := range actions {
		if action.GetQValue() >= minQValue {
			result = append(result, action)
		}
	}
	return result
}

// GetMaxQValueAction 閼惧嘲褰囬張鈧径顪楅崐鑲╂畱閸斻劋缍?
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

// GetRandomUnvisitedAction 閼惧嘲褰囬梾蹇旀簚閺堫亣顔栭梻顔炬畱閸斻劋缍?
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
