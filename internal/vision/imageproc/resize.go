package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

// VLMConfig 定义 VLM 模型特定的图像优化参数。
type VLMConfig struct {
	MaxWidth   int // 最大宽度（默认 1280）
	MaxHeight  int // 最大高度（0 = 不限，GPT-4o: 768）
	Quality    int // JPEG 质量（默认 90）
	BlockAlign int // 像素对齐需求（Qwen2.5-VL: 28，0 = 不要求）
	Format     ImageFormat
}

// DefaultVLMConfig 返回通用默认配置（1280px 宽度，JPEG Q90）。
func DefaultVLMConfig() VLMConfig {
	return VLMConfig{
		MaxWidth:   1280,
		MaxHeight:  0,
		Quality:    90,
		BlockAlign: 0,
		Format:     FormatJPEG,
	}
}

// ConfigGPT4o 返回 GPT-4o 优化配置（2048x768，JPEG Q90）。
func ConfigGPT4o() VLMConfig {
	return VLMConfig{
		MaxWidth:   2048,
		MaxHeight:  768,
		Quality:    90,
		BlockAlign: 0,
		Format:     FormatJPEG,
	}
}

// ConfigQwenVL 返回 Qwen2.5-VL 优化配置（1280px，28px block align）。
func ConfigQwenVL() VLMConfig {
	return VLMConfig{
		MaxWidth:   1280,
		MaxHeight:  0,
		Quality:    90,
		BlockAlign: 28,
		Format:     FormatJPEG,
	}
}

// ResizeWithInterpolation 使用双线性插值调整图像尺寸。
func ResizeWithInterpolation(img image.Image, newW, newH int) *image.RGBA {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 || newW <= 0 || newH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, newW, newH))
	}

	scaleX := float64(srcW) / float64(newW)
	scaleY := float64(srcH) / float64(newH)

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))

	for dy := 0; dy < newH; dy++ {
		for dx := 0; dx < newW; dx++ {
			srcX := float64(dx)*scaleX + float64(bounds.Min.X)
			srcY := float64(dy)*scaleY + float64(bounds.Min.Y)

			x0 := int(srcX)
			y0 := int(srcY)
			x1 := x0 + 1
			y1 := y0 + 1

			maxX := bounds.Max.X - 1
			maxY := bounds.Max.Y - 1
			if x1 > maxX {
				x1 = maxX
			}
			if y1 > maxY {
				y1 = maxY
			}

			fracX := srcX - float64(x0)
			fracY := srcY - float64(y0)

			c00 := rgbaAt(img, x0, y0)
			c10 := rgbaAt(img, x1, y0)
			c01 := rgbaAt(img, x0, y1)
			c11 := rgbaAt(img, x1, y1)

			r := float64(c00.R)*(1-fracX)*(1-fracY) +
				float64(c10.R)*fracX*(1-fracY) +
				float64(c01.R)*(1-fracX)*fracY +
				float64(c11.R)*fracX*fracY

			g := float64(c00.G)*(1-fracX)*(1-fracY) +
				float64(c10.G)*fracX*(1-fracY) +
				float64(c01.G)*(1-fracX)*fracY +
				float64(c11.G)*fracX*fracY

			b := float64(c00.B)*(1-fracX)*(1-fracY) +
				float64(c10.B)*fracX*(1-fracY) +
				float64(c01.B)*(1-fracX)*fracY +
				float64(c11.B)*fracX*fracY

			a := float64(c00.A)*(1-fracX)*(1-fracY) +
				float64(c10.A)*fracX*(1-fracY) +
				float64(c01.A)*(1-fracX)*fracY +
				float64(c11.A)*fracX*fracY

			dst.SetRGBA(dx, dy, color.RGBA{
				R: clampU8(r),
				G: clampU8(g),
				B: clampU8(b),
				A: clampU8(a),
			})
		}
	}
	return dst
}

func rgbaAt(img image.Image, x, y int) color.RGBA {
	c := img.At(x, y)
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// OptimizeForVLM 对截图进行 VLM 优化处理：
// 1. 检测格式并解码
// 2. 按 MaxWidth/MaxHeight 缩放（双线性插值）
// 3. 按 BlockAlign 对齐尺寸
// 4. 按指定格式和质量重新编码
func OptimizeForVLM(data []byte, cfg VLMConfig) (optimized []byte, origW, origH, newW, newH int, err error) {
	if len(data) == 0 {
		return data, 0, 0, 0, 0, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, 0, 0, 0, 0, nil
	}

	bounds := img.Bounds()
	origW = bounds.Dx()
	origH = bounds.Dy()

	newW = origW
	newH = origH

	needResize := false
	if cfg.MaxWidth > 0 && newW > cfg.MaxWidth {
		scale := float64(cfg.MaxWidth) / float64(newW)
		newW = cfg.MaxWidth
		newH = int(float64(newH)*scale + 0.5)
		if newH <= 0 {
			newH = 1
		}
		needResize = true
	}
	if cfg.MaxHeight > 0 && newH > cfg.MaxHeight {
		scale := float64(cfg.MaxHeight) / float64(newH)
		newH = cfg.MaxHeight
		newW = int(float64(newW)*scale + 0.5)
		if newW <= 0 {
			newW = 1
		}
		needResize = true
	}

	if cfg.BlockAlign > 1 {
		alignedW := ((newW + cfg.BlockAlign - 1) / cfg.BlockAlign) * cfg.BlockAlign
		alignedH := ((newH + cfg.BlockAlign - 1) / cfg.BlockAlign) * cfg.BlockAlign
		if alignedW != newW || alignedH != newH {
			newW = alignedW
			newH = alignedH
			needResize = true
		}
	}

	if !needResize {
		fmt := DetectFormat(data)
		if fmt == cfg.Format && cfg.Format != FormatUnknown {
			return data, origW, origH, origW, origH, nil
		}
		var buf bytes.Buffer
		if err := encodeImage(&buf, img, cfg); err != nil {
			return data, origW, origH, origW, origH, nil
		}
		return buf.Bytes(), origW, origH, origW, origH, nil
	}

	resized := ResizeWithInterpolation(img, newW, newH)
	var buf bytes.Buffer
	if err := encodeImage(&buf, resized, cfg); err != nil {
		return data, origW, origH, origW, origH, nil
	}
	return buf.Bytes(), origW, origH, newW, newH, nil
}

func encodeImage(buf *bytes.Buffer, img image.Image, cfg VLMConfig) error {
	switch cfg.Format {
	case FormatJPEG:
		q := cfg.Quality
		if q <= 0 {
			q = 90
		}
		return jpeg.Encode(buf, img, &jpeg.Options{Quality: q})
	case FormatPNG:
		return png.Encode(buf, img)
	default:
		return jpeg.Encode(buf, img, &jpeg.Options{Quality: 90})
	}
}

// Crop 从图像中裁剪指定矩形区域，返回新 RGBA 图像。
// 如果 rect 超出图像边界，会被截断到有效范围内。
func Crop(img image.Image, left, top, right, bottom int) *image.RGBA {
	bounds := img.Bounds()
	if left < bounds.Min.X {
		left = bounds.Min.X
	}
	if top < bounds.Min.Y {
		top = bounds.Min.Y
	}
	if right > bounds.Max.X {
		right = bounds.Max.X
	}
	if bottom > bounds.Max.Y {
		bottom = bounds.Max.Y
	}
	w := right - left
	h := bottom - top
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, img.At(left+x, top+y))
		}
	}
	return dst
}
