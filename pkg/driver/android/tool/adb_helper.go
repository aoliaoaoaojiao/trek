package tool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	adbEnsureMu sync.Mutex
	adbEnsured  bool
)

func EnsureADBServer() error {
	adbEnsureMu.Lock()
	defer adbEnsureMu.Unlock()

	if adbEnsured {
		return nil
	}

	candidates, err := resolveADBExecutables()
	if err != nil {
		return err
	}

	failures := make([]string, 0, len(candidates))
	for _, adbPath := range candidates {
		cmd := exec.Command(adbPath, "start-server")
		output, err := cmd.CombinedOutput()
		if err == nil {
			adbEnsured = true
			return nil
		}

		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			failures = append(failures, fmt.Sprintf("%s: %v", adbPath, err))
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %v: %s", adbPath, err, trimmed))
	}

	return fmt.Errorf("all adb candidates failed: %s", strings.Join(failures, " | "))
}

func runADBCommand(deviceSerial string, args ...string) (string, error) {
	if err := EnsureADBServer(); err != nil {
		return "", err
	}

	candidates, err := resolveADBExecutables()
	if err != nil {
		return "", err
	}

	commandArgs := make([]string, 0, len(args)+2)
	if deviceSerial != "" {
		commandArgs = append(commandArgs, "-s", deviceSerial)
	}
	commandArgs = append(commandArgs, args...)

	cmd := exec.Command(candidates[0], commandArgs...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", fmt.Errorf("adb command failed: %w", err)
		}
		return trimmed, fmt.Errorf("adb command failed: %w: %s", err, trimmed)
	}

	return trimmed, nil
}

func InstallAPK(deviceSerial, apkPath string, replace bool) error {
	if apkPath == "" {
		return fmt.Errorf("apk path cannot be empty")
	}

	absPath, err := filepath.Abs(apkPath)
	if err != nil {
		return fmt.Errorf("resolve apk path failed: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("apk file not found: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("apk path must be a file: %s", absPath)
	}

	args := make([]string, 0, 3)
	args = append(args, "install")
	if replace {
		args = append(args, "-r")
	}
	args = append(args, absPath)

	output, err := runADBCommand(deviceSerial, args...)
	if err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(output), "success") {
		return fmt.Errorf("adb install failed: %s", output)
	}

	return nil
}

func UninstallPackage(deviceSerial, packageName string, keepData bool) error {
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	args := make([]string, 0, 3)
	args = append(args, "uninstall")
	if keepData {
		args = append(args, "-k")
	}
	args = append(args, packageName)

	output, err := runADBCommand(deviceSerial, args...)
	if err != nil {
		return err
	}
	if !strings.Contains(strings.ToLower(output), "success") {
		return fmt.Errorf("adb uninstall failed: %s", output)
	}

	return nil
}

func resolveADBExecutables() ([]string, error) {
	resolved := make([]string, 0, 6)
	seen := make(map[string]struct{}, 6)
	for _, candidate := range adbExecutableCandidates() {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			resolved = append(resolved, candidate)
		}
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("adb executable not found in PATH or bundled env")
	}

	return resolved, nil
}

func adbExecutableCandidates() []string {
	exeName := adbExecutableName()
	candidates := make([]string, 0, 6)
	if adbPath, err := exec.LookPath(exeName); err == nil {
		candidates = append(candidates, adbPath)
	}

	candidates = append(candidates,
		os.Getenv("TREK_ADB_PATH"),
		os.Getenv("ADB_PATH"),
		filepath.Join(os.Getenv("TREK_ADB_HOME"), exeName),
		filepath.Join(os.Getenv("ANDROID_HOME"), "platform-tools", exeName),
		filepath.Join(os.Getenv("ANDROID_SDK_ROOT"), "platform-tools", exeName),
	)

	if projectRoot, err := RepoRootFromCurrentFile(); err == nil {
		candidates = append(candidates, filepath.Join(projectRoot, "env", runtime.GOOS, "adb", exeName))
	}

	return candidates
}

func adbExecutableName() string {
	if runtime.GOOS == "windows" {
		return "adb.exe"
	}
	return "adb"
}

// RepoRootFromCurrentFile 获取当前程序运行路径
func RepoRootFromCurrentFile() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve current file path failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..")), nil
}
