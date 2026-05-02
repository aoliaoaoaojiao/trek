package monkey

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	opencvEnvRoot    = "OPENCV_DIR"
	opencvEnvBin     = "OPENCV_BIN"
	opencvEnvInclude = "OPENCV_INCLUDE"
	opencvEnvLib     = "OPENCV_LIB"
	opencvEnvLibName = "OPENCV_LIB_NAME"
)

func resolveOpenCVBinDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv(opencvEnvBin)); dir != "" {
		return dir
	}
	if root := strings.TrimSpace(os.Getenv(opencvEnvRoot)); root != "" {
		return filepath.Join(root, "bin")
	}
	return ""
}

func resolveOpenCVRuntimeDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv(opencvEnvBin)); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv(opencvEnvLib)); dir != "" {
		return dir
	}
	if root := strings.TrimSpace(os.Getenv(opencvEnvRoot)); root != "" {
		libDir := filepath.Join(root, "lib")
		if stat, err := os.Stat(libDir); err == nil && stat.IsDir() {
			return libDir
		}
		return filepath.Join(root, "bin")
	}
	return ""
}

func prependEnvPath(key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	current := strings.TrimSpace(os.Getenv(key))
	if current == "" {
		_ = os.Setenv(key, value)
		return
	}
	parts := strings.Split(current, string(os.PathListSeparator))
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}
	_ = os.Setenv(key, value+string(os.PathListSeparator)+current)
}
