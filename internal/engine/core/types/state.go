package types

import (
	"fmt"
	"math/rand"
	"sort"
	"sync/atomic"
	"time"
	"trek/internal/engine/core/tool"
	"trek/logger"
)

// 鐘舵€佸鐞嗗父閲?

const (
	// STATE_MERGE_DETAIL_TEXT 鏄惁鍚堝苟璇︾粏鏂囨湰
	STATE_MERGE_DETAIL_TEXT = true

	// STATE_WITH_WIDGET_ORDER 鏄惁鍦ㄥ搱甯屼腑鍖呭惈widget椤哄簭
	STATE_WITH_WIDGET_ORDER = false
)

var _ IState = (*State)(nil)

// State 鐘舵€佺粨鏋勶紝StateKey
type State struct {
	Node
	PriorityNodeImpl
	Hashcode      uintptr
	PageName      string
	RootBounds    *Rect
	Actions       StatefulActionList
	Widgets       WidgetList
	MergedWidgets WidgetListMap
	HasNoDetail   bool
	BackAction    *StatefulAction
}

// NewState 鍒涘缓鏂扮殑鐘舵€?

func NewState() *State {
	return &State{
		Node:             *NewNode(),
		PriorityNodeImpl: *NewPriorityNode(),
		PageName:         "",
		RootBounds:       NewRect(0, 0, 0, 0),
		Actions:          make(StatefulActionList, 0),
		Widgets:          make(WidgetList, 0),
		MergedWidgets:    make(WidgetListMap),
		HasNoDetail:      false,
		BackAction:       nil,
	}
}

// NewStateWithPage 鍒涘缓甯︽椿鍔ㄥ悕绉扮殑鐘舵€?

func NewStateWithPage(pageName string) *State {
	logger.Debugf("create state for page: %s", pageName)

	return &State{
		Node:             *NewNode(),
		PriorityNodeImpl: *NewPriorityNode(),
		PageName:         pageName,
		RootBounds:       NewRect(0, 0, 0, 0),
		Actions:          make(StatefulActionList, 0),
		Widgets:          make(WidgetList, 0),
		MergedWidgets:    make(WidgetListMap),
		HasNoDetail:      false,
		BackAction:       nil,
	}
}
func (s *State) GetWidgets() WidgetList {
	return s.Widgets
}

// GetBackAction 鑾峰彇杩斿洖鍔ㄤ綔
func (s *State) GetBackAction() *StatefulAction {
	return s.BackAction
}

// GetActions 鑾峰彇鎵€鏈夊姩浣?

func (s *State) GetActions() StatefulActionList {
	return s.Actions
}

// SetPriority 璁剧疆鐘舵€佷紭鍏堢骇
func (s *State) SetPriority(priority int32) {
	s.PriorityNodeImpl.SetPriority(priority)
}

// GetpageString 鑾峰彇娲诲姩瀛楃涓?

func (s *State) GetPageNameString() string {
	return s.PageName
}

// String 瀹炵幇Stringer鎺ュ彛
func (s *State) String() string {
	return fmt.Sprintf("State{id:%s, page:%s, widgets:%d, actions:%d}",
		s.GetId(), s.PageName, len(s.Widgets), len(s.Actions))
}

// Hash 璁＄畻鍝堝笇鍊?

func (s *State) Hash() uintptr {
	if s.Hashcode == 0 {
		// page鍝堝笇璁＄畻
		pageHash := (tool.HashString(s.PageName) * 31) << 5

		// 璁＄畻widgets鐨勫悎骞跺搱甯?
		widgetsHash := CombineHashWidgets(s.Widgets, STATE_WITH_WIDGET_ORDER)

		// 瀵归綈C++鐗堟湰鐨勬渶缁堝搱甯岃绠?
		pageHash ^= (widgetsHash << 1)
		s.Hashcode = pageHash
	}
	return s.Hashcode
}

// GetMergedWidgets 鑾峰彇鍚堝苟鐨刉idgets
func (s *State) GetMergedWidgets() WidgetListMap {
	return s.MergedWidgets
}

// TargetActions 鑾峰彇鐩爣鍔ㄤ綔
func (s *State) TargetActions() StatefulActionList {
	result := make(StatefulActionList, 0)
	for _, action := range s.Actions {
		if action.RequireTarget() {
			result = append(result, action)
		}
	}
	return result
}

