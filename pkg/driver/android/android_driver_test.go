package android_test

import (
	"path/filepath"
	"testing"
	"time"
	"trek/internal/engine/core/types"
	"trek/pkg/driver/android"
	"trek/pkg/driver/common"

	"github.com/stretchr/testify/assert"
)

func init() {

	rootPath, _ := common.RepoRootFromCurrentFile()

	common.SetPluginDirPath(filepath.Clean(filepath.Join(filepath.Dir(rootPath), "..", "..")))
}

func TestGetDevice(t *testing.T) {
	driver, err := android.NewAndroidDriver()
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, driver.Name())
}

func TestBaseTouchEvent(t *testing.T) {
	tests := []struct {
		name      string
		touchType android.TouchType
	}{
		{"ADB", android.TouchTypeADB},
		{"Motion", android.TouchTypeMotion},
		{"UIA", android.TouchTypeUIA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := android.NewAndroidDriver(
				android.WithTouch(tt.touchType),
			)
			if err != nil {
				t.Fatal(err)
			}
			assert.NotEmpty(t, driver.Name())

			err = driver.Click(types.Point{
				X: 500,
				Y: 500,
			})
			assert.NoError(t, err)

			err = driver.LongClick(types.Point{
				X: 600,
				Y: 600,
			}, 1000)
			assert.NoError(t, err)

			err = driver.Swipe(
				types.Point{
					X: 100, Y: 100,
				},
				types.Point{
					X: 600, Y: 600,
				},
				10, 1000)
			assert.NoError(t, err)
		})
	}
}

func TestPinch(t *testing.T) {
	tests := []struct {
		name      string
		touchType android.TouchType
	}{
		{"Motion", android.TouchTypeMotion},
		{"UIA", android.TouchTypeUIA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, err := android.NewAndroidDriver(
				android.WithTouch(tt.touchType),
			)
			if err != nil {
				t.Fatal(err)
			}
			assert.NotEmpty(t, driver.Name())

			err = driver.Pinch(
				types.Point{
					X: 500,
					Y: 500},
				150,
				500,
				2*1000,
			)
			assert.NoError(t, err)
			time.Sleep(3 * time.Second)
		})
	}
}
