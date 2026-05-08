package monkey

import "strings"

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
