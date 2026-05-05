package types

import "trek/internal/engine/core/primitives"

// ActionType 定义动作类型
type ActionType int

const (
	CRASH ActionType = iota
	FUZZ
	START
	RESTART
	CLEAN_RESTART
	NOP
	ACTIVATE
	BACK
	FEED
	CLICK
	LONG_CLICK
	INPUT
	SCROLL_TOP_DOWN
	SCROLL_BOTTOM_UP
	SCROLL_LEFT_RIGHT
	SCROLL_RIGHT_LEFT
	SCROLL_BOTTOM_UP_N
	SHELL_EVENT
	Hover
	ActTypeSize // 19
)

func (t ActionType) String() string {
	return actName[t]
}

// actName 动作类型名称映射
var actName = map[ActionType]string{
	CRASH:              "CRASH",
	FUZZ:               "FUZZ",
	START:              "START",
	RESTART:            "RESTART",
	CLEAN_RESTART:      "CLEAN_RESTART",
	NOP:                "NOP",
	ACTIVATE:           "ACTIVATE",
	BACK:               "BACK",
	FEED:               "FEED",
	CLICK:              "CLICK",
	LONG_CLICK:         "LONG_CLICK",
	INPUT:              "INPUT",
	SCROLL_TOP_DOWN:    "SCROLL_TOP_DOWN",
	SCROLL_BOTTOM_UP:   "SCROLL_BOTTOM_UP",
	SCROLL_LEFT_RIGHT:  "SCROLL_LEFT_RIGHT",
	SCROLL_RIGHT_LEFT:  "SCROLL_RIGHT_LEFT",
	SCROLL_BOTTOM_UP_N: "SCROLL_BOTTOM_UP_N",
	SHELL_EVENT:        "SHELL_EVENT",
	Hover:              "Hover",
}

// ScrollType 滚动类型
type ScrollType int

const (
	ALL ScrollType = iota
	Horizontal
	Vertical
	NONE
	VerticalSeries
	ScrollTypeSize // 5
)

// scrollTypeName 滚动类型名称映射
var scrollTypeName = map[ScrollType]string{
	ALL:            "ALL",
	Horizontal:     "Horizontal",
	Vertical:       "Vertical",
	NONE:           "NONE",
	VerticalSeries: "VerticalSeries",
}

// StringToScrollType 字符串转滚动类型
func StringToScrollType(str string) ScrollType {
	for st, name := range scrollTypeName {
		if name == str {
			return st
		}
	}
	return NONE
}

// TransitionVisitType 转换访问类型
type TransitionVisitType int

const (
	NEW_ACTION TransitionVisitType = iota
	NEW_ACTION_TARGET
	EXISTING
)

// OperateType 操作类型
type OperateType int

const (
	None          OperateType = 0
	Enable                    = 0x0001
	Clickable                 = Enable << 1
	Checkable                 = Enable << 2
	LongClickable             = Enable << 3
	Scrollable                = Enable << 4
	Inputable                 = Enable << 5
)

// DeviceType 设备类型
type DeviceType int

const (
	UnknownDevice DeviceType = iota
	Phone
	Tablet
	Computer
)

// Point 点结构。
type Point = primitives.Point

// Rect 矩形结构。
type Rect = primitives.Rect

// NewPoint 创建新的点。
func NewPoint(x, y float64) *Point {
	return primitives.NewPoint(x, y)
}

// NewRect 创建新的矩形。
func NewRect(left, top, right, bottom float64) *Rect {
	return primitives.NewRect(left, top, right, bottom)
}

// RectZero 零矩形。
var RectZero = primitives.RectZero

// PriorityNode 优先级节点接口
type PriorityNode interface {
	GetPriority() int
}

// ScrollInferThreshold 推断可滚动元素的最小可点击后代数量阈值。
// 设为 0 禁用推断。默认值 5。
var ScrollInferThreshold = 5
