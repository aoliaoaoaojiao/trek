package touch_test

import (
	"testing"
	"trek/internal/engine/core/primitives"
	"trek/pkg/driver/android/touch"
)

func TestADBTouch_Pinch(t *testing.T) {
	adbTouch := touch.NewADBTouch(device)
	centerPoint := primitives.Point{
		X: 500,
		Y: 500,
	}
	adbTouch.Pinch(centerPoint, 100, 500, 2*1000)
}
