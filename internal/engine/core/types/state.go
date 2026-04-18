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

const (
	STATE_MERGE_DETAIL_TEXT = true

	STATE_WITH_WIDGET_ORDER = false
)

var _ IState = (*State)(nil)

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

func (s *State) GetBackAction() *StatefulAction {
	return s.BackAction
}

func (s *State) GetActions() StatefulActionList {
	return s.Actions
}

func (s *State) SetPriority(priority int32) {
	s.PriorityNodeImpl.SetPriority(priority)
}

func (s *State) GetPageNameString() string {
	return s.PageName
}

func (s *State) String() string {
	return fmt.Sprintf("State{id:%s, page:%s, widgets:%d, actions:%d}",
		s.GetId(), s.PageName, len(s.Widgets), len(s.Actions))
}

func (s *State) Hash() uintptr {
	if s.Hashcode == 0 {
		pageHash := (tool.HashString(s.PageName) * 31) << 5

		widgetsHash := CombineHashWidgets(s.Widgets, STATE_WITH_WIDGET_ORDER)

		pageHash ^= (widgetsHash << 1)
		s.Hashcode = pageHash
	}
	return s.Hashcode
}

func (s *State) GetMergedWidgets() WidgetListMap {
	return s.MergedWidgets
}

func (s *State) TargetActions() StatefulActionList {
	result := make(StatefulActionList, 0)
	for _, action := range s.Actions {
		if action.RequireTarget() {
			result = append(result, action)
		}
	}
	return result
}

func (s *State) RandomPickUnvisitedAction() IAction {
	action := s.randomPickAction(EnableValidUnvisitedFilter, false)
	if action == nil && EnableValidUnvisitedFilter.Include(s.BackAction) {
		action = s.BackAction
	}
	return action
}

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

func (s *State) RandomPickAction(filter IStatefulActionFilter, includeBack bool) IAction {
	return s.randomPickAction(filter, includeBack)
}

func (s *State) randomPickAction(filter IStatefulActionFilter, includeBack bool) IAction {
	total := s.CountActionPriority(filter, includeBack)
	if total == 0 {
		return nil
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(total)
	return s.pickAction(filter, includeBack, index)
}

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

func (s *State) ContainsTarget(widget IWidget) bool {
	if widget == nil {
		return false
	}

	for _, w := range s.Widgets {
		if w.Hash() == widget.Hash() {
			return true
		}
	}

	for _, widgetList := range s.MergedWidgets {
		for _, w := range widgetList {
			if w.Hash() == widget.Hash() {
				return true
			}
		}
	}

	return false
}

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

func (s *State) Less(other *State) bool {
	if other == nil {
		return true
	}
	return s.Hash() < other.Hash()
}

func (s *State) Equal(other IState) bool {
	if other == nil {
		return false
	}
	return s.Hash() == other.Hash()
}

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

func (s *State) GetId() string {
	return "g0s" + s.Node.GetId()
}

func (s *State) Visit(timestamp time.Time) {
	atomic.AddInt32(&s.Node.VisitedCount, 1)
}

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

func (s *State) MergeWidgetAndStoreMergedOnes(mergeWidgets WidgetSet) int {
	mergedWidgetCount := 0

	if STATE_MERGE_DETAIL_TEXT && len(s.Widgets) > 0 {
		for _, widgetPtr := range s.Widgets {
			_, exists := mergeWidgets[widgetPtr.Hash()]
			if !exists {
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

func (s *State) BuildFromElement(parentWidget IWidget, elem IElement) {
	if elem == nil {
		return
	}

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

	widget := NewWidget(parentWidget, elem)

	s.Widgets = append(s.Widgets, widget)

	for _, childElement := range elem.GetChildren() {
		s.BuildFromElement(widget, childElement)
	}
}

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

func Create(elem IElement, pageName string) *State {
	state := NewStateWithPage(pageName)

	if elem != nil {
		state.RootBounds = elem.GetBounds()
		state.BuildFromElement(nil, elem)
	}

	pageHash := (tool.HashString(state.PageName) * 31) << 5

	mergedWidgets := make(WidgetSet)
	mergedWidgetCount := state.MergeWidgetAndStoreMergedOnes(mergedWidgets)
	if mergedWidgetCount != 0 {
		logger.Debugf("build state merged %d widget", mergedWidgetCount)
		state.Widgets = make(WidgetList, 0, len(mergedWidgets))
		for _, widget := range mergedWidgets {
			state.Widgets = append(state.Widgets, widget)
		}
	}

	pageHash ^= (CombineHashWidgets(state.Widgets, STATE_WITH_WIDGET_ORDER) << 1)
	state.Hashcode = uintptr(pageHash)

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

			actionHash := action.Hash()
			if actionHashSet[actionHash] {

				continue
			}
			actionHashSet[actionHash] = true

			state.Actions = append(state.Actions, action)
		}
	}

	backAction := NewStatefulAction(state, nil, BACK)
	state.BackAction = backAction
	state.Actions = append(state.Actions, backAction)

	return state
}

type StateList []IState

type StateSet map[uintptr]IState

func (s StateSet) Add(state IState) {
	if state != nil {
		s[state.Hash()] = state
	}
}

func (s StateSet) Remove(state IState) {
	if state != nil {
		delete(s, state.Hash())
	}
}

func (s StateSet) Contains(state IState) bool {
	if state == nil {
		return false
	}
	_, exists := s[state.Hash()]
	return exists
}

func (s StateSet) ToSlice() StateList {
	result := make(StateList, 0, len(s))
	for _, state := range s {
		result = append(result, state)
	}
	return result
}

func (states StateList) SortByPriority() {
	sort.Slice(states, func(i, j int) bool {
		return states[i].GetPriority() < states[j].GetPriority()
	})
}

func (states StateList) FilterByPage(page string) StateList {
	result := make(StateList, 0)
	for _, state := range states {
		if state.GetPageNameString() == page {
			result = append(result, state)
		}
	}
	return result
}

func (states StateList) GetByPage(page string) IState {
	for _, state := range states {
		if state.GetPageNameString() == page {
			return state
		}
	}
	return nil
}

var (
	SameRootBounds = NewRect(0, 0, 0, 0)
)
