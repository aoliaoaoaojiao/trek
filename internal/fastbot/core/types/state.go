package types

import (
	"Trek/internal/fastbot/tool"
	"Trek/log"
	"fmt"
	"math/rand"
	"sort"
	"sync/atomic"
	"time"
)

// 状态处理常量
const (
	// STATE_MERGE_DETAIL_TEXT 是否合并详细文本
	STATE_MERGE_DETAIL_TEXT = true

	// STATE_WITH_WIDGET_ORDER 是否在哈希中包含widget顺序
	STATE_WITH_WIDGET_ORDER = false
)

var _ IState = (*State)(nil)

// State 状态结构，StateKey
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

// NewState 创建新的状态
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

// NewStateWithPage 创建带活动名称的状态
func NewStateWithPage(pageName string) *State {
	log.Debugf("create state for page: %s", pageName)

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

// GetBackAction 获取返回动作
func (s *State) GetBackAction() *StatefulAction {
	return s.BackAction
}

// GetActions 获取所有动作
func (s *State) GetActions() StatefulActionList {
	return s.Actions
}

// SetPriority 设置状态优先级
func (s *State) SetPriority(priority int32) {
	s.PriorityNodeImpl.SetPriority(priority)
}

// GetpageString 获取活动字符串
func (s *State) GetPageNameString() string {
	return s.PageName
}

// String 实现Stringer接口
func (s *State) String() string {
	return fmt.Sprintf("State{id:%s, page:%s, widgets:%d, actions:%d}",
		s.GetId(), s.PageName, len(s.Widgets), len(s.Actions))
}

// Hash 计算哈希值
func (s *State) Hash() uintptr {
	if s.Hashcode == 0 {
		// page哈希计算
		pageHash := (tool.HashString(s.PageName) * 31) << 5

		// 计算widgets的合并哈希
		widgetsHash := CombineHashWidgets(s.Widgets, STATE_WITH_WIDGET_ORDER)

		// 对齐C++版本的最终哈希计算
		pageHash ^= (widgetsHash << 1)
		s.Hashcode = pageHash
	}
	return s.Hashcode
}

// GetMergedWidgets 获取合并的Widgets
func (s *State) GetMergedWidgets() WidgetListMap {
	return s.MergedWidgets
}

// TargetActions 获取目标动作
func (s *State) TargetActions() StatefulActionList {
	result := make(StatefulActionList, 0)
	for _, action := range s.Actions {
		if action.RequireTarget() {
			result = append(result, action)
		}
	}
	return result
}

// RandomPickUnvisitedAction 随机选择未访问的动作
func (s *State) RandomPickUnvisitedAction() IAction {
	action := s.randomPickAction(EnableValidUnvisitedFilter, false)
	if action == nil && EnableValidUnvisitedFilter.Include(s.BackAction) {
		action = s.BackAction
	}
	return action
}

// GreedyPickAction 贪心选择最大值的动作
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

// RandomPickAction 随机选择动作
func (s *State) RandomPickAction(filter IStatefulActionFilter, includeBack bool) IAction {
	return s.randomPickAction(filter, includeBack)
}

// randomPickAction 内部随机选择动作方法
func (s *State) randomPickAction(filter IStatefulActionFilter, includeBack bool) IAction {
	total := s.CountActionPriority(filter, includeBack)
	if total == 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(total)
	return s.pickAction(filter, includeBack, index)
}

// ResolveAt 在指定时间解析动作
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
	log.Debugf("resolve a merged widget %d/%d for action %s", index, total, action.GetId())

	action.SetTarget(targetWidgets[index])

	return action
}

// ContainsTarget 检查是否包含目标Widget
func (s *State) ContainsTarget(widget *Widget) bool {
	if widget == nil {
		return false
	}

	for _, w := range s.Widgets {
		if w.Hash() == widget.Hash() {
			return true
		}
	}

	// 检查合并的widgets
	for _, widgetList := range s.MergedWidgets {
		for _, w := range widgetList {
			if w.Hash() == widget.Hash() {
				return true
			}
		}
	}

	return false
}

// IsSaturated 检查动作是否饱和
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

// Less 比较大小
func (s *State) Less(other *State) bool {
	if other == nil {
		return true
	}
	return s.Hash() < other.Hash()
}

// Equal 判断是否相等
func (s *State) Equal(other IState) bool {
	if other == nil {
		return false
	}
	return s.Hash() == other.Hash()
}

// Equals 判断是否相等（别名方法）
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

// FillDetails 填充详细信息
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

// GetId 获取ID
func (s *State) GetId() string {
	return "g0s" + s.Node.GetId()
}

// Visit 更新访问计数
func (s *State) Visit(timestamp time.Time) {
	atomic.AddInt32(&s.Node.VisitedCount, 1)
}

// CountActionPriority 计算动作优先级
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
				log.Debugf("Error: Action should has a positive priority, but we get %d", fp)
				continue
			}
			totalP += int(fp)
			count++
		}
	}

	return totalP
}

