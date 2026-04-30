package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type State = coretypes.State
type StateList = coretypes.StateList
type StateSet = coretypes.StateSet

const (
	STATE_MERGE_DETAIL_TEXT  = coretypes.STATE_MERGE_DETAIL_TEXT
	STATE_WITH_WIDGET_ORDER = coretypes.STATE_WITH_WIDGET_ORDER
)

var SameRootBounds = coretypes.SameRootBounds

func NewState() *State                                   { return coretypes.NewState() }
func NewStateWithPage(pageName string) *State             { return coretypes.NewStateWithPage(pageName) }
func Create(elem IElement, pageName string) *State        { return coretypes.Create(elem, pageName) }
func CombineHashWidgets(widgets WidgetList, withOrder bool) uintptr {
	return coretypes.CombineHashWidgets(widgets, withOrder)
}
