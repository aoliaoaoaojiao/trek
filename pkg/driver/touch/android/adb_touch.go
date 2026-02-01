package android

import (
	"fmt"
	types2 "trek/internal/core/types"
	"trek/pkg/driver"
	"trek/pkg/gadb"
)

var _ driver.ITouch = (*ADBTouch)(nil)

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
		return driver.NoADBDeviceErr
	}
	_, err := a.device.RunShellCommand("input", "tap", fmt.Sprintf("%d %d", int(point.X), int(point.Y)))
	return err
}

func (a *ADBTouch) LongClick(point types2.Point, duration int64) error {
	if a.device == nil {
		return driver.NoADBDeviceErr
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
	// 1. 基础校验：设备非空
	if a.device == nil {
		return driver.NoADBDeviceErr
	}

	// 2. 新增：步骤数和时长的合法性校验（核心，避免除零/无效滑动）
	if step <= 0 {
		return fmt.Errorf("step must be greater than 0, current: %d", step)
	}
	if duration <= 0 {
		return fmt.Errorf("duration must be greater than 0, current: %d", duration)
	}
	// 步骤数为1时，直接执行原有一次性滑动逻辑，兼容旧调用
	if step == 1 {
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

	// 3. 计算核心参数：单步偏移量、单步时长
	// 3.1 转换起点/终点为int64（避免浮点精度问题，ADB坐标为整数）
	startX, startY := int64(startPoint.X), int64(startPoint.Y)
	endX, endY := int64(endPoint.X), int64(endPoint.Y)
	// 3.2 计算X/Y方向总偏移量
	totalDX := endX - startX
	totalDY := endY - startY
	// 3.3 计算单步X/Y偏移量（平均拆分，整数除法，最后一步自动补全剩余距离）
	stepDX := totalDX / step
	stepDY := totalDY / step
	// 3.4 计算单步时长（总时长平均拆分，毫秒）
	stepDuration := duration / step

	// 4. 循环执行分步滑动：当前坐标从起点开始，逐步更新
	currentX, currentY := startX, startY
	for i := int64(0); i < step; i++ {
		// 计算当前步的目标坐标：最后一步直接到终点（避免整数除法的余数偏差）
		var targetX, targetY int64
		if i == step-1 {
			targetX, targetY = endX, endY // 最后一步补全剩余距离，保证精准到终点
		} else {
			targetX = currentX + stepDX
			targetY = currentY + stepDY
		}

		// 执行当前步的短距离滑动（ADB核心命令）
		_, err := a.device.RunShellCommand(
			"input", "swipe",
			fmt.Sprintf("%d %d %d %d %d",
				int(currentX), int(currentY),
				int(targetX), int(targetY),
				int(stepDuration),
			),
		)
		// 某一步失败则立即返回错误，终止后续滑动
		if err != nil {
			return fmt.Errorf("step %d swipe failed: %w", i+1, err)
		}

		// 更新当前坐标为本次步的终点，作为下一步的起点
		currentX, currentY = targetX, targetY
	}

	// 所有步骤执行完成，返回nil
	return nil
}

func (a *ADBTouch) Pinch(centerPoint types2.Point, startDistance float64, endDistance float64, duration int64) error {
	panic("adb not pinchable")
}

func (a *ADBTouch) TouchEvent(touchList ...driver.TouchEvent) error {
	panic("adb not custom touch event")
}
