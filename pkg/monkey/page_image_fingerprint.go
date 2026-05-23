package monkey

import (
	"strings"

	visionfingerprint "trek/internal/vision/fingerprint"
)

const imageFingerprintPrefix = "IMGPage"

func resolveImageFingerprintPageName(screenshot []byte, custom func([]byte) string, regions []ImageFingerprintRegion) string {
	if len(screenshot) == 0 {
		return ""
	}
	if custom != nil {
		if name := strings.TrimSpace(custom(screenshot)); name != "" {
			return name
		}
	}
	return strings.TrimSpace(defaultImageFingerprintNameWithRegions(screenshot, regions))
}

// ResolveImageFingerprintPageName 基于截图生成页面名，供 web 预览等外部入口复用。
func ResolveImageFingerprintPageName(screenshot []byte, regions []ImageFingerprintRegion) string {
	return resolveImageFingerprintPageName(screenshot, nil, regions)
}

// fuzzyEntry 记录一个已见过的指纹及其首次分配的页面名。
type fuzzyEntry struct {
	name string
	hash string
}

// FuzzyPageNameMatcher 使用 Hamming 距离对图片指纹做模糊去重。
// 同一次运行中，视觉差异极小的截图会被映射到同一个页面名。
type FuzzyPageNameMatcher struct {
	threshold int
	seen      []fuzzyEntry
}

// NewFuzzyPageNameMatcher 创建模糊匹配器。threshold 为允许的最大 Hamming 距离。
func NewFuzzyPageNameMatcher(threshold int) *FuzzyPageNameMatcher {
	return &FuzzyPageNameMatcher{threshold: threshold}
}

// Resolve 基于截图计算指纹，并与已见指纹做 Hamming 距离比较。
// 若距离 <= threshold，复用已有页面名；否则分配新页面名。
func (m *FuzzyPageNameMatcher) Resolve(screenshot []byte, regions []ImageFingerprintRegion) string {
	exact := resolveImageFingerprintPageName(screenshot, nil, regions)
	if exact == "" || m.threshold <= 0 {
		return exact
	}
	for _, entry := range m.seen {
		if visionfingerprint.HammingDistance(exact, entry.hash) <= m.threshold {
			return entry.name
		}
	}
	m.seen = append(m.seen, fuzzyEntry{name: exact, hash: exact})
	return exact
}
