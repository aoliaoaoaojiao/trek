package monkey

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// writeStepSnapshotArtifacts 将单步截图和 XML 实时写入磁盘产物目录。
func writeStepSnapshotArtifacts(rootDir string, record StepRecord, phase string, pageName string, xmlText string, screenshot []byte) (*StepArtifactRef, error) {
	pageDirName := sanitizePageDirName(pageName)
	if strings.TrimSpace(pageDirName) == "" {
		pageDirName = "UnknownPage"
	}
	pageDirPath := filepath.Join(rootDir, pageDirName)
	ref := &StepArtifactRef{PageDir: pageDirName}

	needWrite := len(screenshot) > 0 || strings.TrimSpace(xmlText) != ""
	if !needWrite {
		return nil, nil
	}
	if err := os.MkdirAll(pageDirPath, 0755); err != nil {
		return nil, fmt.Errorf("创建页面产物目录失败(%s): %w", pageDirName, err)
	}

	prefix := buildArtifactFilePrefix(record, phase)
	if len(screenshot) > 0 {
		ext := detectImageExt(screenshot)
		fileName := prefix + ext
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), screenshot, 0644); err != nil {
			return nil, fmt.Errorf("写入截图产物失败(%s): %w", fileName, err)
		}
		ref.ScreenshotFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
	}
	if strings.TrimSpace(xmlText) != "" {
		fileName := prefix + ".xml"
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), []byte(xmlText), 0644); err != nil {
			return nil, fmt.Errorf("写入 XML 产物失败(%s): %w", fileName, err)
		}
		ref.XMLFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
	}
	if ref.ScreenshotFile == "" && ref.XMLFile == "" {
		return nil, nil
	}
	return ref, nil
}

func buildArtifactFilePrefix(record StepRecord, phase string) string {
	var b strings.Builder
	b.WriteString("step-")
	b.WriteString(strconv.FormatInt(int64(record.Step), 10))
	b.WriteString("-")
	b.WriteString(phase)
	if action := strings.TrimSpace(record.Action); action != "" {
		b.WriteString("-")
		b.WriteString(sanitizePageDirName(action))
	}
	return b.String()
}

func sanitizePageDirName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteRune('_')
		case r == ' ' || r == '\t':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "._")
}

func detectImageExt(data []byte) string {
	if len(data) == 0 {
		return ".png"
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "jpeg":
			return ".jpg"
		case "png":
			return ".png"
		}
	}
	return ".png"
}

// saveOriginalIfNew 首次遇到该页面目录时保存原始截图。
func saveOriginalIfNew(pageDirPath string, screenshot []byte) {
	if len(screenshot) == 0 {
		return
	}
	// 用文件是否存在来判断，避免引入额外状态
	origPath := filepath.Join(pageDirPath, "original.png")
	if _, err := os.Stat(origPath); err == nil {
		return // 已存在
	}
	_ = os.WriteFile(origPath, screenshot, 0644)
}

// annotateAndSaveMarked 生成标注截图并保存。
func annotateAndSaveMarked(pageDirPath string, prefix string, screenshot []byte, action string, bounds string, swipeStart, swipeEnd string) {
	if len(screenshot) == 0 {
		return
	}
	action = strings.TrimSpace(action)
	if !isAnnotatableAction(action) {
		return
	}
	marked, err := annotateScreenshot(screenshot, action, bounds, swipeStart, swipeEnd)
	if err != nil || len(marked) == 0 {
		return
	}
	markedPath := filepath.Join(pageDirPath, prefix+"-marked.png")
	_ = os.WriteFile(markedPath, marked, 0644)
}

// --- 截图标注绘制 ---

var (
	markColor     = color.RGBA{R: 255, G: 80, B: 0, A: 220}
	borderColor   = color.RGBA{R: 255, G: 0, B: 0, A: 180}
	circleRadius  = 12
	crossHalfLen  = 20
	lineThickness = 4
	borderThick   = 3
)

