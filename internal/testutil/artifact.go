package testutil

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"trek/internal/engine/perception"
)

// PixelRect 表示图片上的绝对像素矩形。
type PixelRect struct {
	Left   int
	Top    int
	Right  int
	Bottom int
}

var artifactColors = []color.RGBA{
	{R: 255, G: 64, B: 64, A: 255},
	{R: 64, G: 180, B: 255, A: 255},
	{R: 64, G: 220, B: 120, A: 255},
	{R: 255, G: 190, B: 64, A: 255},
	{R: 220, G: 64, B: 255, A: 255},
}

// WriteCandidateOverlayPNG 将候选区域绘制到 fixture 图片上，并输出到仓库根目录 log/test-artifacts。
func WriteCandidateOverlayPNG(
	t *testing.T,
	fixtureName string,
	fileName string,
	items []perception.Candidate,
) string {
	t.Helper()

	sourcePath := RootFixturePath(t, fixtureName)
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("读取待标注图片失败: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码待标注图片失败: %v", err)
	}

	canvas := image.NewRGBA(img.Bounds())
	draw.Draw(canvas, canvas.Bounds(), img, img.Bounds().Min, draw.Src)

	for index, item := range items {
		if item.Command == nil {
			continue
		}
		pos := item.Command.Pos
		left := int(pos.Left * float64(canvas.Bounds().Dx()))
		top := int(pos.Top * float64(canvas.Bounds().Dy()))
		right := int(pos.Right * float64(canvas.Bounds().Dx()))
		bottom := int(pos.Bottom * float64(canvas.Bounds().Dy()))
		drawRectOutline(canvas, image.Rect(left, top, right, bottom), artifactColors[index%len(artifactColors)], 3)
	}

	outputPath := RootArtifactPath(t, fileName)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatalf("创建测试产物目录失败: %v", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("创建测试产物失败: %v", err)
	}
	defer file.Close()

	if err := pngEncode(file, canvas); err != nil {
		t.Fatalf("输出标注图失败: %v", err)
	}
	return outputPath
}

// WritePixelRectOverlayPNG 将像素坐标矩形绘制到 fixture 图片上，并输出到仓库根目录 log/test-artifacts。
func WritePixelRectOverlayPNG(
	t *testing.T,
	fixtureName string,
	fileName string,
	rects []PixelRect,
) string {
	t.Helper()

	sourcePath := RootFixturePath(t, fixtureName)
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("读取待标注图片失败: %v", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解码待标注图片失败: %v", err)
	}

	canvas := image.NewRGBA(img.Bounds())
	draw.Draw(canvas, canvas.Bounds(), img, img.Bounds().Min, draw.Src)

	for index, rect := range rects {
		drawRectOutline(
			canvas,
			image.Rect(rect.Left, rect.Top, rect.Right, rect.Bottom),
			artifactColors[index%len(artifactColors)],
			3,
		)
	}

	outputPath := RootArtifactPath(t, fileName)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatalf("创建测试产物目录失败: %v", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("创建测试产物失败: %v", err)
	}
	defer file.Close()

	if err := pngEncode(file, canvas); err != nil {
		t.Fatalf("输出标注图失败: %v", err)
	}
	return outputPath
}

// RootArtifactPath 返回仓库根目录 log/test-artifacts 下的产物路径。
func RootArtifactPath(t *testing.T, fileName string) string {
	t.Helper()
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		t.Fatal("artifact 文件名不能为空")
	}
	rootDir := requireProjectRoot(t)
	return filepath.Join(rootDir, "log", "test-artifacts", fileName)
}

// WriteArtifactBytes 将任意字节内容输出到仓库根目录 log/test-artifacts。
func WriteArtifactBytes(t *testing.T, fileName string, data []byte) string {
	t.Helper()
	outputPath := RootArtifactPath(t, fileName)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		t.Fatalf("创建测试产物目录失败: %v", err)
	}
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		t.Fatalf("写入测试产物失败: %v", err)
	}
	return outputPath
}

func drawRectOutline(img *image.RGBA, rect image.Rectangle, stroke color.RGBA, thickness int) {
	if img == nil || thickness <= 0 {
		return
	}
	bounds := img.Bounds()
	rect = rect.Intersect(bounds)
	if rect.Empty() {
		return
	}

	for offset := 0; offset < thickness; offset++ {
		top := rect.Min.Y + offset
		bottom := rect.Max.Y - 1 - offset
		left := rect.Min.X + offset
		right := rect.Max.X - 1 - offset
		if left > right || top > bottom {
			break
		}
		for x := left; x <= right; x++ {
			img.SetRGBA(x, top, stroke)
			img.SetRGBA(x, bottom, stroke)
		}
		for y := top; y <= bottom; y++ {
			img.SetRGBA(left, y, stroke)
			img.SetRGBA(right, y, stroke)
		}
	}
}

func pngEncode(file *os.File, img image.Image) error {
	return png.Encode(file, img)
}
