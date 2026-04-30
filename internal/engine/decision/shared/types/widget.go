package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type Widget = coretypes.Widget
type WidgetList = coretypes.WidgetList
type WidgetSet = coretypes.WidgetSet
type WidgetListMap = coretypes.WidgetListMap

const (
	STATE_WITH_TEXT              = coretypes.STATE_WITH_TEXT
	STATE_TEXT_MAX_LEN           = coretypes.STATE_TEXT_MAX_LEN
	STATE_WITH_INDEX             = coretypes.STATE_WITH_INDEX
	SCROLL_BOTTOM_UP_N_ENABLE    = coretypes.SCROLL_BOTTOM_UP_N_ENABLE
	FORCE_EDITTEXT_CLICK_TRUE    = coretypes.FORCE_EDITTEXT_CLICK_TRUE
	PARENT_CLICK_CHANGE_CHILDREN = coretypes.PARENT_CLICK_CHANGE_CHILDREN
)

func NewWidget(parent IWidget, element IElement) *Widget { return coretypes.NewWidget(parent, element) }
