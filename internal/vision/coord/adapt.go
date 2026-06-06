package coord

import (
	"trek/internal/engine/core/primitives"
)

// BboxFormat 标识 VLM 输出的坐标格式类型。
type BboxFormat int

const (
	// BboxNormalized 表示 [0,1] 归一化坐标。
	BboxNormalized BboxFormat = iota
	// BboxZeroTo1000 表示 [0,1000] 范围的坐标（GPT-4o 等常用）。
	BboxZeroTo1000
	// BboxPixels 表示原始像素坐标。
	BboxPixels
)

// DetectBboxFormat 自动检测 VLM 输出边界的坐标格式。
// 规则：
//   - 所有值在 [0,1] 范围内 → 归一化格式
//   - 所有值在 [0,1000] 范围内且有值 > 1 → 0-1000 格式
//   - 其他 → 像素格式
func DetectBboxFormat(left, top, right, bottom float64) BboxFormat {
	if left >= 0 && left <= 1 &&
		top >= 0 && top <= 1 &&
		right >= 0 && right <= 1 &&
		bottom >= 0 && bottom <= 1 {
		return BboxNormalized
	}

	if left >= 0 && left <= 1000 &&
		top >= 0 && top <= 1000 &&
		right >= 0 && right <= 1000 &&
		bottom >= 0 && bottom <= 1000 {
		if left > 1 || top > 1 || right > 1 || bottom > 1 {
			return BboxZeroTo1000
		}
	}

	return BboxPixels
}

// AdaptBboxToRect 将 VLM 输出的边界框（任意格式）转换为归一化 [0,1] Rect。
// shotW/shotH 是发给 VLM 的截图尺寸（用于像素格式的归一化）。
// 返回的 Rect 被钳制到 [0,1] 范围并确保几何有效性。
func AdaptBboxToRect(left, top, right, bottom float64, shotW, shotH int) *primitives.Rect {
	format := DetectBboxFormat(left, top, right, bottom)

	var nl, nt, nr, nb float64

	switch format {
	case BboxNormalized:
		nl, nt, nr, nb = left, top, right, bottom

	case BboxZeroTo1000:
		nl = left / 1000.0
		nt = top / 1000.0
		nr = right / 1000.0
		nb = bottom / 1000.0

	case BboxPixels:
		if shotW > 0 && shotH > 0 {
			nl = left / float64(shotW)
			nt = top / float64(shotH)
			nr = right / float64(shotW)
			nb = bottom / float64(shotH)
		} else {
			nl, nt, nr, nb = left, top, right, bottom
		}
	}

	rect := &primitives.Rect{
		Left:   nl,
		Top:    nt,
		Right:  nr,
		Bottom: nb,
	}
	return ClampRect(rect)
}

// AdaptBboxToDeviceRect 将 VLM 输出直接转换为设备像素 Rect。
// 组合 AdaptBboxToRect + ScreenshotToDevice。
func AdaptBboxToDeviceRect(left, top, right, bottom float64, shotW, shotH int, dpr DPRInfo) *primitives.Rect {
	rect := AdaptBboxToRect(left, top, right, bottom, shotW, shotH)
	return ScreenshotToDevice(rect, dpr)
}
