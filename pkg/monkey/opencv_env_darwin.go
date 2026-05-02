//go:build darwin

package monkey

func init() {
	// gocv 在 macOS 下通常依赖 dylib。
	// 支持通过环境变量把运行时目录注入 DYLD_LIBRARY_PATH，
	// 便于仅通过 OPENCV_BIN / OPENCV_LIB / OPENCV_DIR 完成运行期加载。
	prependEnvPath("DYLD_LIBRARY_PATH", resolveOpenCVRuntimeDirFromEnv())
}
