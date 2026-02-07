// Package driver 提供设备驱动接口定义
// 该包定义了与Android设备进行交互的标准接口，包括点击、滑动、触摸等操作
package driver

import "trek/internal/core/types"

// TouchEventType 触摸事件类型定义
type TouchEventType string

const (
	// DOWN_TOUCH_EVENT 手指按下事件
	DOWN_TOUCH_EVENT TouchEventType = "down"
	// UP_TOUCH_EVENT 手指抬起事件
	UP_TOUCH_EVENT TouchEventType = "up"
	// MOVE_TOUCH_EVENT 手指移动事件
	MOVE_TOUCH_EVENT TouchEventType = "move"
)

// TouchEvent 触摸事件结构体
// 用于描述一个完整的触摸动作，包括手指ID、事件类型和等待时间
type TouchEvent struct {
	// FingerID 手指标识符，用于多指触摸场景
	FingerID int64
	// Type 触摸事件类型
	Type TouchEventType
	// WaitTime 等待时间，单位为毫秒
	WaitTime int64
	types.Point
}

type ITouch interface {
	// Click 执行点击操作
	// point: 点击位置的坐标点
	// 返回错误信息，如果操作成功则返回nil
	Click(point types.Point) error

	// LongClick 执行长按操作
	// point: 长按位置的坐标点
	// duration: 长按持续时间，单位为毫秒
	// 返回错误信息，如果操作成功则返回nil
	LongClick(point types.Point, duration int64) error

	// Swipe 执行滑动操作
	// startPoint: 滑动起始位置
	// endPoint: 滑动结束位置
	// step: 滑动步数，数值越大滑动越平滑
	// duration: 滑动持续时间，单位为毫秒
	// 返回错误信息，如果操作成功则返回nil
	Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error

	// Pinch 执行缩放手势操作
	// centerPoint: 缩放手势的中心点
	// startDistance: 起始距离（两指间的距离）
	// endDistance: 结束距离（两指间的距离）
	// duration: 缩放持续时间，单位为毫秒
	// 返回错误信息，如果操作成功则返回nil
	// 当endDistance > startDistance时为放大，endDistance < startDistance时为缩小
	Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error

	// TouchEvent 执行复杂的触摸事件序列
	// touchList: 触摸事件列表，可以包含多个触摸动作
	// 返回错误信息，如果操作成功则返回nil
	TouchEvent(touchList ...TouchEvent) error

	Close() error
}

type IPageSource interface {
	// DumpPageSource 获取完整的页面信息
	// 以字符串形式返回完整页面的信息
	DumpPageSource() (string, error)

	Close() error
}

type IScreenshot interface {
	// Screenshot 截图
	// 返回当前的界面截图
	Screenshot() ([]byte, error)
	// SaveScreenshot 保存截图
	SaveScreenshot(path string) error
	// Record 录屏
	Record(path string) ([]byte, error)
	// StopRecording 停止录屏
	StopRecording() error

	Close() error
}

// IDriver 设备驱动接口
type IDriver interface {
	ITouch
	IScreenshot

	RegisterPageSource(name string, source IPageSource)
	GetPageSource(name string) IPageSource

	// Name 获取设备名称
	// 返回当前连接设备的标识名称
	Name() string

	// GetInfo 获取设备信息
	// 返回设备的详细信息，包括型号、版本、分辨率等
	GetInfo() map[string]interface{}
}
