package touch

import (
	"errors"
	"fmt"
	types2 "trek/internal/engine/core/types"
	"trek/log"
	"trek/pkg/driver/android/gadb"
	"trek/pkg/driver/common"
)

var _ common.ITouch = (*ADBTouch)(nil)

func NewADBTouch(device *gadb.Device) *ADBTouch {
	return &ADBTouch{
		device: device,
	}
}

type ADBTouch struct {
	device *gadb.Device
}

func (a *ADBTouch) Close() error {
	a.device = nil
	return nil
}

func (a *ADBTouch) Click(point types2.Point) error {
	if a.device == nil {
		return common.NoADBDeviceErr
	}
	_, err := a.device.RunShellCommand("input", "tap", fmt.Sprintf("%d %d", int(point.X), int(point.Y)))
	return err
}

func (a *ADBTouch) LongClick(point types2.Point, duration int64) error {
	if a.device == nil {
		return common.NoADBDeviceErr
	}

	_, err := a.device.RunShellCommand(
		"input", "swipe",
		fmt.Sprintf("%d %d %d %d %d",
			int(point.X), int(point.Y),
			int(point.X), int(point.Y),
			duration,
		),
	)

	return err

}

func (a *ADBTouch) Swipe(startPoint types2.Point, endPoint types2.Point, step int64, duration int64) error {
	// 基础校验：设备非空
	if a.device == nil {
		return common.NoADBDeviceErr
	}

	log.Warn("adb touch settings do not support steps")
	if duration <= 0 {
		return fmt.Errorf("duration must be greater than 0, current: %d", duration)
	}

	_, err := a.device.RunShellCommand(
		"input", "swipe",
		fmt.Sprintf("%d %d %d %d %d",
			int(startPoint.X), int(startPoint.Y),
			int(endPoint.X), int(endPoint.Y),
			int(duration),
		),
	)
	return err
}

func (a *ADBTouch) Pinch(centerPoint types2.Point, startDistance float64, endDistance float64, duration int64) error {
	return errors.New("adb not pinchable")
}

func (a *ADBTouch) TouchEvent(touchList ...common.TouchEvent) error {
	return errors.New("adb not custom touch event")
}
