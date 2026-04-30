package common

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"sync"
)

func GetRandomPort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("获取随机端口失败: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return port, nil
}

var (
	pluginDirPath = ""
	pluginDirMu   sync.RWMutex
)

func SetPluginDirPath(dirPath string) {
	pluginDirMu.Lock()
	pluginDirPath = dirPath
	pluginDirMu.Unlock()
}

func GetPluginDirPath() (string, error) {
	pluginDirMu.RLock()
	dir := pluginDirPath
	pluginDirMu.RUnlock()

	if dir != "" {
		return dir, nil
	}
	projectRoot, err := RepoRootFromCurrentFile()
	if err != nil {
		return "", fmt.Errorf("resolve repo root failed: %w", err)
	}

	return filepath.Join(projectRoot, "plugins", "uia"), nil
}

// RepoRootFromCurrentFile 获取当前程序运行路径
func RepoRootFromCurrentFile() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve current file path failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile))), nil
}
