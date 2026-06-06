package annotation

import (
	"image"
	"image/color"
)

// BitmapFont5x7 包含数字 0-9 的预光栅化 5x7 像素字形。
// 参考 Midscene 的 box-select.ts 设计，每个数字编码为 7 行 x 5 列的二维数组。
type BitmapFont5x7 struct {
	glyphs [10][7]byte // [数字][行]，每字节低 5 位表示该行的 5 个像素
}

// DefaultFont5x7 返回嵌入的默认 5x7 数字字体数据。
func DefaultFont5x7() *BitmapFont5x7 {
	return &BitmapFont5x7{glyphs: digitGlyphs}
}

// DrawDigit 在 RGBA 图像上的指定位置绘制一个数字。
// scale: 缩放因子（2 = 每个位图像素变成 2x2 屏像素）。
// 返回绘制的数字在图像像素坐标中的边界框。
func (f *BitmapFont5x7) DrawDigit(img *image.RGBA, digit int, x, y int, scale int, col color.Color) image.Rectangle {
	if digit < 0 || digit > 9 {
		digit = 0
	}
	if scale <= 0 {
		scale = 2
	}

	bounds := img.Bounds()
	w := 5 * scale  // 字符宽度（像素）
	h := 7 * scale  // 字符高度（像素）

	for row := 0; row < 7; row++ {
		mask := f.glyphs[digit][row]
		for c := 0; c < 5; c++ {
			// 检查该列像素是否设置（从高位到低位遍历）
			if (mask>>(4-c))&1 == 1 {
				// 将 1 个位图像素映射到 scale×scale 屏像素块
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						px := x + c*scale + sx
						py := y + row*scale + sy
						if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
							img.Set(px, py, col)
						}
					}
				}
			}
		}
	}

	return image.Rect(x, y, x+w, y+h)
}

// DrawNumber 在 RGBA 图像上的指定位置绘制一个多位数。
// 字符间距 1 像素（缩放后）。
// 返回整体边界框。
func (f *BitmapFont5x7) DrawNumber(img *image.RGBA, number int, x, y int, scale int, col color.Color) image.Rectangle {
	if scale <= 0 {
		scale = 2
	}

	// 将数字转为字符串
	digits := itoa(number)

	cx := x
	for _, d := range digits {
		rect := f.DrawDigit(img, int(d-'0'), cx, y, scale, col)
		cx = rect.Max.X + 1*scale/2 // 一个字符宽度的间距（缩放后）
	}

	return image.Rect(x, y, cx, y+7*scale)
}

// TextWidth 返回指定数字串的渲染宽度（像素）。
func (f *BitmapFont5x7) TextWidth(number int, scale int) int {
	if scale <= 0 {
		scale = 2
	}
	s := itoa(number)
	return len(s)*(5*scale) + (len(s)-1)*(1*scale/2) + 1
}

// TextHeight 返回渲染高度（像素）。
func (f *BitmapFont5x7) TextHeight(scale int) int {
	if scale <= 0 {
		scale = 2
	}
	return 7 * scale
}

// itoa 将整数转为十进制数字切片。
func itoa(n int) []int {
	if n == 0 {
		return []int{0}
	}
	// 计算位数
	tmp := n
	digits := 0
	for tmp > 0 {
		digits++
		tmp /= 10
	}
	result := make([]int, digits)
	for i := digits - 1; i >= 0; i-- {
		result[i] = n % 10
		n /= 10
	}
	return result
}

// digitGlyphs 编码数字 0-9 的 5x7 位图。
// 每字节低 5 位：bit4=左, bit3, bit2, bit1, bit0=右。
var digitGlyphs = [10][7]byte{
	// 0
	{0b01110, 0b10001, 0b10011, 0b10101, 0b11001, 0b10001, 0b01110},
	// 1
	{0b00100, 0b01100, 0b00100, 0b00100, 0b00100, 0b00100, 0b11111},
	// 2
	{0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111},
	// 3
	{0b01110, 0b10001, 0b00001, 0b00110, 0b00001, 0b10001, 0b01110},
	// 4
	{0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	// 5
	{0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110},
	// 6
	{0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	// 7
	{0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	// 8
	{0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	// 9
	{0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
}