// RandomPickUnvisitedAction 闅忔満閫夋嫨鏈闂殑鍔ㄤ綔
func (s *State) RandomPickUnvisitedAction() IAction {
	action := s.randomPickAction(EnableValidUnvisitedFilter, false)
	if action == nil && EnableValidUnvisitedFilter.Include(s.BackAction) {
		action = s.BackAction
	}
	return action
}

// GreedyPickAction 璐績閫夋嫨鏈€澶у€肩殑鍔ㄤ綔
func (s *State) GreedyPickAction(filter IStatefulActionFilter) IAction {
	if filter == nil {
		filter = EnableValidValuePriorityFilter
	}

	filtered := FilterActions(s.Actions, filter)
	if len(filtered) == 0 {
		return nil
	}

	maxAction := filtered[0]
	maxPriority := filter.GetPriority(maxAction)

	for _, action := range filtered[1:] {
		priority := filter.GetPriority(action)
		if priority > maxPriority {
			maxAction = action
			maxPriority = priority
		}
	}

	return maxAction
}

// RandomPickAction 闅忔満閫夋嫨鍔ㄤ綔
func (s *State) RandomPickAction(filter IStatefulActionFilter, includeBack bool) IAction {
	return s.randomPickAction(filter, includeBack)
}

// randomPickAction 鍐呴儴闅忔満閫夋嫨鍔ㄤ綔鏂规硶
func (s *State) randomPickAction(filter IStatefulActionFilter, includeBack bool) IAction {
	total := s.CountActionPriority(filter, includeBack)
	if total == 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(total)
	return s.pickAction(filter, includeBack, index)
}

// ResolveAt 鍦ㄦ寚瀹氭椂闂磋В鏋愬姩浣?

func (s *State) ResolveAt(action *StatefulAction, t time.Time) *StatefulAction {
	if action == nil {
		return nil
	}

	if action.GetTarget() == nil {
		return action
	}

	targetHash := action.GetTarget().Hash()
	targetWidgets, exists := s.MergedWidgets[targetHash]
	if !exists {
		return action
	}

	total := len(targetWidgets)
	index := int(action.GetVisitedCount()) % total
	logger.Debugf("resolve a merged widget %d/%d for action %s", index, total, action.GetId())

	action.SetTarget(targetWidgets[index])

	return action
}

// ContainsTarget 妫€鏌ユ槸鍚﹀寘鍚洰鏍嘩idget
func (s *State) ContainsTarget(widget IWidget) bool {
	if widget == nil {
		return false
	}

	for _, w := range s.Widgets {
		if w.Hash() == widget.Hash() {
			return true
		}
	}

	// 妫€鏌ュ悎骞剁殑widgets
	for _, widgetList := range s.MergedWidgets {
		for _, w := range widgetList {
			if w.Hash() == widget.Hash() {
				return true
			}
		}
	}

	return false
}

// IsSaturated 妫€鏌ュ姩浣滄槸鍚﹂ケ鍜?

func (s *State) IsSaturated(action *StatefulAction) bool {
	if action == nil {
		return false
	}

	if !action.RequireTarget() {
		return action.IsVisited()
	}

	if action.GetTarget() != nil {
		targetHash := action.GetTarget().Hash()
		if mergedWidgets, exists := s.MergedWidgets[targetHash]; exists {
			return action.GetVisitedCount() > int32(len(mergedWidgets))
		}
	}

	return action.GetVisitedCount() >= 1
}

// Less 姣旇緝澶у皬
func (s *State) Less(other *State) bool {
	if other == nil {
		return true
	}
	return s.Hash() < other.Hash()
}

// Equal 鍒ゆ柇鏄惁鐩哥瓑
func (s *State) Equal(other IState) bool {
	if other == nil {
		return false
	}
	return s.Hash() == other.Hash()
}

// Equals 鍒ゆ柇鏄惁鐩哥瓑锛堝埆鍚嶆柟娉曪級
func (s *State) Equals(other IState) bool {
	return s.Equal(other)
}

func (s *State) ClearDetails() {
	for _, widget := range s.Widgets {
		widget.ClearDetails()
	}
	s.MergedWidgets = make(WidgetListMap)
	s.HasNoDetail = true
	s.Hashcode = 0
}

// FillDetails 濉厖璇︾粏淇℃伅
func (s *State) FillDetails(copyIState IState) {
	if copyIState == nil {
		return
	}

	if copyState, ok := copyIState.(*State); ok {
		s.HasNoDetail = copyState.HasNoDetail
		s.Widgets = make(WidgetList, len(copyState.Widgets))
		for i, widget := range copyState.Widgets {
			s.Widgets[i] = widget
		}

		s.MergedWidgets = make(WidgetListMap)
		for hash, widgetList := range copyState.MergedWidgets {
			newList := make(WidgetList, len(widgetList))
			for i, widget := range widgetList {
				newList[i] = widget
			}
			s.MergedWidgets[hash] = newList
		}

		s.Hashcode = copyState.Hashcode
	}
}

