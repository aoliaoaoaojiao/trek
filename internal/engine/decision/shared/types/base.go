package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出，实际实现已迁移至 core/types。

type ActionType = coretypes.ActionType
type ScrollType = coretypes.ScrollType
type TransitionVisitType = coretypes.TransitionVisitType
type OperateType = coretypes.OperateType
type DeviceType = coretypes.DeviceType
type Point = coretypes.Point
type Rect = coretypes.Rect
type PriorityNode = coretypes.PriorityNode

const (
	CRASH              = coretypes.CRASH
	FUZZ               = coretypes.FUZZ
	START              = coretypes.START
	RESTART            = coretypes.RESTART
	CLEAN_RESTART      = coretypes.CLEAN_RESTART
	NOP                = coretypes.NOP
	ACTIVATE           = coretypes.ACTIVATE
	BACK               = coretypes.BACK
	FEED               = coretypes.FEED
	CLICK              = coretypes.CLICK
	LONG_CLICK         = coretypes.LONG_CLICK
	INPUT              = coretypes.INPUT
	SCROLL_TOP_DOWN    = coretypes.SCROLL_TOP_DOWN
	SCROLL_BOTTOM_UP   = coretypes.SCROLL_BOTTOM_UP
	SCROLL_LEFT_RIGHT  = coretypes.SCROLL_LEFT_RIGHT
	SCROLL_RIGHT_LEFT  = coretypes.SCROLL_RIGHT_LEFT
	SCROLL_BOTTOM_UP_N = coretypes.SCROLL_BOTTOM_UP_N
	SHELL_EVENT        = coretypes.SHELL_EVENT
	Hover              = coretypes.Hover
	ActTypeSize        = coretypes.ActTypeSize

	ALL            = coretypes.ALL
	Horizontal     = coretypes.Horizontal
	Vertical       = coretypes.Vertical
	NONE           = coretypes.NONE
	VerticalSeries = coretypes.VerticalSeries
	ScrollTypeSize = coretypes.ScrollTypeSize

	NEW_ACTION        = coretypes.NEW_ACTION
	NEW_ACTION_TARGET = coretypes.NEW_ACTION_TARGET
	EXISTING          = coretypes.EXISTING

	None          = coretypes.None
	Enable        = coretypes.Enable
	Clickable     = coretypes.Clickable
	Checkable     = coretypes.Checkable
	LongClickable = coretypes.LongClickable
	Scrollable    = coretypes.Scrollable
	Inputable     = coretypes.Inputable

	UnknownDevice = coretypes.UnknownDevice
	Phone         = coretypes.Phone
	Tablet        = coretypes.Tablet
	Computer      = coretypes.Computer
)

var RectZero = coretypes.RectZero

func NewPoint(x, y float64) *Point { return coretypes.NewPoint(x, y) }
func NewRect(left, top, right, bottom float64) *Rect {
	return coretypes.NewRect(left, top, right, bottom)
}
func StringToScrollType(str string) ScrollType { return coretypes.StringToScrollType(str) }
