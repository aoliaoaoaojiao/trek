package deeplocate

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"

	"trek/internal/engine/core/primitives"
	"trek/internal/vision/coord"
	"trek/internal/vision/imageproc"
)

// DeepLocateConfig 配置两阶段定位行为。
type DeepLocateConfig struct {
	Enabled         bool
	SectionExpandPx int // 区域扩展像素数（默认 100）
	SectionMinSize  int // 区域最小尺寸（默认 400）
	ZoomFactor      int // 第二阶段放大倍数（默认 2）
}

// DefaultConfig 返回默认 DeepLocate 配置。
func DefaultConfig() DeepLocateConfig {
	return DeepLocateConfig{
		Enabled:         true,
		SectionExpandPx: 100,
		SectionMinSize:  400,
		ZoomFactor:      2,
	}
}

// VLMResponse 是 VLM 单次调用的响应（边界框，任意格式）。
type VLMResponse struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

// CropInfo 保存裁剪区域的像素坐标（在全屏中的位置）。
type CropInfo struct {
	Left   int
	Top    int
	Width  int
	Height int
}

// SectionResult 保存第一阶段（区域定位）的结果。
type SectionResult struct {
	SectionRect *primitives.Rect // 全屏空间的归一化 [0,1] 区域边界框
	Crop        CropInfo         // 裁剪区域的像素坐标
	ZoomImage   []byte           // 放大后的区域截图（JPEG）
	ZoomWidth   int              // 放大后宽度
	ZoomHeight  int              // 放大后高度
}

// ElementResult 保存第二阶段（元素定位）的结果。
type ElementResult struct {
	ElementRect *primitives.Rect // 全屏空间的归一化 [0,1] 元素边界框
	ZoomRect    *primitives.Rect // 放大空间的归一化 [0,1] 元素边界框
}

// DoSection 执行 DeepLocate 第一阶段：在全屏截图中定位目标区域。
// screenshot: 原始截图字节
// shotW, shotH: 原始截图尺寸
// vlmResp: VLM 返回的区域边界框
// 返回放大后的区域截图和裁剪信息。
func DoSection(screenshot []byte, shotW, shotH int, vlmResp VLMResponse, cfg DeepLocateConfig) (*SectionResult, error) {
	if len(screenshot) == 0 || shotW <= 0 || shotH <= 0 {
		return nil, fmt.Errorf("deeplocate: invalid screenshot")
	}

	// 1. VLM 输出 → 归一化 [0,1] rect
	sectionRect := coord.AdaptBboxToRect(vlmResp.Left, vlmResp.Top, vlmResp.Right, vlmResp.Bottom, shotW, shotH)

	// 2. 以像素为单位扩展区域
	expandPx := cfg.SectionExpandPx
	if expandPx <= 0 {
		expandPx = 100
	}
	minSize := cfg.SectionMinSize
	if minSize <= 0 {
		minSize = 400
	}

	pxLeft := int(sectionRect.Left*float64(shotW) + 0.5)
	pxTop := int(sectionRect.Top*float64(shotH) + 0.5)
	pxRight := int(sectionRect.Right*float64(shotW) + 0.5)
	pxBottom := int(sectionRect.Bottom*float64(shotH) + 0.5)

	// 扩展
	pxLeft -= expandPx
	pxTop -= expandPx
	pxRight += expandPx
	pxBottom += expandPx

	// 截断到屏幕边界
	if pxLeft < 0 {
		pxLeft = 0
	}
	if pxTop < 0 {
		pxTop = 0
	}
	if pxRight > shotW {
		pxRight = shotW
	}
	if pxBottom > shotH {
		pxBottom = shotH
	}

	// 确保最小尺寸
	cropW := pxRight - pxLeft
	cropH := pxBottom - pxTop
	if cropW < minSize {
		center := (pxLeft + pxRight) / 2
		half := minSize / 2
		pxLeft = center - half
		pxRight = center + half
		if pxLeft < 0 {
			pxLeft = 0
			pxRight = minSize
		}
		if pxRight > shotW {
			pxRight = shotW
			pxLeft = shotW - minSize
		}
	}
	if cropH < minSize {
		center := (pxTop + pxBottom) / 2
		half := minSize / 2
		pxTop = center - half
		pxBottom = center + half
		if pxTop < 0 {
			pxTop = 0
			pxBottom = minSize
		}
		if pxBottom > shotH {
			pxBottom = shotH
			pxTop = shotH - minSize
		}
	}

	cropW = pxRight - pxLeft
	cropH = pxBottom - pxTop

	// 3. 解码截图并裁剪
	img, _, err := image.Decode(bytes.NewReader(screenshot))
	if err != nil {
		return nil, fmt.Errorf("deeplocate: decode screenshot: %w", err)
	}

	cropped := imageproc.Crop(img, pxLeft, pxTop, pxRight, pxBottom)

	// 4. 放大
	zoomFactor := cfg.ZoomFactor
	if zoomFactor <= 0 {
		zoomFactor = 2
	}
	zoomW := cropW * zoomFactor
	zoomH := cropH * zoomFactor
	zoomed := imageproc.ResizeWithInterpolation(cropped, zoomW, zoomH)

	// 5. 编码为 JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, zoomed, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("deeplocate: encode zoom: %w", err)
	}

	return &SectionResult{
		SectionRect: sectionRect,
		Crop: CropInfo{
			Left:   pxLeft,
			Top:    pxTop,
			Width:  cropW,
			Height: cropH,
		},
		ZoomImage:  buf.Bytes(),
		ZoomWidth:  zoomW,
		ZoomHeight: zoomH,
	}, nil
}

