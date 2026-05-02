package monkey

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenCVBinDirFromEnvPrefersExplicitBin(t *testing.T) {
	t.Setenv(opencvEnvRoot, `C:\opencv\build`)
	t.Setenv(opencvEnvBin, `D:\opencv\bin`)

	got := resolveOpenCVBinDirFromEnv()
	if got != `D:\opencv\bin` {
		t.Fatalf("应优先使用显式 bin 目录，实际: %s", got)
	}
}

func TestResolveOpenCVBinDirFromEnvFallsBackToRootBin(t *testing.T) {
	t.Setenv(opencvEnvRoot, `C:\opencv\build`)
	t.Setenv(opencvEnvBin, "")

	got := resolveOpenCVBinDirFromEnv()
	expect := filepath.Join(`C:\opencv\build`, "bin")
	if got != expect {
		t.Fatalf("应回退到 root/bin，实际: %s", got)
	}
}

func TestResolveOpenCVRuntimeDirFromEnvPrefersLib(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("创建 lib 目录失败: %v", err)
	}
	t.Setenv(opencvEnvRoot, root)
	t.Setenv(opencvEnvBin, "")
	t.Setenv(opencvEnvLib, "")

	got := resolveOpenCVRuntimeDirFromEnv()
	if got != libDir {
		t.Fatalf("运行期目录应优先回退到 root/lib，实际: %s", got)
	}
}

func TestResolveOpenCVRuntimeDirFromEnvFallsBackToLibEnv(t *testing.T) {
	t.Setenv(opencvEnvRoot, "")
	t.Setenv(opencvEnvBin, "")
	t.Setenv(opencvEnvLib, `D:\opencv\lib`)

	got := resolveOpenCVRuntimeDirFromEnv()
	if got != `D:\opencv\lib` {
		t.Fatalf("运行期目录应优先使用显式 lib 目录，实际: %s", got)
	}
}

func TestPrependEnvPathAvoidsDuplicate(t *testing.T) {
	const key = "TREK_TEST_PATH_KEY"
	sep := string(os.PathListSeparator)
	t.Setenv(key, `C:\a`+sep+`C:\b`)

	prependEnvPath(key, `C:\b`)
	got := os.Getenv(key)
	expect := `C:\a` + sep + `C:\b`
	if got != expect {
		t.Fatalf("不应重复追加 PATH，实际: %s", got)
	}
}
