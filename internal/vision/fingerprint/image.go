package fingerprint

import (
	"bytes"
	"encoding/hex"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math/bits"
	"strings"

	"trek/internal/config"
)

// DefaultHammingThreshold 是默认的 Hamming 距离阈值。
// 对于默认 2 region（512 bit）指纹，阈值 10 表示允许约 2% 的 bit 差异。
// 状态栏时间/电量变化通常导致 10-20 bits 差异，阈值 10 可以过滤这些噪声。
const DefaultHammingThreshold = config.DefaultImageFingerprintHammingThreshold

const (
	// Prefix 是图片页面指纹的统一前缀。
	Prefix            = "IMGPage"
	fingerprintWidth  = 9
	fingerprintHeight = 8
)

// Region 定义截图上的归一化局部区域，用于增强局部变化敏感度。
type Region struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

// Name 基于“整图 + 局部区域”组合感知哈希生成稳定图片指纹。
func Name(data []byte, regions []Region) string {
	if len(data) == 0 {
		return ""
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		return ""
	}

	rects := fingerprintRegions(bounds, regions)
	hashBytes := make([]byte, 0, len(rects)*fingerprintHeight*2)
	for _, region := range rects {
		hashBytes = append(hashBytes, regionFingerprintBytes(img, region)...)
	}

	return Prefix + ":" + hex.EncodeToString(hashBytes)
}

func regionFingerprintBytes(img image.Image, bounds image.Rectangle) []byte {
	sampled := sampleGrayGrid(img, bounds, fingerprintWidth, fingerprintHeight)
	hashBytes := make([]byte, fingerprintHeight*2)
	for y := 0; y < fingerprintHeight; y++ {
		var row byte
		for x := 0; x < fingerprintWidth-1; x++ {
			left := sampled[y*fingerprintWidth+x]
			right := sampled[y*fingerprintWidth+x+1]
			if left > right {
				row |= 1 << uint(7-x)
			}
		}
		hashBytes[y] = row
	}

	avgThreshold := averageGray(sampled, fingerprintWidth-1, fingerprintHeight)
	for y := 0; y < fingerprintHeight; y++ {
		var row byte
		for x := 0; x < fingerprintWidth-1; x++ {
			if sampled[y*fingerprintWidth+x] >= avgThreshold {
				row |= 1 << uint(7-x)
			}
		}
		hashBytes[fingerprintHeight+y] = row
	}
	return hashBytes
}

func fingerprintRegions(bounds image.Rectangle, custom []Region) []image.Rectangle {
	regions := []image.Rectangle{bounds}

	for _, region := range custom {
		rect := normalizeFingerprintRegion(bounds, region)
		if validFingerprintRect(rect, bounds) && !containsRect(regions, rect) {
			regions = append(regions, rect)
		}
	}
	if len(regions) > 1 {
		return regions
	}

	// 中间内容区更贴近滚动列表、卡片流、对话区等实际变化区域，
	// 组合进去后能降低“整图结构相近但局部内容变化”被误判为同页的概率。
	content := insetByRatio(bounds, 0.08, 0.18, 0.08, 0.12)
	if validFingerprintRect(content, bounds) && !containsRect(regions, content) {
		regions = append(regions, content)
	}

	return regions
}

func normalizeFingerprintRegion(bounds image.Rectangle, region Region) image.Rectangle {
	left := clamp01(region.Left)
	top := clamp01(region.Top)
	right := clamp01(region.Right)
	bottom := clamp01(region.Bottom)
	if right <= left || bottom <= top {
		return image.Rectangle{}
	}

	x0 := bounds.Min.X + int(float64(bounds.Dx())*left)
	y0 := bounds.Min.Y + int(float64(bounds.Dy())*top)
	x1 := bounds.Min.X + int(float64(bounds.Dx())*right)
	y1 := bounds.Min.Y + int(float64(bounds.Dy())*bottom)
	return image.Rect(x0, y0, x1, y1)
}

