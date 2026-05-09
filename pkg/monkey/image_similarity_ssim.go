package monkey

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
)

const (
	ssimK1               = 0.01
	ssimK2               = 0.03
	ssimDynamicRange     = 255.0
	defaultSSIMWindow    = 11
	defaultSSIMSigma     = 1.5
	minComparablePixels  = 4
)

// ComputeImageSSIM 计算两张截图的结构相似性（SSIM）。
// 返回值范围通常在 0~1，越接近 1 表示视觉结构越相似。
func ComputeImageSSIM(first, second []byte) (float64, error) {
	return ComputeImageSSIMWithRegions(first, second, nil)
}

// ComputeImageSSIMWithRegions 支持在整图之外，追加一个或多个归一化 ROI 做加权比较。
// 若传入 regions 为空，则只比较整图；若不为空，则返回“整图 + 自定义区域”的面积加权平均 SSIM。
func ComputeImageSSIMWithRegions(first, second []byte, regions []ImageFingerprintRegion) (float64, error) {
	firstImg, err := decodeComparableImage(first)
	if err != nil {
		return 0, fmt.Errorf("解码第一张图片失败: %w", err)
	}
	secondImg, err := decodeComparableImage(second)
	if err != nil {
		return 0, fmt.Errorf("解码第二张图片失败: %w", err)
	}

	firstBounds := firstImg.Bounds()
	secondBounds := secondImg.Bounds()
	if firstBounds.Empty() || secondBounds.Empty() {
		return 0, errors.New("图片尺寸不能为空")
	}

	pairs := comparableRegions(firstBounds, secondBounds, regions)
	if len(pairs) == 0 {
		return 0, errors.New("没有可比较的有效区域")
	}

	var weightedSum float64
	var totalWeight float64
	for _, pair := range pairs {
		score, weight, scoreErr := computeRegionSSIM(firstImg, pair.first, secondImg, pair.second)
		if scoreErr != nil {
			return 0, scoreErr
		}
		weightedSum += score * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0, errors.New("可比较区域权重不能为空")
	}
	return weightedSum / totalWeight, nil
}

type ssimRegionPair struct {
	first  image.Rectangle
	second image.Rectangle
}

