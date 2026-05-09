package monkey

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"
	"trek/internal/testutil"
)

func TestComputeImageSSIMReturnsOneForSameImage(t *testing.T) {
	data := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	score, err := ComputeImageSSIM(data, data)
	if err != nil {
		t.Fatalf("计算同图 SSIM 失败: %v", err)
	}
	if score < 0.9999 {
		t.Fatalf("同图 SSIM 应接近 1，实际: %.6f", score)
	}
}

func TestComputeImageSSIMDropsAfterVisualMutation(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateGameNavigationFixture(t, original)

	score, err := ComputeImageSSIM(original, mutated)
	if err != nil {
		t.Fatalf("计算变更图 SSIM 失败: %v", err)
	}
	if score >= 0.999 {
		t.Fatalf("明显视觉变更后 SSIM 不应仍接近 1，实际: %.6f", score)
	}
}

func TestComputeImageSSIMWithRegionsAmplifiesContentMutation(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateContentAreaFixture(t, original)

	globalScore, err := ComputeImageSSIM(original, mutated)
	if err != nil {
		t.Fatalf("计算整图 SSIM 失败: %v", err)
	}
	regionScore, err := ComputeImageSSIMWithRegions(original, mutated, []ImageFingerprintRegion{
		{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.6},
	})
	if err != nil {
		t.Fatalf("计算 ROI SSIM 失败: %v", err)
	}
	if regionScore >= globalScore {
		t.Fatalf("追加内容区 ROI 后，SSIM 应更敏感并下降: global=%.6f region=%.6f", globalScore, regionScore)
	}
}

func TestComputeImageSSIMSupportsDifferentImageSizes(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	resized := mustResizeFixture(t, original, 320, 640)

	score, err := ComputeImageSSIM(original, resized)
	if err != nil {
		t.Fatalf("不同尺寸图片计算 SSIM 失败: %v", err)
	}
	if score <= 0.6 {
		t.Fatalf("同内容不同尺寸的 SSIM 不应过低，实际: %.6f", score)
	}
}

func TestComputeImageSSIMRejectsEmptyInput(t *testing.T) {
	if _, err := ComputeImageSSIM(nil, nil); err == nil {
		t.Fatal("空输入应返回错误")
	}
}

func mustResizeFixture(t *testing.T, src []byte, width, height int) []byte {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("解码待缩放图片失败: %v", err)
	}
	bounds := img.Bounds()
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fx := sampleCoord(x, width, bounds.Dx())
			fy := sampleCoord(y, height, bounds.Dy())
			gray := bilinearGrayAt(img, bounds, fx, fy)
			target.Set(x, y, color.Gray{Y: gray})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, target); err != nil {
		t.Fatalf("编码缩放图片失败: %v", err)
	}
	return buf.Bytes()
}

func TestComputeImageSSIMWithRegionsDeduplicatesSameRegion(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateContentAreaFixture(t, original)

	scoreSingle, err := ComputeImageSSIMWithRegions(original, mutated, []ImageFingerprintRegion{
		{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.6},
	})
	if err != nil {
		t.Fatalf("单 ROI SSIM 失败: %v", err)
	}
	scoreDuplicated, err := ComputeImageSSIMWithRegions(original, mutated, []ImageFingerprintRegion{
		{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.6},
		{Left: 0.25, Top: 0.25, Right: 0.75, Bottom: 0.6},
	})
	if err != nil {
		t.Fatalf("重复 ROI SSIM 失败: %v", err)
	}
	if scoreSingle != scoreDuplicated {
		t.Fatalf("重复 ROI 不应影响 SSIM: single=%.6f duplicated=%.6f", scoreSingle, scoreDuplicated)
	}
}

func TestComputeImageSSIMWithRegionsRejectsTooSmallRegion(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	_, err := ComputeImageSSIMWithRegions(original, original, []ImageFingerprintRegion{
		{Left: 0.1, Top: 0.1, Right: 0.1005, Bottom: 0.1005},
	})
	if err != nil {
		t.Fatalf("极小 ROI 应被忽略并回退到整图，而不是报错: %v", err)
	}
}

func TestGaussianKernelIsNormalized(t *testing.T) {
	kernel := gaussianKernel(11, 1.5)
	var sum float64
	for _, value := range kernel {
		sum += value
	}
	if sum < 0.999999 || sum > 1.000001 {
		t.Fatalf("高斯核应归一化，实际总和: %.9f", sum)
	}
}

func TestComputeGraySSIMOnFlatImages(t *testing.T) {
	first := make([]uint8, 64)
	second := make([]uint8, 64)
	for i := range first {
		first[i] = 128
		second[i] = 128
	}
	score, err := computeGraySSIM(first, second, 8, 8)
	if err != nil {
		t.Fatalf("平坦图像 SSIM 失败: %v", err)
	}
	if score < 0.9999 {
		t.Fatalf("相同平坦图像的 SSIM 应接近 1，实际: %.6f", score)
	}
}

func mustPaintRect(t *testing.T, src []byte, rect image.Rectangle, fill color.Color) []byte {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("解码图片失败: %v", err)
	}
	bounds := img.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, img, bounds.Min, draw.Src)
	draw.Draw(canvas, rect, &image.Uniform{C: fill}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		t.Fatalf("编码图片失败: %v", err)
	}
	return buf.Bytes()
}
