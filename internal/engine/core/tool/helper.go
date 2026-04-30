package tool

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"time"
)

// HashString 计算字符串哈希值
func HashString(s string) uintptr {
	h := fnv.New64a()
	h.Write([]byte(s))
	return uintptr(h.Sum64())
}

// HashInt 计算整数哈希值
func HashInt(i int) uintptr {
	h := fnv.New64a()
	h.Write([]byte(fmt.Sprintf("%d", i)))
	return uintptr(h.Sum64())
}

// IsZhCnByte 判断字节是否为中文字符
func IsZhCnByte(b byte) bool {
	// 正确的UTF-8中文字符范围检查
	return b >= 0xE4 && b <= 0xE9
}

// IsZhCn 判断是否为中文字符
func IsZhCn(c rune) bool {
	return c >= 0x4e00 && c <= 0x9fff
}

// RandomInt 生成随机数
func RandomInt(min, max int) int {
	if min >= max {
		return min
	}
	rand.Seed(time.Now().UnixNano())
	return min + rand.Intn(max-min)
}

// RandomIntWithSeed 使用种子生成随机数
func RandomIntWithSeed(min, max, seed int) int {
	if min >= max {
		return min
	}
	rand.Seed(int64(time.Now().Unix() + int64(seed*2989)))
	return min + rand.Intn(max-min)
}

// TrimString 去除字符串首尾空格
func TrimString(str string) string {
	return strings.TrimSpace(str)
}

// SplitString 分割字符串
func SplitString(str string, splitChar byte) []string {
	return strings.Split(str, string(splitChar))
}

// StringReplaceAll 替换字符串中的所有匹配项
func StringReplaceAll(str, from, to string) string {
	return strings.ReplaceAll(str, from, to)
}

// GetTimeFormatStr 获取格式化时间字符串
func GetTimeFormatStr() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// CurrentStamp 获取当前时间戳(毫秒)
func CurrentStamp() float64 {
	return float64(time.Now().UnixNano() / int64(time.Millisecond))
}

// AlphabetSeq 英文字母序列
const (
	AlphabetSeqLen   = 84
	AlphabetChSeqLen = 64
)

var (
	AlphabetSeq   = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz~!@#$%^&*()<>_-;',.?/\"|{}"
	AlphabetChSeq = "你好中文字符串｜。，；（）【】？！测试啊哈"
)

// GetRandomChars 获取随机字符串
func GetRandomChars() string {
	length := RandomInt(11, 1000)
	var result strings.Builder
	seed := uint32(time.Now().Unix())

	for length > 0 {
		// 使用类似C++的随机数生成逻辑
		seed = seed*1103515245 + 12345
		randNum := int(seed % uint32(AlphabetSeqLen*4+AlphabetChSeqLen))

		if randNum < AlphabetSeqLen*4 {
			// 选择ASCII字符
			idx := randNum / 4
			result.WriteByte(AlphabetSeq[idx])
			length--
		} else {
			// 选择中文字符（3个字符一组）
			idx := (randNum - AlphabetSeqLen*4) / 3 * 3
			if idx+2 < AlphabetChSeqLen {
				result.WriteString(AlphabetChSeq[idx : idx+3])
				length -= 3
			} else {
				result.WriteByte(AlphabetSeq[randNum%len(AlphabetSeq)])
				length--
			}
		}
	}

	return result.String()
}
