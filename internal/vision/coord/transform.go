package coord

import (
	"trek/internal/engine/core/primitives"
)

// NormalizedToPixel 将归一化 [0,1] 坐标转换为截图像素坐标。
// 结果被钳制到图像边界内。
func NormalizedToPixel(nx, ny float64, shotW, shotH int) (int, int) {
	px := int(nx*float64(shotW) + 0.5)
	py := int(ny*float64(shotH) + 0.5)
	if px < 0 {
		px = 0
	}
	if px >= shotW {
		px = shotW - 1
	}
	if py < 0 {
		py = 0
	}
	if py >= shotH {
		py = shotH - 1
	}
	return px, py
}

// PixelToNormalized 将截图像素坐标转换为归一化 [0,1] 坐标。
func PixelToNormalized(px, py int, shotW, shotH int) (float64, float64) {
	nx := float64(px) / float64(shotW)
	ny := float64(py) / float64(shotH)
	if nx < 0 {
		nx = 0
	}
	if nx > 1 {
		nx = 1
	}
	if ny < 0 {
		ny = 0
	}
	if ny > 1 {
		ny = 1
	}
	return nx, ny
}

// NormalizedRectToPixel 将归一化 [0,1] Rect 转换为截图像素 Rect。
func NormalizedRectToPixel(r *primitives.Rect, shotW, shotH int) *primitives.Rect {
	if r == nil {
		return nil
	}
	left, top := NormalizedToPixel(r.Left, r.Top, shotW, shotH)
	right, bottom := NormalizedToPixel(r.Right, r.Bottom, shotW, shotH)
	return &primitives.Rect{
		Left:   float64(left),
		Top:    float64(top),
		Right:  float64(right),
		Bottom: float64(bottom),
	}
}

// PixelRectToNormalized 将截图像素 Rect 转换为归一化 [0,1] Rect。
func PixelRectToNormalized(r *primitives.Rect, shotW, shotH int) *primitives.Rect {
	if r == nil {
		return nil
	}
	left, top := PixelToNormalized(int(r.Left), int(r.Top), shotW, shotH)
	right, bottom := PixelToNormalized(int(r.Right), int(r.Bottom), shotW, shotH)
	return &primitives.Rect{
		Left:   left,
		Top:    top,
		Right:  right,
		Bottom: bottom,
	}
}

// ScreenshotToDevice 将截图像素空间的 Rect 映射到设备像素空间。
// 当截图被缩小（VLM 优化后）时，DPR 用于反算回设备尺寸。
// 如果截图未缩放（shotSize == deviceSize），操作相当于单位映射。
//
// 注意：这里的变换基于 DPRInfo 中的截图和设备尺寸比率。
// 当截图被 VLM 优化缩小后，需要将坐标从缩小截图空间映射回物理设备空间。
func ScreenshotToDevice(r *primitives.Rect, dpr DPRInfo) *primitives.Rect {
	if r == nil || !dpr.Valid() {
		return r
	}
	scaleX := float64(dpr.DeviceWidth) / float64(dpr.ScreenshotWidth)
	scaleY := float64(dpr.DeviceHeight) / float64(dpr.ScreenshotHeight)
	if scaleX == 0 {
		scaleX = 1
	}
	if scaleY == 0 {
		scaleY = 1
	}
	return &primitives.Rect{
		Left:   r.Left * scaleX,
		Top:    r.Top * scaleY,
		Right:  r.Right * scaleX,
		Bottom: r.Bottom * scaleY,
	}
}

// DeviceToScreenshot 将设备像素空间的 Rect 映射到截图像素空间。
func DeviceToScreenshot(r *primitives.Rect, dpr DPRInfo) *primitives.Rect {
	if r == nil || !dpr.Valid() {
		return r
	}
	scaleX := float64(dpr.ScreenshotWidth) / float64(dpr.DeviceWidth)
	scaleY := float64(dpr.ScreenshotHeight) / float64(dpr.DeviceHeight)
	if scaleX == 0 {
		scaleX = 1
	}
	if scaleY == 0 {
		scaleY = 1
	}
	return &primitives.Rect{
		Left:   r.Left * scaleX,
		Top:    r.Top * scaleY,
		Right:  r.Right * scaleX,
		Bottom: r.Bottom * scaleY,
	}
}

// ClampRect 将 Rect 的所有值钳制到 [0,1] 并确保 left<right, top<bottom。
func ClampRect(r *primitives.Rect) *primitives.Rect {
	if r == nil {
		return nil
	}
	clamped := *r

	if clamped.Left < 0 {
		clamped.Left = 0
	}
	if clamped.Top < 0 {
		clamped.Top = 0
	}
	if clamped.Right > 1 {
		clamped.Right = 1
	}
	if clamped.Bottom > 1 {
		clamped.Bottom = 1
	}

	// 确保几何有效
	if clamped.Left >= clamped.Right {
		// 如果交叉，保留下边界更靠右的值
		if clamped.Left < 1 {
			clamped.Right = clamped.Left + 0.01
		} else {
			clamped.Left = clamped.Right - 0.01
		}
	}
	if clamped.Top >= clamped.Bottom {
		if clamped.Top < 1 {
			clamped.Bottom = clamped.Top + 0.01
		} else {
			clamped.Top = clamped.Bottom - 0.01
		}
	}

	if clamped.Left < 0 {
		clamped.Left = 0
	}
	if clamped.Right > 1 {
		clamped.Right = 1
	}
	if clamped.Top < 0 {
		clamped.Top = 0
	}
	if clamped.Bottom > 1 {
		clamped.Bottom = 1
	}

	return &clamped
}
