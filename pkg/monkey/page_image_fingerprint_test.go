package monkey

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"trek/internal/testutil"
)

func TestDefaultImageFingerprintNameWithFixture(t *testing.T) {
	for _, fixtureName := range testutil.ListRootFixtures(t) {
		fixtureName := fixtureName
		t.Run(testutil.FixtureStem(fixtureName), func(t *testing.T) {
			data := testutil.ReadRootFixture(t, fixtureName)

			fingerprint := defaultImageFingerprintName(data)
			if fingerprint == "" {
				t.Fatal("页面指纹不应为空")
			}
			if !strings.HasPrefix(fingerprint, imageFingerprintPrefix+":") {
				t.Fatalf("页面指纹前缀错误: %s", fingerprint)
			}
		})
	}
}

func TestDefaultImageFingerprintNameIsStableForSameImage(t *testing.T) {
	for _, fixtureName := range testutil.ListRootFixtures(t) {
		fixtureName := fixtureName
		t.Run(testutil.FixtureStem(fixtureName), func(t *testing.T) {
			data := testutil.ReadRootFixture(t, fixtureName)

			first := defaultImageFingerprintName(data)
			second := defaultImageFingerprintName(data)
			if first == "" || second == "" {
				t.Fatalf("同图指纹不应为空: first=%q second=%q", first, second)
			}
			if first != second {
				t.Fatalf("同图指纹应稳定一致: first=%s second=%s", first, second)
			}
		})
	}
}

func TestDefaultImageFingerprintNameChangesAfterVisualMutation(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateGameNavigationFixture(t, original)

	originalFingerprint := defaultImageFingerprintName(original)
	mutatedFingerprint := defaultImageFingerprintName(mutated)
	if originalFingerprint == "" || mutatedFingerprint == "" {
		t.Fatalf("变更前后指纹都应可计算: original=%q mutated=%q", originalFingerprint, mutatedFingerprint)
	}
	if originalFingerprint == mutatedFingerprint {
		t.Fatalf("明显视觉改动后指纹应变化: original=%s mutated=%s", originalFingerprint, mutatedFingerprint)
	}
}

func TestDefaultImageFingerprintNameChangesAfterContentAreaMutation(t *testing.T) {
	original := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	mutated := mustMutateContentAreaFixture(t, original)

	originalFingerprint := defaultImageFingerprintName(original)
	mutatedFingerprint := defaultImageFingerprintName(mutated)
	if originalFingerprint == "" || mutatedFingerprint == "" {
		t.Fatalf("内容区变更前后指纹都应可计算: original=%q mutated=%q", originalFingerprint, mutatedFingerprint)
	}
	if originalFingerprint == mutatedFingerprint {
		t.Fatalf("中部内容区变化后指纹应变化: original=%s mutated=%s", originalFingerprint, mutatedFingerprint)
	}
}

func TestFingerprintRegionsUsesCustomRegions(t *testing.T) {
	bounds := image.Rect(0, 0, 1000, 2000)
	regions := fingerprintRegions(bounds, []ImageFingerprintRegion{
		{Left: 0.2, Top: 0.3, Right: 0.8, Bottom: 0.7},
	})
	if len(regions) != 2 {
		t.Fatalf("自定义 ROI 时应返回整图 + 自定义区域，实际: %d", len(regions))
	}
	if regions[0] != bounds {
		t.Fatalf("第一个区域应始终是整图: %+v", regions[0])
	}
	expect := image.Rect(200, 600, 800, 1400)
	if regions[1] != expect {
		t.Fatalf("自定义 ROI 映射错误: got=%+v want=%+v", regions[1], expect)
	}
}

func TestFingerprintRegionsDeduplicatesSameRegion(t *testing.T) {
	bounds := image.Rect(0, 0, 1000, 2000)
	regions := fingerprintRegions(bounds, []ImageFingerprintRegion{
		{Left: 0.2, Top: 0.3, Right: 0.8, Bottom: 0.7},
		{Left: 0.2, Top: 0.3, Right: 0.8, Bottom: 0.7},
	})
	if len(regions) != 2 {
		t.Fatalf("重复 ROI 应去重，实际区域数: %d", len(regions))
	}
}

