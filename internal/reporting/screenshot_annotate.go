package reporting

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"
)

var (
	markColor     = color.RGBA{R: 255, G: 80, B: 0, A: 220}   // 橙红色：圆点/线条
	borderColor   = color.RGBA{R: 255, G: 0, B: 0, A: 180}    // 红色：目标区域边框
	circleRadius  = 12
	crossHalfLen  = 20
	lineThickness = 4
	borderThick   = 3
)

// annotateAction 在截图上绘制动作标注，返回标注后的 PNG 字节。
// 不支持的动作类型直接返回原图。
func annotateAction(screenshot []byte, action string, bounds string) ([]byte, error) {
	if len(screenshot) == 0 {
		return screenshot, nil
	}
	action = strings.TrimSpace(action)
	if !isAnnotatableAction(action) {
		return screenshot, nil
	}

	img, _, err := image.Decode(bytes.NewReader(screenshot))
	if err != nil {
		return screenshot, fmt.Errorf("解码截图失败: %w", err)
	}

	canvas := image.NewRGBA(img.Bounds())
	draw.Draw(canvas, canvas.Bounds(), img, image.Point{}, draw.Src)

	rect, ok := parseNormalizedBounds(bounds)
	if !ok {
		return screenshot, nil
	}

	w := float64(canvas.Bounds().Dx())
	h := float64(canvas.Bounds().Dy())
	var left, top, right, bottom int
	if rect[0] <= 1.0 && rect[1] <= 1.0 && rect[2] <= 1.0 && rect[3] <= 1.0 {
		left = int(rect[0]*w + 0.5)
		top = int(rect[1]*h + 0.5)
		right = int(rect[2]*w + 0.5)
		bottom = int(rect[3]*h + 0.5)
	} else {
		left = int(rect[0] + 0.5)
		top = int(rect[1] + 0.5)
		right = int(rect[2] + 0.5)
		bottom = int(rect[3] + 0.5)
	}

	// 绘制目标区域边框
	drawRectOutline(canvas, image.Rect(left, top, right, bottom), borderColor, borderThick)

	cx := (left + right) / 2
	cy := (top + bottom) / 2

	switch {
	case isScrollAction(action):
		// 滑动：画箭头线
		startRatio, endRatio := swipeRatiosForAction(action)
		sx := int(float64(left)+(float64(right)-float64(left))*startRatio[0] + 0.5)
		sy := int(float64(top)+(float64(bottom)-float64(top))*startRatio[1] + 0.5)
		ex := int(float64(left)+(float64(right)-float64(left))*endRatio[0] + 0.5)
		ey := int(float64(top)+(float64(bottom)-float64(top))*endRatio[1] + 0.5)
		drawArrow(canvas, sx, sy, ex, ey, markColor, lineThickness)
	default:
		// 点击/输入：画圆点 + 十字线
		drawFilledCircle(canvas, cx, cy, circleRadius, markColor)
		drawCross(canvas, cx, cy, crossHalfLen, markColor, lineThickness)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return screenshot, fmt.Errorf("编码标注截图失败: %w", err)
	}
	return buf.Bytes(), nil
}

func isAnnotatableAction(action string) bool {
	switch action {
	case "CLICK", "LONG_CLICK", "INPUT",
		"SCROLL_BOTTOM_UP", "SCROLL_TOP_DOWN",
		"SCROLL_LEFT_RIGHT", "SCROLL_RIGHT_LEFT",
		"SCROLL_BOTTOM_UP_N":
		return true
	}
	return false
}

func isScrollAction(action string) bool {
	switch action {
	case "SCROLL_BOTTOM_UP", "SCROLL_TOP_DOWN",
		"SCROLL_LEFT_RIGHT", "SCROLL_RIGHT_LEFT",
		"SCROLL_BOTTOM_UP_N":
		return true
	}
	return false
}

// swipeRatiosForAction 返回滑动起点和终点在目标区域内的比例。
func swipeRatiosForAction(action string) (start, end [2]float64) {
	switch action {
	case "SCROLL_BOTTOM_UP", "SCROLL_BOTTOM_UP_N":
		return [2]float64{0.5, 0.82}, [2]float64{0.5, 0.22}
	case "SCROLL_TOP_DOWN":
		return [2]float64{0.5, 0.22}, [2]float64{0.5, 0.82}
	case "SCROLL_LEFT_RIGHT":
		return [2]float64{0.22, 0.5}, [2]float64{0.82, 0.5}
	case "SCROLL_RIGHT_LEFT":
		return [2]float64{0.82, 0.5}, [2]float64{0.22, 0.5}
	default:
		return [2]float64{0.5, 0.5}, [2]float64{0.5, 0.5}
	}
}