// MergeWidgetAndStoreMergedOnes 合并Widget并存储合并的
func (s *State) MergeWidgetAndStoreMergedOnes(mergeWidgets WidgetSet) int {
	mergedWidgetCount := 0

	// 检查STATE_MERGE_DETAIL_TEXT标志和widgets是否为空
	if STATE_MERGE_DETAIL_TEXT && len(s.Widgets) > 0 {
		for _, widgetPtr := range s.Widgets {
			// 尝试将widget插入到mergeWidgets set中
			// 如果widget已存在，则noMerged为false
			_, exists := mergeWidgets[widgetPtr.Hash()]
			if !exists {
				// 第一次出现，添加到set中
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

// CombineHashWidgets 合并widget哈希 - 对齐C++版本的combineHash
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

// BuildFromElement 从Element构建
func (s *State) BuildFromElement(parentWidget *Widget, elem *Element) {
	if elem == nil {
		return
	}

	// 处理rootBounds逻辑，对齐C++版本
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

	// 创建新的widget
	widget := NewWidget(parentWidget, elem)

	// 添加到widgets列表
	s.Widgets = append(s.Widgets, widget)

	// 递归处理子元素
	for _, childElement := range elem.GetChildren() {
		s.BuildFromElement(widget, childElement)
	}
}

// pickAction 选择动作
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

	log.Debugf("ERROR: action filter is unstable")
	return nil
}

// Create 创建State
func Create(elem *Element, pageName string) *State {
	state := NewStateWithPage(pageName)

	if elem != nil {
		state.RootBounds = elem.GetBounds()
		state.BuildFromElement(nil, elem)
	}

	// 计算page哈希
	pageHash := (tool.HashString(state.PageName) * 31) << 5

	// 合并widgets
	mergedWidgets := make(WidgetSet)
	mergedWidgetCount := state.MergeWidgetAndStoreMergedOnes(mergedWidgets)
	if mergedWidgetCount != 0 {
		log.Debugf("build state merged %d widget", mergedWidgetCount)
		state.Widgets = make(WidgetList, 0, len(mergedWidgets))
		for _, widget := range mergedWidgets {
			state.Widgets = append(state.Widgets, widget)
		}
	}

	// 计算最终哈希
	pageHash ^= (CombineHashWidgets(state.Widgets, STATE_WITH_WIDGET_ORDER) << 1)
	state.Hashcode = uintptr(pageHash)

	// 为每个widget创建动作（添加去重逻辑）
	actionHashSet := make(map[uintptr]bool)
	for _, widget := range state.Widgets {
		if widget.GetBounds() == nil {
			log.Errorf("NULL Bounds happened")
			continue
		}
		if widget.GetBounds().IsEmpty() {
			continue
		}
		actions := widget.GetActions()
		for _, actionType := range actions {
			action := NewStatefulAction(state, widget, actionType)

			// 去重：检查是否已经存在相同哈希值的action
			actionHash := action.Hash()
			if actionHashSet[actionHash] {

				continue
			}
			actionHashSet[actionHash] = true

			state.Actions = append(state.Actions, action)
		}
	}

	// 创建返回动作
	backAction := NewStatefulAction(state, nil, BACK)
	state.BackAction = backAction
	state.Actions = append(state.Actions, backAction)

	return state
}

// StateList State接口切片
type StateList []IState

// StateSet State接口集合
type StateSet map[uintptr]IState

// Add 添加到集合
func (s StateSet) Add(state IState) {
	if state != nil {
		s[state.Hash()] = state
	}
}

// Remove 从集合中移除
func (s StateSet) Remove(state IState) {
	if state != nil {
		delete(s, state.Hash())
	}
}

// Contains 检查是否包含
func (s StateSet) Contains(state IState) bool {
	if state == nil {
		return false
	}
	_, exists := s[state.Hash()]
	return exists
}

// ToSlice 转换为切片
func (s StateSet) ToSlice() StateList {
	result := make(StateList, 0, len(s))
	for _, state := range s {
		result = append(result, state)
	}
	return result
}

// SortByPriority 按优先级排序
func (states StateList) SortByPriority() {
	sort.Slice(states, func(i, j int) bool {
		return states[i].GetPriority() < states[j].GetPriority()
	})
}

// FilterByPage 按活动名称过滤
func (states StateList) FilterByPage(page string) StateList {
	result := make(StateList, 0)
	for _, state := range states {
		if state.GetPageNameString() == page {
			result = append(result, state)
		}
	}
	return result
}

// GetByPage 根据活动名称获取状态
func (states StateList) GetByPage(page string) IState {
	for _, state := range states {
		if state.GetPageNameString() == page {
			return state
		}
	}
	return nil
}

// 全局变量
var (
	SameRootBounds = NewRect(0, 0, 0, 0)
)