// DoElement 执行 DeepLocate 第二阶段：在放大的区域截图中精确定位元素。
// vlmResp: VLM 在放大截图中返回的元素边界框
// section: 第一阶段返回的区域信息
// origW, origH: 原始全屏截图尺寸
// 返回全屏空间的归一化元素 rect。
func DoElement(vlmResp VLMResponse, section *SectionResult, origW, origH int, cfg DeepLocateConfig,
) (*ElementResult, error) {
	if section == nil {
		return nil, fmt.Errorf("deeplocate: nil section result")
	}

	zoomFactor := cfg.ZoomFactor
	if zoomFactor <= 0 {
		zoomFactor = 2
	}

	// 1. VLM 输出 → 放大截图空间的归一化 [0,1] rect
	zoomRect := coord.AdaptBboxToRect(vlmResp.Left, vlmResp.Top, vlmResp.Right, vlmResp.Bottom,
		section.ZoomWidth, section.ZoomHeight)

	// 2. 在放大空间中的像素坐标
	ePxLeft := zoomRect.Left * float64(section.ZoomWidth)
	ePxTop := zoomRect.Top * float64(section.ZoomHeight)
	ePxRight := zoomRect.Right * float64(section.ZoomWidth)
	ePxBottom := zoomRect.Bottom * float64(section.ZoomHeight)

	// 3. 逆缩放（÷ zoomFactor）回到裁剪空间
	ePxLeft /= float64(zoomFactor)
	ePxTop /= float64(zoomFactor)
	ePxRight /= float64(zoomFactor)
	ePxBottom /= float64(zoomFactor)

	// 4. 加 crop 偏移回到全屏像素空间
	ePxLeft += float64(section.Crop.Left)
	ePxTop += float64(section.Crop.Top)
	ePxRight += float64(section.Crop.Left)
	ePxBottom += float64(section.Crop.Top)

	// 5. 归一化到 [0,1] 全屏空间
	nLeft := 0.0
	nTop := 0.0
	nRight := 0.0
	nBottom := 0.0
	if origW > 0 && origH > 0 {
		nLeft = ePxLeft / float64(origW)
		nTop = ePxTop / float64(origH)
		nRight = ePxRight / float64(origW)
		nBottom = ePxBottom / float64(origH)
	} else {
		nLeft = ePxLeft / float64(origW)
		nTop = ePxTop / float64(origH)
		nRight = ePxRight / float64(origW)
		nBottom = ePxBottom / float64(origH)
	}

	// 钳制
	elementRect := coord.ClampRect(&primitives.Rect{
		Left: nLeft, Top: nTop, Right: nRight, Bottom: nBottom,
	})

	return &ElementResult{
		ElementRect: elementRect,
		ZoomRect:    zoomRect,
	}, nil
}