// parseNormalizedBounds 解析 [L,T,B,R] 格式的归一化坐标。
func parseNormalizedBounds(s string) ([4]float64, bool) {
	var result [4]float64
	text := strings.TrimSpace(s)
	if text == "" {
		return result, false
	}
	// 去掉方括号
	text = strings.TrimPrefix(text, "[")
	text = strings.TrimSuffix(text, "]")
	parts := strings.Split(text, ",")
	if len(parts) != 4 {
		// 尝试 ][ 格式: [L,T][R,B]
		text = strings.TrimSpace(s)
		text = strings.TrimPrefix(text, "[")
		text = strings.TrimSuffix(text, "]")
		segments := strings.Split(text, "][")
		if len(segments) == 2 {
			lt := strings.Split(segments[0], ",")
			rb := strings.Split(segments[1], ",")
			if len(lt) == 2 && len(rb) == 2 {
				var err error
				if result[0], err = strconv.ParseFloat(strings.TrimSpace(lt[0]), 64); err != nil {
					return result, false
				}
				if result[1], err = strconv.ParseFloat(strings.TrimSpace(lt[1]), 64); err != nil {
					return result, false
				}
				if result[2], err = strconv.ParseFloat(strings.TrimSpace(rb[0]), 64); err != nil {
					return result, false
				}
				if result[3], err = strconv.ParseFloat(strings.TrimSpace(rb[1]), 64); err != nil {
					return result, false
				}
				return result, true
			}
		}
		return result, false
	}
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return result, false
		}
		result[i] = v
	}
	return result, true
}

// drawFilledCircle 在画布上绘制填充圆。
func drawFilledCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	bounds := img.Bounds()
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			if x*x+y*y <= r*r {
				px, py := cx+x, cy+y
				if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
					img.SetRGBA(px, py, c)
				}
			}
		}
	}
}

// drawCross 在画布上绘制十字线。
func drawCross(img *image.RGBA, cx, cy, halfLen int, c color.RGBA, thickness int) {
	drawThickLine(img, cx-halfLen, cy, cx+halfLen, cy, c, thickness)
	drawThickLine(img, cx, cy-halfLen, cx, cy+halfLen, c, thickness)
}

// drawArrow 在画布上绘制带箭头的线。
func drawArrow(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	drawThickLine(img, x0, y0, x1, y1, c, thickness)

	// 箭头
	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 1 {
		return
	}
	// 归一化方向
	ux, uy := dx/length, dy/length
	// 箭头长度
	arrowLen := 20.0
	arrowWidth := 10.0
	// 箭头两翼
	ax := float64(x1) - arrowLen*ux + arrowWidth*uy
	ay := float64(y1) - arrowLen*uy - arrowWidth*ux
	bx := float64(x1) - arrowLen*ux - arrowWidth*uy
	by := float64(y1) - arrowLen*uy + arrowWidth*ux
	drawThickLine(img, x1, y1, int(ax), int(ay), c, thickness)
	drawThickLine(img, x1, y1, int(bx), int(by), c, thickness)
}

// drawThickLine 使用 Bresenham 算法画粗线。
func drawThickLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	// 使用简单的逐像素方式画线，对每个像素画 thickness 大小的方块
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := 1
	sy := 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	x, y := x0, y0
	bounds := img.Bounds()
	half := thickness / 2

	for {
		// 画粗线：以当前点为中心画方块
		for dy2 := -half; dy2 <= half; dy2++ {
			for dx2 := -half; dx2 <= half; dx2++ {
				px, py := x+dx2, y+dy2
				if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
					img.SetRGBA(px, py, c)
				}
			}
		}
		if x == x1 && y == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// drawRectOutline 在画布上绘制矩形边框。
func drawRectOutline(img *image.RGBA, rect image.Rectangle, c color.RGBA, thickness int) {
	drawThickLine(img, rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y, c, thickness) // 上
	drawThickLine(img, rect.Min.X, rect.Max.Y, rect.Max.X, rect.Max.Y, c, thickness) // 下
	drawThickLine(img, rect.Min.X, rect.Min.Y, rect.Min.X, rect.Max.Y, c, thickness) // 左
	drawThickLine(img, rect.Max.X, rect.Min.Y, rect.Max.X, rect.Max.Y, c, thickness) // 右
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