func (s *State) HasDetail() bool {
	return !s.HasNoDetail
}

// GetId 鑾峰彇ID
func (s *State) GetId() string {
	return "g0s" + s.Node.GetId()
}

// Visit 鏇存柊璁块棶璁℃暟
func (s *State) Visit(timestamp time.Time) {
	atomic.AddInt32(&s.Node.VisitedCount, 1)
}

// CountActionPriority 璁＄畻鍔ㄤ綔浼樺厛绾?

func (s *State) CountActionPriority(filter IStatefulActionFilter, includeBack bool) int {
	if filter == nil {
		filter = EnableValidFilter
	}

	totalP := 0
	count := 0
	for _, action := range s.Actions {
		if !includeBack && action.IsBack() {
			continue
		}
		included := filter.Include(action)
		if included {
			fp := filter.GetPriority(action)
			if fp <= 0 {
				logger.Debugf("Error: Action should has a positive priority, but we get %d", fp)
				continue
			}
			totalP += int(fp)
			count++
		}
	}

	return totalP
}

// MergeWidgetAndStoreMergedOnes 鍚堝苟Widget骞跺瓨鍌ㄥ悎骞剁殑
func (s *State) MergeWidgetAndStoreMergedOnes(mergeWidgets WidgetSet) int {
	mergedWidgetCount := 0

	// 妫€鏌TATE_MERGE_DETAIL_TEXT鏍囧織鍜寃idgets鏄惁涓虹┖
	if STATE_MERGE_DETAIL_TEXT && len(s.Widgets) > 0 {
		for _, widgetPtr := range s.Widgets {
			// 灏濊瘯灏唚idget鎻掑叆鍒癿ergeWidgets set涓?			// 濡傛灉widget宸插瓨鍦紝鍒檔oMerged涓篺alse
			_, exists := mergeWidgets[widgetPtr.Hash()]
			if !exists {
				// 绗竴娆″嚭鐜帮紝娣诲姞鍒皊et涓?
				mergeWidgets[widgetPtr.Hash()] = widgetPtr
			} else {
				h := widgetPtr.Hash()
				mergedWidgetCount++

				if _, exists := s.MergedWidgets[h]; !exists {
					s.MergedWidgets[h] = WidgetList{widgetPtr}
				} else {
					s.MergedWidgets[h] = append(s.MergedWidgets[h], widgetPtr)
				}
			}
		}
	}

	return mergedWidgetCount
}

// CombineHashWidgets 鍚堝苟widget鍝堝笇 - 瀵归綈C++鐗堟湰鐨刢ombineHash
func CombineHashWidgets(widgets WidgetList, withOrder bool) uintptr {
	count := len(widgets)
	combinedHashcode := uintptr(0x1)

	for i := 0; i < count; i++ {
		widget := widgets[i]
		if widget != nil {
			combinedHashcode ^= widget.Hash()
			if withOrder {
				combinedHashcode ^= uintptr(127 * (i << 6))
			}
		}
	}

	return combinedHashcode
}

// BuildFromElement 浠嶦lement鏋勫缓
func (s *State) BuildFromElement(parentWidget IWidget, elem IElement) {
	if elem == nil {
		return
	}

	// 澶勭悊rootBounds閫昏緫锛屽榻怌++鐗堟湰
	if elem.GetParent() == nil && !elem.GetBounds().IsEmpty() {
		if SameRootBounds.IsEmpty() {
			SameRootBounds = elem.GetBounds()
		}
		if SameRootBounds.Equal(elem.GetBounds()) {
			s.RootBounds = SameRootBounds
		} else {
			s.RootBounds = elem.GetBounds()
		}
	}

	// 鍒涘缓鏂扮殑widget
	widget := NewWidget(parentWidget, elem)

	// 娣诲姞鍒皐idgets鍒楄〃
	s.Widgets = append(s.Widgets, widget)

	// 閫掑綊澶勭悊瀛愬厓绱?
	for _, childElement := range elem.GetChildren() {
		s.BuildFromElement(widget, childElement)
	}
}

