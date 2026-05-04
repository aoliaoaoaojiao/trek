package util

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
)

var loadDotEnvOnce sync.Once

// LoadDotEnvFiles 加载项目约定的 .env 文件。
// godotenv.Load 仅在环境变量未设置时补充值，因此按高优先级到低优先级依次加载。
func LoadDotEnvFiles() {
	loadDotEnvOnce.Do(func() {
		rootDir, err := findProjectRootFromWorkingDir()
		if err != nil {
			_ = godotenv.Load(".env.local")
			_ = godotenv.Load(".env.development.local")
			_ = godotenv.Load(".env")
			return
		}
		_ = godotenv.Load(filepath.Join(rootDir, ".env.local"))
		_ = godotenv.Load(filepath.Join(rootDir, ".env.development.local"))
		_ = godotenv.Load(filepath.Join(rootDir, ".env"))
	})
}

func findProjectRootFromWorkingDir() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	current := filepath.Clean(startDir)
	for {
		info, statErr := os.Stat(filepath.Join(current, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", os.ErrNotExist
		}
		current = parent
	}
}