func annotateScreenshot(screenshot []byte, action string, bounds string, swipeStart, swipeEnd string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(screenshot))
	if err != nil {
		return nil, fmt.Errorf("解码截图失败: %w", err)
	}

	canvas := image.NewRGBA(img.Bounds())
	draw.Draw(canvas, canvas.Bounds(), img, image.Point{}, draw.Src)

	rect, ok := parseNormalizedBounds(bounds)
	if !ok {
		return nil, fmt.Errorf("解析 bounds 失败: %s", bounds)
	}

	w := float64(canvas.Bounds().Dx())
	h := float64(canvas.Bounds().Dy())
	var left, top, right, bottom int
	// 坐标可能是归一化（0-1）或像素值，自动判断
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

	// bounds 无效（BACK/SCROLL 等无目标区域的操作）：用屏幕中心作为参考点，青色边框
	if left >= right || top >= bottom {
		left = int(w*0.2 + 0.5)
		top = int(h*0.2 + 0.5)
		right = int(w*0.8 + 0.5)
		bottom = int(h*0.8 + 0.5)
		rect = [4]float64{0.2, 0.2, 0.8, 0.8}
		borderColor = color.RGBA{0, 200, 255, 255}
	}

	cx := (left + right) / 2
	cy := (top + bottom) / 2

	switch {
	case isScrollAction(action):
		var sx, sy, ex, ey int
		// 优先用实际 swipe 坐标（完整轨迹），否则用 bounds 内比例
		if start, end, err := parseSwipePoints(swipeStart, swipeEnd); err == nil {
			sx, sy, ex, ey = int(start[0]), int(start[1]), int(end[0]), int(end[1])
		} else {
			startRatio, endRatio := swipeRatiosForAction(action)
			sx = int((rect[0]+(rect[2]-rect[0])*startRatio[0])*w + 0.5)
			sy = int((rect[1]+(rect[3]-rect[1])*startRatio[1])*h + 0.5)
			ex = int((rect[0]+(rect[2]-rect[0])*endRatio[0])*w + 0.5)
			ey = int((rect[1]+(rect[3]-rect[1])*endRatio[1])*h + 0.5)
		}
		drawArrow(canvas, sx, sy, ex, ey, markColor, lineThickness)
	default:
		drawRectOutline(canvas, image.Rect(left, top, right, bottom), borderColor, borderThick)
		drawFilledCircle(canvas, cx, cy, circleRadius, markColor)
		drawCross(canvas, cx, cy, crossHalfLen, markColor, lineThickness)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, fmt.Errorf("编码标注截图失败: %w", err)
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

// parseSwipePoints 解析 "[x,y]" 格式的滑动起终点坐标。
func parseSwipePoints(startStr, endStr string) ([2]float64, [2]float64, error) {
	parse := func(s string) ([2]float64, error) {
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		parts := strings.Split(s, ",")
		if len(parts) != 2 {
			return [2]float64{}, fmt.Errorf("invalid swipe point: %s", s)
		}
		x, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		y, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil {
			return [2]float64{}, fmt.Errorf("parse swipe point failed: %s", s)
		}
		return [2]float64{x, y}, nil
	}
	start, err1 := parse(startStr)
	end, err2 := parse(endStr)
	if err1 != nil || err2 != nil {
		return [2]float64{}, [2]float64{}, fmt.Errorf("parse swipe points failed")
	}
	return start, end, nil
}

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

func parseNormalizedBounds(s string) ([4]float64, bool) {
	var result [4]float64
	text := strings.TrimSpace(s)
	if text == "" {
		return result, false
	}
	text = strings.TrimPrefix(text, "[")
	text = strings.TrimSuffix(text, "]")
	parts := strings.Split(text, ",")
	if len(parts) != 4 {
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

func drawCross(img *image.RGBA, cx, cy, halfLen int, c color.RGBA, thickness int) {
	drawThickLine(img, cx-halfLen, cy, cx+halfLen, cy, c, thickness)
	drawThickLine(img, cx, cy-halfLen, cx, cy+halfLen, c, thickness)
}

func drawArrow(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	// 粗线
	drawThickLine(img, x0, y0, x1, y1, c, thickness)

	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 1 {
		return
	}
	ux, uy := dx/length, dy/length

	// 填充三角箭头
	arrowLen := 30.0
	arrowWidth := 16.0
	ax := float64(x1) - arrowLen*ux + arrowWidth*uy
	ay := float64(y1) - arrowLen*uy - arrowWidth*ux
	bx := float64(x1) - arrowLen*ux - arrowWidth*uy
	by := float64(y1) - arrowLen*uy + arrowWidth*ux

	pts := [][2]float64{{float64(x1), float64(y1)}, {ax, ay}, {bx, by}}
	fillTriangle(img, pts, c)
}

// fillTriangle 用扫描线算法填充三角形。
func fillTriangle(img *image.RGBA, pts [][2]float64, c color.RGBA) {
	minY, maxY := int(pts[0][1]), int(pts[0][1])
	for _, p := range pts {
		y := int(p[1])
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	bounds := img.Bounds()
	for y := minY; y <= maxY; y++ {
		xs := []int{}
		n := len(pts)
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			yi, yj := pts[i][1], pts[j][1]
			if (yi <= float64(y) && yj > float64(y)) || (yj <= float64(y) && yi > float64(y)) {
				t := (float64(y) - yi) / (yj - yi)
				x := int(pts[i][0] + t*(pts[j][0]-pts[i][0]))
				xs = append(xs, x)
			}
		}
		if len(xs) >= 2 {
			if xs[0] > xs[1] {
				xs[0], xs[1] = xs[1], xs[0]
			}
			for x := xs[0]; x <= xs[1]; x++ {
				if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
					img.SetRGBA(x, y, c)
				}
			}
		}
	}
}

func drawRectOutline(img *image.RGBA, rect image.Rectangle, c color.RGBA, thickness int) {
	drawThickLine(img, rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y, c, thickness)
	drawThickLine(img, rect.Min.X, rect.Max.Y, rect.Max.X, rect.Max.Y, c, thickness)
	drawThickLine(img, rect.Min.X, rect.Min.Y, rect.Min.X, rect.Max.Y, c, thickness)
	drawThickLine(img, rect.Max.X, rect.Min.Y, rect.Max.X, rect.Max.Y, c, thickness)
}

func drawThickLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA, thickness int) {
	dx := absInt(x1 - x0)
	dy := absInt(y1 - y0)
	sx, sy := 1, 1
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

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
