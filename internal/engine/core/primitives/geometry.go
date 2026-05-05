package primitives

import "fmt"

// Point 表示二维坐标点。
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NewPoint 创建新的点。
func NewPoint(x, y float64) *Point {
	return &Point{X: x, Y: y}
}

// Hash 计算点的哈希值。
func (p *Point) Hash() uintptr {
	ux := uintptr(p.X)
	uy := uintptr(p.Y)
	part1 := (31 * ux) << 1
	part2 := ((127 * uy) << 2) >> 1
	return part1 ^ part2
}

// Equal 判断点是否相等。
func (p *Point) Equal(other *Point) bool {
	return p.X == other.X && p.Y == other.Y
}

// String 返回点的字符串表示。
func (p *Point) String() string {
	return fmt.Sprintf("(%.0f,%.0f)", p.X, p.Y)
}

// Rect 表示矩形区域。
type Rect struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

// NewRect 创建新的矩形。
func NewRect(left, top, right, bottom float64) *Rect {
	return &Rect{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}
}

// IsEmpty 判断矩形是否为空。
func (r *Rect) IsEmpty() bool {
	return r.Left >= r.Right || r.Top >= r.Bottom
}

// Contains 判断点是否在矩形内。
func (r *Rect) Contains(point *Point) bool {
	return point.X >= r.Left && point.X < r.Right &&
		point.Y >= r.Top && point.Y < r.Bottom
}

// Center 返回矩形中心点。
func (r *Rect) Center() *Point {
	return &Point{
		X: (r.Left + r.Right) / 2,
		Y: (r.Top + r.Bottom) / 2,
	}
}

// Hash 计算矩形的哈希值。
func (r *Rect) Hash() uintptr {
	uTop := uintptr(r.Top)
	uBottom := uintptr(r.Bottom)
	uLeft := uintptr(r.Left)
	uRight := uintptr(r.Right)
	part1 := (31 * uTop << 1) ^ (uBottom << 2)
	part2 := ((uLeft << 1) ^ (127 * uRight << 2)) >> 1
	return part1 ^ part2
}

// Equal 判断矩形是否相等。
func (r *Rect) Equal(other *Rect) bool {
	return r.Top == other.Top && r.Bottom == other.Bottom &&
		r.Left == other.Left && r.Right == other.Right
}

// String 返回矩形的字符串表示，保留小数点后 3 位。
func (r *Rect) String() string {
	return fmt.Sprintf("[%.3f,%.3f,%.3f,%.3f]", r.Left, r.Top, r.Right, r.Bottom)
}

// RectZero 表示零矩形。
var RectZero = &Rect{}
