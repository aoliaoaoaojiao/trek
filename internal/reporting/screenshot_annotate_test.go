package reporting

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestAnnotateActionClick(t *testing.T) {
	screenshot := makeTestPNG(t, 200, 200, color.RGBA{R: 0, G: 0, B: 200, A: 255})
	bounds := "[0.200,0.200,0.800,0.800]"

	marked, err := annotateAction(screenshot, "CLICK", bounds)
	if err != nil {
		t.Fatalf("annotateAction 错误: %v", err)
	}
	if len(marked) == 0 {
		t.Fatal("标注截图为空")
	}
	if bytes.Equal(marked, screenshot) {
		t.Fatal("标注截图与原图完全相同，标注未生效")
	}

	// 验证中心点附近像素与原图不同（标注已绘制）
	origImg, _, _ := image.Decode(bytes.NewReader(screenshot))
	markedImg, _, err := image.Decode(bytes.NewReader(marked))
	if err != nil {
		t.Fatalf("解码标注截图失败: %v", err)
	}
	bounds2 := markedImg.Bounds()
	cx := (bounds2.Min.X + bounds2.Max.X) / 2
	cy := (bounds2.Min.Y + bounds2.Max.Y) / 2
	found := false
	for dy := -circleRadius; dy <= circleRadius; dy++ {
		for dx := -circleRadius; dx <= circleRadius; dx++ {
			if dx*dx+dy*dy > circleRadius*circleRadius {
				continue
			}
			origR, origG, origB, _ := origImg.At(cx+dx, cy+dy).RGBA()
			markR, markG, markB, _ := markedImg.At(cx+dx, cy+dy).RGBA()
			if origR != markR || origG != markG || origB != markB {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Errorf("中心点 (%d,%d) 附近像素与原图相同，标注未生效", cx, cy)
	}
}

func TestAnnotateActionScroll(t *testing.T) {
	screenshot := makeTestPNG(t, 200, 200, color.RGBA{R: 0, G: 0, B: 200, A: 255})
	bounds := "[0.100,0.100,0.900,0.900]"

	marked, err := annotateAction(screenshot, "SCROLL_BOTTOM_UP", bounds)
	if err != nil {
		t.Fatalf("annotateAction 错误: %v", err)
	}
	if bytes.Equal(marked, screenshot) {
		t.Fatal("标注截图与原图完全相同，标注未生效")
	}
}

func TestAnnotateActionNonAnnotatable(t *testing.T) {
	screenshot := makeTestPNG(t, 100, 100, color.RGBA{R: 200, A: 255})

	marked, err := annotateAction(screenshot, "BACK", "[0.1,0.1,0.9,0.9]")
	if err != nil {
		t.Fatalf("annotateAction 错误: %v", err)
	}
	if !bytes.Equal(marked, screenshot) {
		t.Fatal("BACK 动作不应生成标注")
	}
}

func TestAnnotateActionEmptyBounds(t *testing.T) {
	screenshot := makeTestPNG(t, 100, 100, color.RGBA{R: 200, A: 255})

	marked, err := annotateAction(screenshot, "CLICK", "")
	if err != nil {
		t.Fatalf("annotateAction 错误: %v", err)
	}
	if !bytes.Equal(marked, screenshot) {
		t.Fatal("空 bounds 不应生成标注")
	}
}

func TestParseNormalizedBounds(t *testing.T) {
	tests := []struct {
		input string
		wantL float64
		wantT float64
		wantR float64
		wantB float64
		ok    bool
	}{
		{"[0.100,0.200,0.300,0.400]", 0.1, 0.2, 0.3, 0.4, true},
		{"[0.5,0.5,0.5,0.5]", 0.5, 0.5, 0.5, 0.5, true},
		{"", 0, 0, 0, 0, false},
		{"[1,2]", 0, 0, 0, 0, false},
	}
	for _, tt := range tests {
		rect, ok := parseNormalizedBounds(tt.input)
		if ok != tt.ok {
			t.Errorf("parseNormalizedBounds(%q) ok=%v, want %v", tt.input, ok, tt.ok)
			continue
		}
		if ok && (rect[0] != tt.wantL || rect[1] != tt.wantT || rect[2] != tt.wantR || rect[3] != tt.wantB) {
			t.Errorf("parseNormalizedBounds(%q) = %v, want [%f,%f,%f,%f]", tt.input, rect, tt.wantL, tt.wantT, tt.wantR, tt.wantB)
		}
	}
}

func makeTestPNG(t *testing.T, w, h int, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("编码 png 失败: %v", err)
	}
	return buf.Bytes()
}
