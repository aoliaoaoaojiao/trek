package reuse

import (
	types2 "trek/internal/engine/decision/shared/types"
	"trek/logger"
)

type ReuseState struct {
	pageName string
	types2.State
}

func NewReuseState(pageName string) *ReuseState {
	baseState := types2.NewStateWithPage(pageName)
	rs := &ReuseState{
		pageName: pageName,
		State:    *baseState,
	}
	rs.HasNoDetail = false
	return rs
}

func Create(pageName string, element types2.IElement) *ReuseState {
	state := NewReuseState(pageName)
	logger.Debugf("Creating ReuseState for page: %s", pageName)
	state.buildState(element)
	return state
}

func (rs *ReuseState) buildState(element types2.IElement) {
	rs.buildStateFromElement(nil, element)
	rs.mergeWidgetsInState()
	rs.buildHashForState()
	rs.buildActionForState()
}

func (rs *ReuseState) buildStateFromElement(parentWidget *RichWidget, element types2.IElement) {
	rs.buildBoundingBox(element)

	widget := NewRichWidget(parentWidget, element)
	rs.Widgets = append(rs.Widgets, &widget.Widget)
	logger.Debugf("Added RichWidget to state, total widgets now: %d, widget=%s", len(rs.Widgets), widget.String())

	for _, child := range element.GetChildren() {
		rs.buildFromElement(widget, child)
	}
}

func (rs *ReuseState) buildFromElement(parentWidget *RichWidget, elem types2.IElement) {
	rs.buildBoundingBox(elem)

	var parentWidgetPtr *types2.Widget
	if parentWidget != nil {
		parentWidgetPtr = &parentWidget.Widget
	}

	widget := types2.NewWidget(parentWidgetPtr, elem)
	rs.Widgets = append(rs.Widgets, widget)
	logger.Debugf("Added Widget to state, total widgets now: %d, widget=%s", len(rs.Widgets), widget.String())

	for _, child := range elem.GetChildren() {
		rs.buildFromElement(parentWidget, child)
	}
}

func (rs *ReuseState) buildBoundingBox(element types2.IElement) {
	if element.GetParent() == nil && !element.GetBounds().IsEmpty() {
		if types2.SameRootBounds.IsEmpty() && element != nil {
			types2.SameRootBounds = element.GetBounds()
		}
		if types2.SameRootBounds.Equal(element.GetBounds()) {
			rs.RootBounds = types2.SameRootBounds
		} else {
			rs.RootBounds = element.GetBounds()
		}
	}
}

func (rs *ReuseState) buildHashForState() {
	pageHash := (HashString(rs.PageName) * 31) << 5
	pageHash ^= (CombineHashWidgets(rs.Widgets, types2.STATE_WITH_WIDGET_ORDER) << 1)
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

	backAction := NewPageNameAction(rs.PageName, nil, types2.BACK)
	rs.Actions = append(rs.Actions, &backAction.StatefulAction)
	logger.Debugf("Added back action to state, total actions now: %d", len(rs.Actions))
}

func (rs *ReuseState) mergeWidgetsInState() {
	mergedWidgets := make(types2.WidgetSet)
	mergedCount := rs.MergeWidgetAndStoreMergedOnes(mergedWidgets)
	logger.Debugf("MergeWidgetAndStoreMergedOnes merged %d widgets", mergedCount)
}