// pickAction 閫夋嫨鍔ㄤ綔
func (s *State) pickAction(filter IStatefulActionFilter, includeBack bool, index int) IAction {
	if filter == nil {
		filter = EnableValidFilter
	}

	ii := index
	for _, action := range s.Actions {
		if !includeBack && action.IsBack() {
			continue
		}
		if filter.Include(action) {
			p := filter.GetPriority(action)
			if p > int32(ii) {
				return action
			} else {
				ii = ii - int(p)
			}
		}
	}

	logger.Debugf("ERROR: action filter is unstable")
	return nil
}

// Create 鍒涘缓State
func Create(elem IElement, pageName string) *State {
	state := NewStateWithPage(pageName)

	if elem != nil {
		state.RootBounds = elem.GetBounds()
		state.BuildFromElement(nil, elem)
	}

	// 璁＄畻page鍝堝笇
	pageHash := (tool.HashString(state.PageName) * 31) << 5

	// 鍚堝苟widgets
	mergedWidgets := make(WidgetSet)
	mergedWidgetCount := state.MergeWidgetAndStoreMergedOnes(mergedWidgets)
	if mergedWidgetCount != 0 {
		logger.Debugf("build state merged %d widget", mergedWidgetCount)
		state.Widgets = make(WidgetList, 0, len(mergedWidgets))
		for _, widget := range mergedWidgets {
			state.Widgets = append(state.Widgets, widget)
		}
	}

	// 璁＄畻鏈€缁堝搱甯?
	pageHash ^= (CombineHashWidgets(state.Widgets, STATE_WITH_WIDGET_ORDER) << 1)
	state.Hashcode = uintptr(pageHash)

	// 涓烘瘡涓獁idget鍒涘缓鍔ㄤ綔锛堟坊鍔犲幓閲嶉€昏緫锛?
	actionHashSet := make(map[uintptr]bool)
	for _, widget := range state.Widgets {
		if widget.GetBounds() == nil {
			logger.Errorf("NULL Bounds happened")
			continue
		}
		if widget.GetBounds().IsEmpty() {
			continue
		}
		actions := widget.GetActions()
		for _, actionType := range actions {
			action := NewStatefulAction(state, widget, actionType)

			// 鍘婚噸锛氭鏌ユ槸鍚﹀凡缁忓瓨鍦ㄧ浉鍚屽搱甯屽€肩殑action
			actionHash := action.Hash()
			if actionHashSet[actionHash] {

				continue
			}
			actionHashSet[actionHash] = true

			state.Actions = append(state.Actions, action)
		}
	}

	// 鍒涘缓杩斿洖鍔ㄤ綔
	backAction := NewStatefulAction(state, nil, BACK)
	state.BackAction = backAction
	state.Actions = append(state.Actions, backAction)

	return state
}

// StateList State鎺ュ彛鍒囩墖
type StateList []IState

// StateSet State鎺ュ彛闆嗗悎
type StateSet map[uintptr]IState

// Add 娣诲姞鍒伴泦鍚?

func (s StateSet) Add(state IState) {
	if state != nil {
		s[state.Hash()] = state
	}
}

// Remove 浠庨泦鍚堜腑绉婚櫎
func (s StateSet) Remove(state IState) {
	if state != nil {
		delete(s, state.Hash())
	}
}

// Contains 妫€鏌ユ槸鍚﹀寘鍚?

func (s StateSet) Contains(state IState) bool {
	if state == nil {
		return false
	}
	_, exists := s[state.Hash()]
	return exists
}

// ToSlice 杞崲涓哄垏鐗?

func (s StateSet) ToSlice() StateList {
	result := make(StateList, 0, len(s))
	for _, state := range s {
		result = append(result, state)
	}
	return result
}

// SortByPriority 鎸変紭鍏堢骇鎺掑簭
func (states StateList) SortByPriority() {
	sort.Slice(states, func(i, j int) bool {
		return states[i].GetPriority() < states[j].GetPriority()
	})
}

// FilterByPage 鎸夋椿鍔ㄥ悕绉拌繃婊?

func (states StateList) FilterByPage(page string) StateList {
	result := make(StateList, 0)
	for _, state := range states {
		if state.GetPageNameString() == page {
			result = append(result, state)
		}
	}
	return result
}

// GetByPage 鏍规嵁娲诲姩鍚嶇О鑾峰彇鐘舵€?

func (states StateList) GetByPage(page string) IState {
	for _, state := range states {
		if state.GetPageNameString() == page {
			return state
		}
	}
	return nil
}

// 鍏ㄥ眬鍙橀噺
var (
	SameRootBounds = NewRect(0, 0, 0, 0)
)