func insetByRatio(bounds image.Rectangle, left, top, right, bottom float64) image.Rectangle {
	width := bounds.Dx()
	height := bounds.Dy()
	x0 := bounds.Min.X + int(float64(width)*left)
	y0 := bounds.Min.Y + int(float64(height)*top)
	x1 := bounds.Max.X - int(float64(width)*right)
	y1 := bounds.Max.Y - int(float64(height)*bottom)
	return image.Rect(x0, y0, x1, y1)
}

func validFingerprintRect(rect, parent image.Rectangle) bool {
	if rect.Empty() {
		return false
	}
	if !rect.In(parent) {
		return false
	}
	return rect.Dx() >= fingerprintWidth && rect.Dy() >= fingerprintHeight
}

func containsRect(rects []image.Rectangle, target image.Rectangle) bool {
	for _, rect := range rects {
		if rect == target {
			return true
		}
	}
	return false
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

// sampleGrayGrid 按目标网格做双线性采样，兼顾缩放稳定性与实现成本。
// 返回一维平铺数组，减少热点路径里的小对象分配。
func sampleGrayGrid(img image.Image, bounds image.Rectangle, width, height int) []uint8 {
	grid := make([]uint8, width*height)
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			fx := sampleCoord(x, width, srcWidth)
			fy := sampleCoord(y, height, srcHeight)
			grid[y*width+x] = bilinearGrayAt(img, bounds, fx, fy)
		}
	}

	return grid
}

func sampleCoord(index, dstSize, srcSize int) float64 {
	if dstSize <= 1 || srcSize <= 1 {
		return 0
	}
	return float64(index) * float64(srcSize-1) / float64(dstSize-1)
}

func bilinearGrayAt(img image.Image, bounds image.Rectangle, fx, fy float64) uint8 {
	x0 := int(fx)
	y0 := int(fy)
	x1 := minInt(x0+1, bounds.Dx()-1)
	y1 := minInt(y0+1, bounds.Dy()-1)

	wx := fx - float64(x0)
	wy := fy - float64(y0)

	p00 := float64(grayAt(img, bounds.Min.X+x0, bounds.Min.Y+y0))
	p10 := float64(grayAt(img, bounds.Min.X+x1, bounds.Min.Y+y0))
	p01 := float64(grayAt(img, bounds.Min.X+x0, bounds.Min.Y+y1))
	p11 := float64(grayAt(img, bounds.Min.X+x1, bounds.Min.Y+y1))

	top := p00*(1-wx) + p10*wx
	bottom := p01*(1-wx) + p11*wx
	return uint8(top*(1-wy) + bottom*wy + 0.5)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func averageGray(sampled []uint8, width, height int) uint8 {
	var sum uint64
	var count uint64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sum += uint64(sampled[y*fingerprintWidth+x])
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return uint8(sum / count)
}

func grayAt(img image.Image, x, y int) uint8 {
	r, g, b, _ := img.At(x, y).RGBA()

	// RGBA 返回 16 位通道，这里按 Rec.601 近似换算为 8 位灰度。
	gray := (299*r + 587*g + 114*b + 500) / 1000
	return uint8(gray >> 8)
}

// HammingDistance 计算两个 IMGPage: 指纹之间的 Hamming 距离（不同 bit 数）。
// 两个指纹的 region 数必须相同，否则返回 -1。
func HammingDistance(a, b string) int {
	aBytes := fingerprintHashBytes(a)
	bBytes := fingerprintHashBytes(b)
	if aBytes == nil || bBytes == nil || len(aBytes) != len(bBytes) {
		return -1
	}
	distance := 0
	for i := range aBytes {
		distance += bits.OnesCount8(aBytes[i] ^ bBytes[i])
	}
	return distance
}

// fingerprintHashBytes 从 "IMGPage:xxxx" 格式中提取原始哈希字节。
func fingerprintHashBytes(name string) []byte {
	hexStr := strings.TrimPrefix(name, Prefix+":")
	if len(hexStr) == 0 {
		return nil
	}
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil
	}
	return decoded
}
