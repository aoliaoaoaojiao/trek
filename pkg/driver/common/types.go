// Package common defines shared driver interfaces.
package common

import (
	"context"

	"trek/internal/engine/decision/shared/types"
)

type TouchEventType string

const (
	DOWN_TOUCH_EVENT TouchEventType = "down"
	UP_TOUCH_EVENT   TouchEventType = "up"
	MOVE_TOUCH_EVENT TouchEventType = "move"
)

type TouchEvent struct {
	FingerID int64
	Type     TouchEventType
	WaitTime int64
	types.Point
}

type ITouch interface {
	Click(point types.Point) error
	LongClick(point types.Point, duration int64) error
	Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error
	Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error
	TouchEvent(touchList ...TouchEvent) error
	Close() error
}

type IPageSource interface {
	DumpPageSource() (string, error)
	Close() error
}

type IScreenCapture interface {
	Screenshot(ctx context.Context) ([]byte, error)
	SaveScreenshot(path string) error
	Record(path string) error
	StopRecording() error
	Close() error
}

type IAppControl interface {
	Back() error
	StartApp(packageName string) error
	RestartApp(packageName string, clean bool) error
	ActivateApp(packageName string) error
}

type ITextInput interface {
	InputText(text string, clear bool) error
}

type IHealthCheck interface {
	CheckCrash(packageName string) (bool, error)
	CheckANR(packageName string) (bool, error)
	ClearLogcat() error
}

type EnvironmentCheckResult struct {
	ADBReady        bool
	DeviceReady     bool
	PageSourceReady bool
	UIAReady        bool
	PageSourceType  string
	DeviceName      string
	Detail          string
}

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
