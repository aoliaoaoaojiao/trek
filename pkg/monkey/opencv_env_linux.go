//go:build linux

package monkey

func init() {
	// gocv 在 Linux 下通常依赖 libopencv_*.so 或 OpenCV 相关共享库。
	// 支持通过环境变量把运行时目录注入 LD_LIBRARY_PATH，
	// 便于仅通过 OPENCV_BIN / OPENCV_LIB / OPENCV_DIR 完成运行期加载。
	prependEnvPath("LD_LIBRARY_PATH", resolveOpenCVRuntimeDirFromEnv())
}
