// Package coord 提供三空间坐标转换工具：物理像素、逻辑(CSS)像素、截图像素。
// 参考 Midscene 的坐标变换管线（commonContextParser + adaptBboxToRect）。
//
// 三种空间的关系：
//   - 截图像素：截图图像的实际像素尺寸
//   - 物理像素：设备屏幕的物理分辨率
//   - 逻辑像素：DPR 归一化后的 CSS 逻辑尺寸
//
// DPR = 截图像素宽度 / 逻辑像素宽度
package coord

// No external imports needed - DPRInfo is self-contained.

// DPRInfo 保存设备像素比和相关尺寸信息。
type DPRInfo struct {
	ScreenshotWidth  int // 截图图像宽度（像素）
	ScreenshotHeight int // 截图图像高度（像素）
	LogicalWidth     int // 逻辑/CSS 宽度
	LogicalHeight    int // 逻辑/CSS 高度
	DeviceWidth      int // 设备物理宽度（像素，>= 截图宽度）
	DeviceHeight     int // 设备物理高度（像素，>= 截图高度）
}

// DPR 返回设备像素比（截图宽度 / 逻辑宽度）。
// 当逻辑宽度为 0 时返回 1.0（单位映射）。
func (d DPRInfo) DPR() float64 {
	if d.LogicalWidth <= 0 {
		return 1.0
	}
	return float64(d.ScreenshotWidth) / float64(d.LogicalWidth)
}

// ScreenshotToLogicalRatio 返回截图到逻辑空间的缩放比。
// 当截图被缩小（VLM 优化）后，此比率用于将缩小截图坐标映射回逻辑空间。
// ratio = DPR / shrinkFactor （当截图被缩放时）
// 未缩小时等价于 DPR。
func (d DPRInfo) ScreenshotToLogicalRatio(shrinkFactor float64) float64 {
	if shrinkFactor <= 0 {
		shrinkFactor = 1
	}
	return d.DPR() / shrinkFactor
}

// Valid 检查 DPRInfo 是否包含有效的尺寸信息。
func (d DPRInfo) Valid() bool {
	return d.ScreenshotWidth > 0 && d.ScreenshotHeight > 0
}
