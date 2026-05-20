package monkey

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func makeTestScreenshot(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func decodePNGSize(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func TestCropScreenshotByNormalizedRange(t *testing.T) {
	// 200x100 screenshot
	screenshot := makeTestScreenshot(200, 100)

	// Crop to middle 50% horizontally, full height
	area := EffectiveTouchRange{Left: 0.25, Top: 0, Right: 0.75, Bottom: 1.0}
	cropped := cropScreenshotByNormalizedRange(screenshot, area)

	w, h := decodePNGSize(cropped)
	if w != 100 || h != 100 {
		t.Fatalf("裁剪后尺寸不符合预期: %dx%d, 期望 100x100", w, h)
	}
}

func TestCropScreenshotMiddleRegion(t *testing.T) {
	screenshot := makeTestScreenshot(200, 200)

	// Crop to center 50%
	area := EffectiveTouchRange{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.75}
	cropped := cropScreenshotByNormalizedRange(screenshot, area)

	w, h := decodePNGSize(cropped)
	if w != 100 || h != 100 {
		t.Fatalf("裁剪后尺寸不符合预期: %dx%d, 期望 100x100", w, h)
	}
}

func TestCropScreenshotSkipsFullScreen(t *testing.T) {
	screenshot := makeTestScreenshot(200, 100)

	// Full screen range — should return original
	area := EffectiveTouchRange{Left: 0, Top: 0, Right: 1, Bottom: 1}
	cropped := cropScreenshotByNormalizedRange(screenshot, area)

	if len(cropped) != len(screenshot) {
		t.Fatalf("全屏范围不应裁剪: got %d bytes, want %d", len(cropped), len(screenshot))
	}
}

func TestCropScreenshotSkipsNearlyFullScreen(t *testing.T) {
	screenshot := makeTestScreenshot(200, 100)

	// Nearly full screen — should still skip
	area := EffectiveTouchRange{Left: -0.01, Top: -0.01, Right: 1.01, Bottom: 1.01}
	cropped := cropScreenshotByNormalizedRange(screenshot, area)

	if len(cropped) != len(screenshot) {
		t.Fatalf("近似全屏范围不应裁剪: got %d bytes, want %d", len(cropped), len(screenshot))
	}
}

func TestCropScreenshotHandlesEmptyInput(t *testing.T) {
	area := EffectiveTouchRange{Left: 0.1, Top: 0.1, Right: 0.9, Bottom: 0.9}
	cropped := cropScreenshotByNormalizedRange(nil, area)
	if cropped != nil {
		t.Fatalf("nil 输入应返回 nil")
	}
	cropped = cropScreenshotByNormalizedRange([]byte{}, area)
	if len(cropped) != 0 {
		t.Fatalf("空输入应返回空")
	}
}

func TestCropScreenshotHandlesInvalidPNG(t *testing.T) {
	area := EffectiveTouchRange{Left: 0.1, Top: 0.1, Right: 0.9, Bottom: 0.9}
	cropped := cropScreenshotByNormalizedRange([]byte("not a png"), area)
	if string(cropped) != "not a png" {
		t.Fatalf("无效 PNG 应原样返回")
	}
}
