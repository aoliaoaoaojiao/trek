package common

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
)

func GetRandomPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	return port
}

var pluginDirPath = ""

func SetPluginDirPath(dirPath string) {
	pluginDirPath = dirPath
}

func GetPluginDirPath() (string, error) {
	if pluginDirPath != "" {
		return pluginDirPath, nil
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
