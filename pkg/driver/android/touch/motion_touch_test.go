package touch_test

import (
	"testing"
	"trek/internal/core/types"
	gadb2 "trek/pkg/driver/android/gadb"
	"trek/pkg/driver/android/tool"
	"trek/pkg/driver/android/touch"
)

var (
	device *gadb2.Device
)

func init() {
	device, _ = tool.GetDevice("")
}

func TestMotionTouch_Pinch(t *testing.T) {
	motionTouch := touch.NewMotionTouch(device)

	centerPoint := types.Point{
		X: 0.5,
		Y: 0.5,
	}

	// 放大
	motionTouch.Pinch(centerPoint, 0.3, 0.9, 3*1000)

	// 缩小
	motionTouch.Pinch(centerPoint, 0.9, 0.3, 3*1000)

	centerPoint2 := types.Point{
		X: 500,
		Y: 500,
	}

	// 放大
	motionTouch.Pinch(centerPoint2, 300, 900, 3*1000)

	// 缩小
	motionTouch.Pinch(centerPoint2, 900, 300, 3*1000)

}

func TestMotionTouch_Swipe(t *testing.T) {
	motionTouch := touch.NewMotionTouch(device)
	statPoint1 := types.Point{
		X: 0.1,
		Y: 0.5,
	}

	endPoint1 := types.Point{
		X: 0.9,
		Y: 0.5,
	}

	motionTouch.Swipe(statPoint1, endPoint1, 10, 2*1000)

	statPoint2 := types.Point{
		X: 100,
		Y: 700,
	}

	endPoint2 := types.Point{
		X: 100,
		Y: 200,
	}

	motionTouch.Swipe(statPoint2, endPoint2, 10, 2*1000)
}

func TestMotionTouch_Click(t *testing.T) {
	motionTouch := touch.NewMotionTouch(device)
	motionTouch.Click(types.Point{
		X: 200,
		Y: 700,
	})
}
