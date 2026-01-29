package types

import (
	"fmt"
)

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
	SCROLL_TOP_DOWN
	SCROLL_BOTTOM_UP
	SCROLL_LEFT_RIGHT
	SCROLL_RIGHT_LEFT
	SCROLL_BOTTOM_UP_N
	SHELL_EVENT
	Hover
	ActTypeSize // 18
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

// AlgorithmType 算法类型
type AlgorithmType int

const (
	Random AlgorithmType = 0
	Reuse  AlgorithmType = 4
	Server AlgorithmType = 6
)

// AlgorithmTypeName 算法类型名称映射
var AlgorithmTypeName = map[AlgorithmType]string{
	Random: "Random",
	Reuse:  "reuse",
	Server: "Server",
}

// String 返回算法类型的字符串表示
func (at AlgorithmType) String() string {
	if name, ok := AlgorithmTypeName[at]; ok {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", at)
}

// DeviceType 设备类型
type DeviceType int

const (
	UnknownDevice DeviceType = iota
	Phone
	Tablet
	Computer
)

// Point 点结构
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NewPoint 创建新的点
func NewPoint(x, y float64) *Point {
	return &Point{X: x, Y: y}
}

// Hash 计算点的哈希值
func (p *Point) Hash() uintptr {
	// 将x、y转为uintptr（对应C++的无符号运算，31U、127U，以及返回值uintptr_t）
	ux := uintptr(p.X)
	uy := uintptr(p.Y)

	// 3. 分步实现原C++运算逻辑，保留所有优先级和位运算行为
	// 第一部分：(31U * std::hash<int>{}(x) << 1)
	part1 := (31 * ux) << 1

	// 第二部分：((127U * std::hash<int>{}(y) << 2) >> 1)
	part2 := ((127 * uy) << 2) >> 1

	// 4. 整体异或，返回与原代码完全一致的结果
	return part1 ^ part2
}

// Equal 判断点是否相等
func (p *Point) Equal(other *Point) bool {
	return p.X == other.X && p.Y == other.Y
}

// String 返回点的字符串表示
func (p *Point) String() string {
	return fmt.Sprintf("(%.0f,%.0f)", p.X, p.Y)
}

// Rect 矩形结构
type Rect struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

// NewRect 创建新的矩形
func NewRect(left, top, right, bottom float64) *Rect {
	return &Rect{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}
}

// IsEmpty 判断矩形是否为空
func (r *Rect) IsEmpty() bool {
	return r.Left >= r.Right || r.Top >= r.Bottom
}

// Contains 判断点是否在矩形内
func (r *Rect) Contains(point *Point) bool {
	return point.X >= r.Left && point.X < r.Right &&
		point.Y >= r.Top && point.Y < r.Bottom
}

// Center 返回矩形中心点
func (r *Rect) Center() *Point {
	return &Point{
		X: (r.Left + r.Right) / 2,
		Y: (r.Top + r.Bottom) / 2,
	}
}

// Hash 计算矩形的哈希值
func (r *Rect) Hash() uintptr {

	uTop := uintptr(r.Top)
	uBottom := uintptr(r.Bottom)
	uLeft := uintptr(r.Left)
	uRight := uintptr(r.Right)

	part1 := (31 * uTop << 1) ^ (uBottom << 2)

	part2 := ((uLeft << 1) ^ (127 * uRight << 2)) >> 1

	return part1 ^ part2
}

// Equal 判断矩形是否相等
func (r *Rect) Equal(other *Rect) bool {
	return r.Top == other.Top && r.Bottom == other.Bottom &&
		r.Left == other.Left && r.Right == other.Right
}

// String 返回矩形的字符串表示，保留小数点后3位
func (r *Rect) String() string {
	return fmt.Sprintf("[%.3f,%.3f,%.3f,%.3f]", r.Left, r.Top, r.Right, r.Bottom)
}

// RectZero 零矩形
var RectZero = &Rect{}

// PriorityNode 优先级节点接口
type PriorityNode interface {
	GetPriority() int
}