func decodeComparableImage(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, errors.New("图片数据不能为空")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

func comparableRegions(firstBounds, secondBounds image.Rectangle, regions []ImageFingerprintRegion) []ssimRegionPair {
	pairs := []ssimRegionPair{{
		first:  firstBounds,
		second: secondBounds,
	}}

	for _, region := range regions {
		firstRect := normalizeFingerprintRegion(firstBounds, region)
		secondRect := normalizeFingerprintRegion(secondBounds, region)
		if !validComparableRect(firstRect, firstBounds) || !validComparableRect(secondRect, secondBounds) {
			continue
		}
		if containsRegionPair(pairs, firstRect, secondRect) {
			continue
		}
		pairs = append(pairs, ssimRegionPair{first: firstRect, second: secondRect})
	}

	return pairs
}

func containsRegionPair(pairs []ssimRegionPair, first, second image.Rectangle) bool {
	for _, pair := range pairs {
		if pair.first == first && pair.second == second {
			return true
		}
	}
	return false
}

func validComparableRect(rect, parent image.Rectangle) bool {
	if rect.Empty() || !rect.In(parent) {
		return false
	}
	return rect.Dx() >= minComparablePixels && rect.Dy() >= minComparablePixels
}

func computeRegionSSIM(firstImg image.Image, firstRect image.Rectangle, secondImg image.Image, secondRect image.Rectangle) (float64, float64, error) {
	targetWidth := minInt(firstRect.Dx(), secondRect.Dx())
	targetHeight := minInt(firstRect.Dy(), secondRect.Dy())
	if targetWidth < minComparablePixels || targetHeight < minComparablePixels {
		return 0, 0, fmt.Errorf("区域过小，无法计算 SSIM: first=%v second=%v", firstRect, secondRect)
	}

	firstGray := sampleGrayGrid(firstImg, firstRect, targetWidth, targetHeight)
	secondGray := sampleGrayGrid(secondImg, secondRect, targetWidth, targetHeight)
	score, err := computeGraySSIM(firstGray, secondGray, targetWidth, targetHeight)
	if err != nil {
		return 0, 0, err
	}

	weight := float64(targetWidth * targetHeight)
	return score, weight, nil
}

func computeGraySSIM(first, second []uint8, width, height int) (float64, error) {
	if len(first) != width*height || len(second) != width*height {
		return 0, errors.New("灰度图尺寸与数据长度不一致")
	}
	windowSize := adaptiveSSIMWindow(width, height)
	if windowSize <= 0 {
		return 0, errors.New("SSIM 窗口大小非法")
	}

	sigma := adaptiveSSIMSigma(windowSize)
	kernel := gaussianKernel(windowSize, sigma)

	firstFloat := bytesToFloat64(first)
	secondFloat := bytesToFloat64(second)
	firstSquared := multiplyArrays(firstFloat, firstFloat)
	secondSquared := multiplyArrays(secondFloat, secondFloat)
	product := multiplyArrays(firstFloat, secondFloat)

	mu1, outWidth, outHeight := gaussianBlurValid(firstFloat, width, height, kernel)
	mu2, _, _ := gaussianBlurValid(secondFloat, width, height, kernel)
	firstVar, _, _ := gaussianBlurValid(firstSquared, width, height, kernel)
	secondVar, _, _ := gaussianBlurValid(secondSquared, width, height, kernel)
	covariance, _, _ := gaussianBlurValid(product, width, height, kernel)

	if len(mu1) == 0 || len(mu2) == 0 || outWidth <= 0 || outHeight <= 0 {
		return 0, errors.New("SSIM 计算结果为空")
	}

	c1 := math.Pow(ssimK1*ssimDynamicRange, 2)
	c2 := math.Pow(ssimK2*ssimDynamicRange, 2)

	var sum float64
	for i := range mu1 {
		mu1Sq := mu1[i] * mu1[i]
		mu2Sq := mu2[i] * mu2[i]
		mu1Mu2 := mu1[i] * mu2[i]

		sigma1Sq := firstVar[i] - mu1Sq
		sigma2Sq := secondVar[i] - mu2Sq
		sigma12 := covariance[i] - mu1Mu2

		if sigma1Sq < 0 && sigma1Sq > -1e-9 {
			sigma1Sq = 0
		}
		if sigma2Sq < 0 && sigma2Sq > -1e-9 {
			sigma2Sq = 0
		}

		numerator := (2*mu1Mu2 + c1) * (2*sigma12 + c2)
		denominator := (mu1Sq + mu2Sq + c1) * (sigma1Sq + sigma2Sq + c2)
		if denominator == 0 {
			if numerator == 0 {
				sum += 1
				continue
			}
			return 0, errors.New("SSIM 分母为 0")
		}
		sum += numerator / denominator
	}

	return sum / float64(len(mu1)), nil
}

func adaptiveSSIMWindow(width, height int) int {
	window := minInt(defaultSSIMWindow, minInt(width, height))
	if window%2 == 0 {
		window--
	}
	if window < 1 {
		return 1
	}
	return window
}

func adaptiveSSIMSigma(window int) float64 {
	if window <= 1 {
		return 1
	}
	if window == defaultSSIMWindow {
		return defaultSSIMSigma
	}
	return math.Max(0.5, float64(window)/6.0)
}

func gaussianKernel(size int, sigma float64) []float64 {
	kernel := make([]float64, size)
	center := float64(size-1) / 2.0
	var sum float64
	for i := 0; i < size; i++ {
		x := float64(i) - center
		value := math.Exp(-(x * x) / (2 * sigma * sigma))
		kernel[i] = value
		sum += value
	}
	if sum == 0 {
		return kernel
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	return kernel
}

func gaussianBlurValid(src []float64, width, height int, kernel []float64) ([]float64, int, int) {
	horizontal, horizontalWidth := convolveHorizontalValid(src, width, height, kernel)
	vertical, verticalHeight := convolveVerticalValid(horizontal, horizontalWidth, height, kernel)
	return vertical, horizontalWidth, verticalHeight
}

func convolveHorizontalValid(src []float64, width, height int, kernel []float64) ([]float64, int) {
	if len(kernel) == 0 || width < len(kernel) {
		return nil, 0
	}
	outWidth := width - len(kernel) + 1
	out := make([]float64, outWidth*height)
	for y := 0; y < height; y++ {
		rowOffset := y * width
		outOffset := y * outWidth
		for x := 0; x < outWidth; x++ {
			var sum float64
			for k := 0; k < len(kernel); k++ {
				sum += src[rowOffset+x+k] * kernel[k]
			}
			out[outOffset+x] = sum
		}
	}
	return out, outWidth
}

func convolveVerticalValid(src []float64, width, height int, kernel []float64) ([]float64, int) {
	if len(kernel) == 0 || height < len(kernel) {
		return nil, 0
	}
	outHeight := height - len(kernel) + 1
	out := make([]float64, width*outHeight)
	for y := 0; y < outHeight; y++ {
		outOffset := y * width
		for x := 0; x < width; x++ {
			var sum float64
			for k := 0; k < len(kernel); k++ {
				sum += src[(y+k)*width+x] * kernel[k]
			}
			out[outOffset+x] = sum
		}
	}
	return out, outHeight
}

func bytesToFloat64(src []uint8) []float64 {
	dst := make([]float64, len(src))
	for i, value := range src {
		dst[i] = float64(value)
	}
	return dst
}

func multiplyArrays(first, second []float64) []float64 {
	out := make([]float64, len(first))
	for i := range first {
		out[i] = first[i] * second[i]
	}
	return out
}
