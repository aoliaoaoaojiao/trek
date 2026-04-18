// Package common 定义设备驱动接口。
package common

import (
	"trek/internal/engine/core/types"
)

// TouchEventType 表示触摸事件类型。
type TouchEventType string

const (
	// DOWN_TOUCH_EVENT 手指按下事件。
	DOWN_TOUCH_EVENT TouchEventType = "down"
	// UP_TOUCH_EVENT 手指抬起事件。
	UP_TOUCH_EVENT TouchEventType = "up"
	// MOVE_TOUCH_EVENT 手指移动事件。
	MOVE_TOUCH_EVENT TouchEventType = "move"
)

// TouchEvent 描述一条触摸指令。
type TouchEvent struct {
	// FingerID 多指触控时的手指标识。
	FingerID int64
	// Type 触摸事件类型。
	Type TouchEventType
	// WaitTime 事件执行后的等待时间，单位毫秒。
	WaitTime int64
	types.Point
}

// ITouch 定义触控能力。
type ITouch interface {
	Click(point types.Point) error
	LongClick(point types.Point, duration int64) error
	Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error
	Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error
	TouchEvent(touchList ...TouchEvent) error
	Close() error
}

// IPageSource 定义页面源能力。
type IPageSource interface {
	DumpPageSource() (string, error)
	Close() error
}

// IScreenCapture 定义截图与录屏能力。
type IScreenCapture interface {
	Screenshot() ([]byte, error)
	SaveScreenshot(path string) error
	Record(path string) error
	StopRecording() error
	Close() error
}

// IAppControl 定义应用生命周期与导航控制能力。
type IAppControl interface {
	Back() error
	StartApp(packageName string) error
	RestartApp(packageName string, clean bool) error
	ActivateApp(packageName string) error
}

// ITextInput 定义文本输入能力。
type ITextInput interface {
	InputText(text string, clear bool) error
}

// IHealthCheck 定义 Crash/ANR 健康检测能力。
type IHealthCheck interface {
	CheckCrash(packageName string) (bool, error)
	CheckANR(packageName string) (bool, error)
	ClearLogcat() error
}

// EnvironmentCheckResult 是运行前环境检测结果。
type EnvironmentCheckResult struct {
	ADBReady        bool
	DeviceReady     bool
	PageSourceReady bool
	UIAReady        bool
	PageSourceType  string
	DeviceName      string
	Detail          string
}

// IDriver 定义统一设备驱动接口。
// 该接口强制要求驱动实现应用控制、输入与健康检测能力。
type IDriver interface {
	ITouch
	IScreenCapture
	IAppControl
	ITextInput
	IHealthCheck

	GetPageSource(pageSourceType string) IPageSource
	Name() string
	GetInfo() map[string]interface{}
	CheckEnvironment(pageSourceType string) (*EnvironmentCheckResult, error)
}
