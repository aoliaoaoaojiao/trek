package reuse

import (
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision/shared/tool"
	"trek/logger"
)

type ReuseState struct {
	pageName string
	types.State
}

func NewReuseState(pageName string) *ReuseState {
	baseState := types.NewStateWithPage(pageName)
	rs := &ReuseState{
		pageName: pageName,
		State:    *baseState,
	}
	rs.HasNoDetail = false
	return rs
}

func Create(pageName string, element types.IElement) *ReuseState {
	state := NewReuseState(pageName)
	logger.Debugf("Creating ReuseState for page: %s", pageName)
	state.buildState(element)
	return state
}

func (rs *ReuseState) buildState(element types.IElement) {
	rs.buildStateFromElement(nil, element)
	rs.mergeWidgetsInState()
	rs.buildHashForState()
	rs.buildActionForState()
}

func (rs *ReuseState) buildStateFromElement(parentWidget *RichWidget, element types.IElement) {
	rs.buildBoundingBox(element)

	widget := NewRichWidget(parentWidget, element)
	rs.Widgets = append(rs.Widgets, &widget.Widget)
	logger.Debugf("Added RichWidget to state, total widgets now: %d, widget=%s", len(rs.Widgets), widget.String())

	for _, child := range element.GetChildren() {
		rs.buildFromElement(widget, child)
	}
}

func (rs *ReuseState) buildFromElement(parentWidget *RichWidget, elem types.IElement) {
	rs.buildBoundingBox(elem)

	var parentWidgetPtr *types.Widget
	if parentWidget != nil {
		parentWidgetPtr = &parentWidget.Widget
	}

	widget := types.NewWidget(parentWidgetPtr, elem)
	rs.Widgets = append(rs.Widgets, widget)
	logger.Debugf("Added Widget to state, total widgets now: %d, widget=%s", len(rs.Widgets), widget.String())

	for _, child := range elem.GetChildren() {
		rs.buildFromElement(parentWidget, child)
	}
}

func (rs *ReuseState) buildBoundingBox(element types.IElement) {
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

func (rs *ReuseState) buildHashForState() {
	pageHash := (tool.HashString(rs.PageName) * 31) << 5
	pageHash ^= (types.CombineHashWidgets(rs.Widgets, types.STATE_WITH_WIDGET_ORDER) << 1)
	rs.Hashcode = pageHash
}

func (rs *ReuseState) buildActionForState() {
	for _, widget := range rs.Widgets {
		if widget.GetBounds() == nil {
			logger.Errorf("NULL Bounds happened")
			continue
		}

		for _, action := range widget.GetActions() {
			pageNameAction := NewPageNameAction(rs.PageName, widget, action)
			rs.Actions = append(rs.Actions, &pageNameAction.StatefulAction)
			logger.Debugf("Added action to state, total actions now: %d", len(rs.Actions))
		}
	}

	backAction := NewPageNameAction(rs.PageName, nil, types.BACK)
	rs.Actions = append(rs.Actions, &backAction.StatefulAction)
	logger.Debugf("Added back action to state, total actions now: %d", len(rs.Actions))
}

func (rs *ReuseState) mergeWidgetsInState() {
	mergedWidgets := make(types.WidgetSet)
	mergedCount := rs.MergeWidgetAndStoreMergedOnes(mergedWidgets)
	logger.Debugf("MergeWidgetAndStoreMergedOnes merged %d widgets", mergedCount)
}
