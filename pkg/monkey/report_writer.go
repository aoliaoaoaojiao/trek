package monkey

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// writeStepSnapshotArtifacts 将单步截图和 XML 实时写入磁盘产物目录。
func writeStepSnapshotArtifacts(rootDir string, record StepRecord, phase string, pageName string, xmlText string, screenshot []byte) (*StepArtifactRef, error) {
	pageDirName := sanitizePageDirName(pageName)
	if strings.TrimSpace(pageDirName) == "" {
		pageDirName = "UnknownPage"
	}
	pageDirPath := filepath.Join(rootDir, pageDirName)
	ref := &StepArtifactRef{PageDir: pageDirName}

	needWrite := len(screenshot) > 0 || strings.TrimSpace(xmlText) != ""
	if !needWrite {
		return nil, nil
	}
	if err := os.MkdirAll(pageDirPath, 0755); err != nil {
		return nil, fmt.Errorf("创建页面产物目录失败(%s): %w", pageDirName, err)
	}

	prefix := buildArtifactFilePrefix(record, phase)
	if len(screenshot) > 0 {
		ext := detectImageExt(screenshot)
		fileName := prefix + ext
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), screenshot, 0644); err != nil {
			return nil, fmt.Errorf("写入截图产物失败(%s): %w", fileName, err)
		}
		ref.ScreenshotFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
	}
	if strings.TrimSpace(xmlText) != "" {
		fileName := prefix + ".xml"
		if err := os.WriteFile(filepath.Join(pageDirPath, fileName), []byte(xmlText), 0644); err != nil {
			return nil, fmt.Errorf("写入 XML 产物失败(%s): %w", fileName, err)
		}
		ref.XMLFile = filepath.ToSlash(filepath.Join(pageDirName, fileName))
	}
	if ref.ScreenshotFile == "" && ref.XMLFile == "" {
		return nil, nil
	}
	return ref, nil
}

func buildArtifactFilePrefix(record StepRecord, phase string) string {
	var b strings.Builder
	b.WriteString("step-")
	b.WriteString(strconv.FormatInt(int64(record.Step), 10))
	b.WriteString("-")
	b.WriteString(phase)
	if action := strings.TrimSpace(record.Action); action != "" {
		b.WriteString("-")
		b.WriteString(sanitizePageDirName(action))
	}
	return b.String()
}

func sanitizePageDirName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteRune('_')
		case r == ' ' || r == '\t':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "._")
}

func detectImageExt(data []byte) string {
	if len(data) == 0 {
		return ".png"
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "jpeg":
			return ".jpg"
		case "png":
			return ".png"
		}
	}
	return ".png"
}
