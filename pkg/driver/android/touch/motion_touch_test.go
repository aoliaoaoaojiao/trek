package touch_test

import (
	"testing"
	"trek/internal/engine/decision/shared/types"
	"trek/pkg/driver/android/adb"
	"trek/pkg/driver/android/touch"
	"trek/pkg/driver/android/utils"
)

var (
	device *adb.Device
)

func init() {
	device, _ = utils.GetDevice("")
}

func TestMotionTouch_Pinch(t *testing.T) {
	motionTouch, err := touch.NewMotionTouch(device)
	if err != nil {
		t.Fatalf("NewMotionTouch failed: %v", err)
	}

	centerPoint := types.Point{
		X: 0.5,
		Y: 0.5,
	}

	// 鏀惧ぇ
	motionTouch.Pinch(centerPoint, 0.3, 0.9, 3*1000)

	// 缂╁皬
	motionTouch.Pinch(centerPoint, 0.9, 0.3, 3*1000)

	centerPoint2 := types.Point{
		X: 500,
		Y: 500,
	}

	// 鏀惧ぇ
	motionTouch.Pinch(centerPoint2, 300, 900, 3*1000)

	// 缂╁皬
	motionTouch.Pinch(centerPoint2, 900, 300, 3*1000)

}

func TestMotionTouch_Swipe(t *testing.T) {
	motionTouch, err := touch.NewMotionTouch(device)
	if err != nil {
		t.Fatalf("NewMotionTouch failed: %v", err)
	}
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
	motionTouch, err := touch.NewMotionTouch(device)
	if err != nil {
		t.Fatalf("NewMotionTouch failed: %v", err)
	}
	motionTouch.Click(types.Point{
		X: 0.147,
		Y: 0.861,
	})
}

func TestMotionTouch_longClick(t *testing.T) {
	motionTouch, err := touch.NewMotionTouch(device)
	if err != nil {
		t.Fatalf("NewMotionTouch failed: %v", err)
	}
	motionTouch.LongClick(types.Point{
		X: 0.147,
		Y: 0.861,
	}, 8000)
}
