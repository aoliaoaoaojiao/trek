// Package imageproc 提供 VLM 优化的图像处理管线，包括双线性插值缩放、
// 格式检测和多模型兼容的 JPEG/PNG 编码。
//
// 参考 Midscene 的 transform.ts 设计：
//   - 双线性插值（替代 Midscene 的 Sharp Lanczos3 / Photon CatmullRom）
//   - 模型感知缩放（GPT-4o 2048×768、Qwen2.5-VL 28px block align）
//   - JPEG quality 90（Midscene 标准）
package imageproc

import (
	"encoding/base64"
)

// ImageFormat 表示图像编码格式。
type ImageFormat int

const (
	FormatUnknown ImageFormat = iota
	FormatPNG
	FormatJPEG
)

// MediaType 返回 MIME 类型字符串。
func (f ImageFormat) MediaType() string {
	switch f {
	case FormatPNG:
		return "image/png"
	case FormatJPEG:
		return "image/jpeg"
	default:
		return ""
	}
}

// DetectFormat 通过魔数检测图像格式。
// PNG:  89 50 4E 47 0D 0A 1A 0A
// JPEG: FF D8 FF
func DetectFormat(data []byte) ImageFormat {
	if len(data) < 4 {
		return FormatUnknown
	}
	// PNG 签名
	if len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 &&
		data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A &&
		data[6] == 0x1A && data[7] == 0x0A {
		return FormatPNG
	}
	// JPEG 签名
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return FormatJPEG
	}
	return FormatUnknown
}

// ToBase64 编码字节为标准 base64 字符串。
func ToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// FromBase64 解码 base64 字符串为字节。
func FromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
