package touch

import (
	"context"
	"errors"
	"fmt"
	"trek/internal/engine/decision/shared/types"
	"trek/logger"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/common"
)

var _ common.ITouch = (*ADBTouch)(nil)

func NewADBTouch(device *adb.Device) *ADBTouch {
	return &ADBTouch{
		device: device,
	}
}

type ADBTouch struct {
	device *adb.Device
}

func (a *ADBTouch) Close() error {
	a.device = nil
	return nil
}

func (a *ADBTouch) Click(point types.Point) error {
	if a.device == nil {
		return common.NoADBDeviceErr
	}
	_, err := a.device.RunShellCommand(context.Background(), "input", "tap", fmt.Sprintf("%d %d", int(point.X), int(point.Y)))
	return err
}

func (a *ADBTouch) LongClick(point types.Point, duration int64) error {
	if a.device == nil {
		return common.NoADBDeviceErr
	}

	_, err := a.device.RunShellCommand(context.Background(),
		"input", "swipe",
		fmt.Sprintf("%d %d %d %d %d",
			int(point.X), int(point.Y),
			int(point.X), int(point.Y),
			duration,
		),
	)

	return err

}

func (a *ADBTouch) Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error {
	// 基础校验：设备非空
	if a.device == nil {
		return common.NoADBDeviceErr
	}

	logger.Warn("adb touch settings do not support steps")
	if duration <= 0 {
		return fmt.Errorf("duration must be greater than 0, current: %d", duration)
	}

	_, err := a.device.RunShellCommand(context.Background(),
		"input", "swipe",
		fmt.Sprintf("%d %d %d %d %d",
			int(startPoint.X), int(startPoint.Y),
			int(endPoint.X), int(endPoint.Y),
			int(duration),
		),
	)
	return err
}

func (a *ADBTouch) Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error {
	return errors.New("adb not pinchable")
}

func (a *ADBTouch) TouchEvent(touchList ...common.TouchEvent) error {
	return errors.New("adb not custom touch event")
}
