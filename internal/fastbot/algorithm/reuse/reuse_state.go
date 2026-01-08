package reuse

import (
	"Trek/internal/fastbot/core/types"
	"Trek/log"
)

// ReuseState 重用状态类，持有所有Widgets及其关联的动作等
type ReuseState struct {
	pageName string
	types.State
}

// NewReuseState 创建新的ReuseState
func NewReuseState(pageName string) *ReuseState {
	baseState := types.NewStateWithPage(pageName)

	rs := &ReuseState{
		pageName: pageName,
		State:    *baseState,
	}

	rs.HasNoDetail = false
	return rs
}

// Create 根据元素和活动名称创建ReuseState
func Create(pageName string, element *types.Element) *ReuseState {

	statePointer := NewReuseState(pageName)
	log.Debugf("Creating ReuseState for page: %s", pageName)
	statePointer.buildState(element)

	return statePointer
}

// buildState 构建状态
func (rs *ReuseState) buildState(element *types.Element) {
	rs.buildStateFromElement(nil, element)
	rs.mergeWidgetsInState()
	rs.buildHashForState()
	rs.buildActionForState()
}

// buildStateFromElement 从元素构建状态
func (rs *ReuseState) buildStateFromElement(parentWidget *RichWidget, element *types.Element) {
	rs.buildBoundingBox(element)

	// 使用RichWidget构建状态
	widget := NewRichWidget(parentWidget, element)
	rs.Widgets = append(rs.Widgets, &widget.Widget)
	log.Debugf("Added RichWidget to state, total widgets now: %d", len(rs.Widgets))

	for _, childElement := range element.GetChildren() {
		rs.buildFromElement(widget, childElement)
	}
}

// buildFromElement 从元素构建
func (rs *ReuseState) buildFromElement(parentWidget *RichWidget, elem *types.Element) {
	rs.buildBoundingBox(elem)

	var parentWidgetPtr *types.Widget
	if parentWidget != nil {
		parentWidgetPtr = &parentWidget.Widget
	}

	widget := types.NewWidget(parentWidgetPtr, elem)
	rs.Widgets = append(rs.Widgets, widget)
	log.Debugf("Added Widget to state, total widgets now: %d", len(rs.Widgets))

	for _, childElement := range elem.GetChildren() {
		rs.buildFromElement(parentWidget, childElement)
	}
}

// buildBoundingBox 构建边界框
func (rs *ReuseState) buildBoundingBox(element *types.Element) {
	if element.GetParent() == nil && !element.GetBounds().IsEmpty() {
		if types.SameRootBounds.IsEmpty() && element != nil {
			types.SameRootBounds = element.GetBounds()
		}
		if types.SameRootBounds.Equal(element.GetBounds()) {
			rs.RootBounds = types.SameRootBounds
		} else {
			rs.RootBounds = element.GetBounds()
		}
	}
}

// buildHashForState 构建状态哈希
func (rs *ReuseState) buildHashForState() {
	// 构建哈希
	pageString := rs.PageName
	pageHash := (HashString(pageString) * 31) << 5
	pageHash ^= (CombineHashWidgets(rs.Widgets, types.STATE_WITH_WIDGET_ORDER) << 1)
	rs.Hashcode = pageHash

}

// buildActionForState 构建状态动作
func (rs *ReuseState) buildActionForState() {
	for _, widget := range rs.Widgets {
		if widget.GetBounds() == nil {
			log.Errorf("NULL Bounds happened")
			continue
		}

		actions := widget.GetActions()
		for _, action := range actions {
			pageNameAction := NewPageNameAction(rs.PageName, widget, action)
			rs.Actions = append(rs.Actions, &pageNameAction.StatefulAction)
			log.Debugf("Added action to state, total actions now: %d", len(rs.Actions))
		}
	}

	// 创建返回动作
	backAction := NewPageNameAction(rs.PageName, nil, types.BACK)
	rs.BackAction = &backAction.StatefulAction
	rs.Actions = append(rs.Actions, rs.BackAction)
	log.Debugf("Added back action to state, total actions now: %d", len(rs.Actions))
}

// mergeWidgetsInState 合并状态中的Widgets
func (rs *ReuseState) mergeWidgetsInState() {
	mergedWidgets := make(types.WidgetSet)
	mergedCount := rs.MergeWidgetAndStoreMergedOnes(mergedWidgets)

	if mergedCount != 0 {
		log.Debugf("build state merged %d widget", mergedCount)
		rs.Widgets = make(types.WidgetList, 0, len(mergedWidgets))
		for _, widget := range mergedWidgets {
			rs.Widgets = append(rs.Widgets, widget)
		}
	}
}
