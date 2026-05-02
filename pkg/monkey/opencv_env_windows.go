//go:build windows

package monkey

func init() {
	// gocv 在 Windows 下依赖 opencv_world*.dll。
	// 支持通过环境变量把 OpenCV bin 目录注入 PATH，
	// 便于仅通过 OPENCV_BIN / OPENCV_DIR 完成运行期加载。
	prependEnvPath("PATH", resolveOpenCVBinDirFromEnv())
}
