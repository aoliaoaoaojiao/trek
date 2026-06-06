package annotation

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// 标签颜色调色板（5 种高对比度颜色，循环使用）。
// 参考 Midscene 的红/蓝/棕/绿/紫设计。
var labelColors = []color.RGBA{
	{R: 220, G: 50, B: 50, A: 220},   // 红
	{R: 50, G: 100, B: 220, A: 220},   // 蓝
	{R: 180, G: 120, B: 50, A: 220},   // 棕
	{R: 50, G: 180, B: 80, A: 220},    // 绿
	{R: 160, G: 60, B: 180, A: 220},   // 紫
}

var labelBgColors = []color.RGBA{
	{R: 220, G: 50, B: 50, A: 180},    // 红（半透明背景）
	{R: 50, G: 100, B: 220, A: 180},   // 蓝（半透明背景）
	{R: 180, G: 120, B: 50, A: 180},   // 棕（半透明背景）
	{R: 50, G: 180, B: 80, A: 180},    // 绿（半透明背景）
	{R: 160, G: 60, B: 180, A: 180},   // 紫（半透明背景）
}

// LabelColorPalette 提供带编号元素标签的颜色循环分配。
type LabelColorPalette struct {
	index int
}

// NewLabelColorPalette 创建新的颜色调色板。
func NewLabelColorPalette() *LabelColorPalette {
	return &LabelColorPalette{}
}

// Next 返回下一个标签颜色（前景）和背景色。
func (p *LabelColorPalette) Next() (fg, bg color.RGBA) {
	c := labelColors[p.index%len(labelColors)]
	bc := labelBgColors[p.index%len(labelBgColors)]
	p.index++
	return c, bc
}

// Reset 重置颜色索引。
func (p *LabelColorPalette) Reset() {
	p.index = 0
}

// DrawLabeledBoxes 在图像上绘制带编号的边界框标注。
// 参数：
//   - img: 源图像（将在其副本上绘制）
//   - boxes: 元素边界矩形列表（截图像素坐标）
//   - font: 5x7 位图字体
//   - palette: 颜色调色板
//   - scale: 字体缩放因子（推荐 2）
//
// 返回绘制了标注的 RGBA 图像。
// 标签位置通过碰撞规避算法选择最优位置。
func DrawLabeledBoxes(img *image.RGBA, boxes []image.Rectangle, font *BitmapFont5x7, palette *LabelColorPalette, scale int) *image.RGBA {
	if scale <= 0 {
		scale = 2
	}

	// 创建输出画布
	bounds := img.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, img, image.Point{}, draw.Src)

	// 跟踪已放置的标签位置（用于碰撞检测）
	var placedLabels []image.Rectangle

	for i, box := range boxes {
		fg, bg := palette.Next()
		num := i + 1 // 编号从 1 开始

		// 绘制边界框
		drawBoxOutline(canvas, box, fg, 2)

		// 计算标签尺寸
		labelW := font.TextWidth(num, scale)
		labelH := font.TextHeight(scale)
		pad := 2 // 内边距

		// 候选标签位置（按优先级排序）
		candidates := []struct {
			x, y int // 标签左上角
		}{
			// 1. 框外左上
			{box.Min.X, box.Min.Y - labelH - pad*2 - 2},
			// 2. 框外右上
			{box.Max.X - labelW - pad*2, box.Min.Y - labelH - pad*2 - 2},
			// 3. 框外左下
			{box.Min.X, box.Max.Y + 2},
			// 4. 框外右下
			{box.Max.X - labelW - pad*2, box.Max.Y + 2},
			// 5. 框内顶部
			{box.Min.X + 2, box.Min.Y + 2},
		}

		// 选择第一个无碰撞的位置
		chosen := candidates[len(candidates)-1] // 默认最后一候选
		for _, c := range candidates {
			labelRect := image.Rect(c.x, c.y, c.x+labelW+pad*2, c.y+labelH+pad*2)
			// 检查是否在图像边界内
			if labelRect.Min.X < bounds.Min.X || labelRect.Max.X > bounds.Max.X ||
				labelRect.Min.Y < bounds.Min.Y || labelRect.Max.Y > bounds.Max.Y {
				continue
			}
			// 检查是否与已放置标签碰撞
			if hasOverlap(labelRect, placedLabels) {
				continue
			}
			chosen = c
			break
		}

		// 绘制标签背景
		labelRect := image.Rect(chosen.x, chosen.y, chosen.x+labelW+pad*2, chosen.y+labelH+pad*2)
		drawFilledRect(canvas, labelRect, bg)

		// 绘制数字
		textX := chosen.x + pad
		textY := chosen.y + pad
		// 绘制数字阴影增加可读性
		font.DrawNumber(canvas, num, textX+1, textY+1, scale, color.RGBA{0, 0, 0, 160})
		// 绘制数字主体
		font.DrawNumber(canvas, num, textX, textY, scale, color.White)

		// 记录已放置标签
		placedLabels = append(placedLabels, labelRect)
	}

	return canvas
}

// DrawLabeledBoxesFromBytes 是 DrawLabeledBoxes 的便捷封装。
// 解码 PNG/JPEG → 调用 DrawLabeledBoxes → 重新编码为 PNG。
func DrawLabeledBoxesFromBytes(data []byte, boxes []image.Rectangle) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, err
	}

	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	font := DefaultFont5x7()
	palette := NewLabelColorPalette()
	result := DrawLabeledBoxes(rgba, boxes, font, palette, 2)

	var buf bytes.Buffer
	if err := png.Encode(&buf, result); err != nil {
		return data, err
	}
	return buf.Bytes(), nil
}

// drawBoxOutline 在画布上绘制矩形边框。
func drawBoxOutline(img *image.RGBA, rect image.Rectangle, c color.RGBA, thickness int) {
	bounds := img.Bounds()
	for t := 0; t < thickness; t++ {
		// 上
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && rect.Min.Y+t >= bounds.Min.Y && rect.Min.Y+t < bounds.Max.Y {
				img.SetRGBA(x, rect.Min.Y+t, c)
			}
		}
		// 下
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && rect.Max.Y-1-t >= bounds.Min.Y && rect.Max.Y-1-t < bounds.Max.Y {
				img.SetRGBA(x, rect.Max.Y-1-t, c)
			}
		}
		// 左
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			if rect.Min.X+t >= bounds.Min.X && rect.Min.X+t < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(rect.Min.X+t, y, c)
			}
		}
		// 右
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			if rect.Max.X-1-t >= bounds.Min.X && rect.Max.X-1-t < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				img.SetRGBA(rect.Max.X-1-t, y, c)
			}
		}
	}
}

// drawFilledRect 在画布上绘制填充矩形。
func drawFilledRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	bounds := img.Bounds()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
				// alpha 混合
				existing := img.RGBAAt(x, y)
				alpha := float64(c.A) / 255.0
				img.SetRGBA(x, y, color.RGBA{
					R: uint8(float64(c.R)*alpha + float64(existing.R)*(1-alpha)),
					G: uint8(float64(c.G)*alpha + float64(existing.G)*(1-alpha)),
					B: uint8(float64(c.B)*alpha + float64(existing.B)*(1-alpha)),
					A: 255,
				})
			}
		}
	}
}

// hasOverlap 检查候选矩形是否与已有矩形列表中的任何一个重叠。
func hasOverlap(candidate image.Rectangle, placed []image.Rectangle) bool {
	for _, p := range placed {
		if candidate.Overlaps(p) {
			return true
		}
	}
	return false
}
