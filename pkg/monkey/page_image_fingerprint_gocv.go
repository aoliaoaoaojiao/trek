//go:build gocv

package monkey

import (
	"encoding/hex"
	"image"

	"gocv.io/x/gocv"
)

// defaultImageFingerprintName 使用 gocv 生成截图感知哈希。
// 算法采用灰度缩放后的 dHash，兼顾稳定性与计算成本。
func defaultImageFingerprintName(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	mat, err := gocv.IMDecode(data, gocv.IMReadColor)
	if err != nil || mat.Empty() {
		return ""
	}
	defer mat.Close()

	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(mat, &gray, gocv.ColorBGRToGray)

	resized := gocv.NewMat()
	defer resized.Close()
	gocv.Resize(gray, &resized, image.Pt(9, 8), 0, 0, gocv.InterpolationArea)
	if resized.Empty() {
		return ""
	}

	hashBytes := make([]byte, 8)
	for y := 0; y < 8; y++ {
		var row byte
		for x := 0; x < 8; x++ {
			left := resized.GetUCharAt(y, x)
			right := resized.GetUCharAt(y, x+1)
			if left > right {
				row |= 1 << uint(7-x)
			}
		}
		hashBytes[y] = row
	}
	return imageFingerprintPrefix + ":" + hex.EncodeToString(hashBytes)
}