func mustMutateGameNavigationFixture(t *testing.T, src []byte) []byte {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}
	bounds := img.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, img, bounds.Min, draw.Src)

	// 在左上区域打上一块黑色标记，确保缩放到 9x8 后仍会影响 dHash。
	markRect := image.Rect(0, 0, bounds.Dx()/4, bounds.Dy()/4)
	draw.Draw(canvas, markRect, &image.Uniform{C: color.RGBA{R: 0, G: 0, B: 0, A: 255}}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		t.Fatalf("重新编码变更图片失败: %v", err)
	}
	return buf.Bytes()
}

func mustMutateContentAreaFixture(t *testing.T, src []byte) []byte {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}
	bounds := img.Bounds()
	canvas := image.NewRGBA(bounds)
	draw.Draw(canvas, bounds, img, bounds.Min, draw.Src)

	markRect := image.Rect(
		bounds.Min.X+bounds.Dx()/3,
		bounds.Min.Y+bounds.Dy()/3,
		bounds.Min.X+bounds.Dx()*2/3,
		bounds.Min.Y+bounds.Dy()/2,
	)
	draw.Draw(canvas, markRect, &image.Uniform{C: color.RGBA{R: 255, G: 255, B: 255, A: 255}}, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		t.Fatalf("重新编码内容区变更图片失败: %v", err)
	}
	return buf.Bytes()
}

func readTestStarImage(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "ImgPageTest", "star", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取测试图片失败: path=%s err=%v", path, err)
	}
	return data
}

func TestFuzzyPageNameMatcher_StarScreenshots_SamePage(t *testing.T) {
	starA := readTestStarImage(t, "star_a.png")
	starB := readTestStarImage(t, "star_b.png")
	starC := readTestStarImage(t, "star_c.png")

	matcher := NewFuzzyPageNameMatcher(6)

	nameA := matcher.Resolve(starA, nil)
	nameB := matcher.Resolve(starB, nil)
	nameC := matcher.Resolve(starC, nil)

	if nameA == "" || nameB == "" || nameC == "" {
		t.Fatalf("页面名不应为空: A=%q B=%q C=%q", nameA, nameB, nameC)
	}

	// 三张相似截图应映射到同一个页面名
	if nameA != nameB {
		t.Fatalf("star_a 和 star_b 应映射到同一页面名: A=%q B=%q", nameA, nameB)
	}
	if nameA != nameC {
		t.Fatalf("star_a 和 star_c 应映射到同一页面名: A=%q C=%q", nameA, nameC)
	}
}

func TestFuzzyPageNameMatcher_ThresholdZero_Disabled(t *testing.T) {
	starA := readTestStarImage(t, "star_a.png")
	starB := readTestStarImage(t, "star_b.png")

	matcher := NewFuzzyPageNameMatcher(0)

	nameA := matcher.Resolve(starA, nil)
	nameB := matcher.Resolve(starB, nil)

	if nameA == "" || nameB == "" {
		t.Fatalf("页面名不应为空: A=%q B=%q", nameA, nameB)
	}

	// 阈值为 0 时禁用模糊匹配，应返回不同页面名
	if nameA == nameB {
		t.Fatalf("阈值为 0 时不应合并: A=%q B=%q", nameA, nameB)
	}
}

func TestFuzzyPageNameMatcher_DifferentPage_DifferentName(t *testing.T) {
	gameNav := testutil.ReadRootFixture(t, testutil.FixtureGameNavigation)
	starA := readTestStarImage(t, "star_a.png")

	matcher := NewFuzzyPageNameMatcher(6)

	nameNav := matcher.Resolve(gameNav, nil)
	nameStar := matcher.Resolve(starA, nil)

	if nameNav == "" || nameStar == "" {
		t.Fatalf("页面名不应为空: nav=%q star=%q", nameNav, nameStar)
	}

	// 完全不同的界面应返回不同的页面名
	if nameNav == nameStar {
		t.Fatalf("不同界面不应映射到同一页面名: nav=%q star=%q", nameNav, nameStar)
	}
}

func TestFuzzyPageNameMatcher穩態(t *testing.T) {
	starA := readTestStarImage(t, "star_a.png")

	matcher := NewFuzzyPageNameMatcher(6)

	// 第一次见到
	name1 := matcher.Resolve(starA, nil)
	// 第二次见到同一张图
	name2 := matcher.Resolve(starA, nil)

	if name1 != name2 {
		t.Fatalf("同一张图多次调用应返回相同页面名: first=%q second=%q", name1, name2)
	}
}
