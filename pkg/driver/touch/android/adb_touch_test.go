package android_test

import (
	"testing"
	"trek/internal/core/types"
	touch "trek/pkg/driver/touch/android"
)

func TestADBTouch_Pinch(t *testing.T) {
	adbTouch := touch.NewADBTouch(device)
	centerPoint := types.Point{
		X: 500,
		Y: 500,
	}
	adbTouch.Pinch(centerPoint, 100, 500, 2*1000)
}
