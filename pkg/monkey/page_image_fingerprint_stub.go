//go:build !gocv

package monkey

// defaultImageFingerprintName 是无 gocv 构建下的降级实现。
// 为避免在未安装 OpenCV 的环境中引入硬编译失败，这里返回空字符串，
// 调用方会继续回退到自定义解析器或 XML 策略。
func defaultImageFingerprintName(_ []byte) string {
	return ""
}
