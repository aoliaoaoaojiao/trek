package android_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"trek/internal/engine/decision/shared/types"
	"trek/pkg/driver/android"
	"trek/pkg/driver/common"
	"trek/pkg/driver/common/page/poco"

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
			defer driver.Close()
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

func TestScreenshot(t *testing.T) {
	driver, err := android.NewAndroidDriver()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	assert.NotEmpty(t, driver.Name())

	data, err := driver.Screenshot()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	screenshotPath := filepath.Join("screen.png")
	fmt.Println(screenshotPath)
	err = driver.SaveScreenshot(screenshotPath)
	assert.NoError(t, err)

	fileInfo, err := os.Stat(screenshotPath)
	assert.NoError(t, err)
	assert.Greater(t, fileInfo.Size(), int64(0))
}

func TestRecord(t *testing.T) {
	driver, err := android.NewAndroidDriver()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	assert.NotEmpty(t, driver.Name())

	recordPath := filepath.Join("screen.mp4")
	err = driver.Record(recordPath)
	if err != nil {
		if strings.Contains(err.Error(), "Unknown server option: size_info") {
			t.Skipf("当前 scrcpy 服务端不支持录像参数，跳过录像测试: %v", err)
		}
		t.Fatal(err)
	}

	time.Sleep(3 * time.Second)

	err = driver.StopRecording()
	assert.NoError(t, err)

	fileInfo, err := os.Stat(recordPath)
	assert.NoError(t, err)
	assert.Greater(t, fileInfo.Size(), int64(0))
}

func TestGetUIAPageSource(t *testing.T) {
	driver, err := android.NewAndroidDriver()
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	assert.NotEmpty(t, driver.Name())

	pageSource := driver.GetPageSource(string(android.PageTypeUIA))
	assert.NotNil(t, pageSource)

	source, err := pageSource.DumpPageSource()
	assert.NoError(t, err)
	assert.NotEmpty(t, source)
}

func TestGetUnityPageSource(t *testing.T) {
	driver, err := android.NewAndroidDriver(
		android.WithPoco(poco.Unity3d, poco.Unity3d.GetDefaultPort()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	assert.NotEmpty(t, driver.Name())

	pageSource := driver.GetPageSource(string(android.PageTypePoco))
	assert.NotNil(t, pageSource)

	source, err := pageSource.DumpPageSource()
	assert.NoError(t, err)
	assert.NotEmpty(t, source)
}

func TestGetCocos2dxJsPageSource(t *testing.T) {
	driver, err := android.NewAndroidDriver(
		android.WithPoco(poco.Cocos2dxJs, poco.Cocos2dxJs.GetDefaultPort()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	assert.NotEmpty(t, driver.Name())

	pageSource := driver.GetPageSource(string(android.PageTypePoco))
	assert.NotNil(t, pageSource)

	source, err := pageSource.DumpPageSource()
	assert.NoError(t, err)
	assert.NotEmpty(t, source)
}

func TestGetCocos2dxLuaPageSource(t *testing.T) {
	driver, err := android.NewAndroidDriver(
		android.WithPoco(poco.Cocos2dxLua, poco.Cocos2dxLua.GetDefaultPort()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	assert.NotEmpty(t, driver.Name())

	pageSource := driver.GetPageSource(string(android.PageTypePoco))
	assert.NotNil(t, pageSource)

	source, err := pageSource.DumpPageSource()
	fmt.Println(source)
	assert.NoError(t, err)
	assert.NotEmpty(t, source)
}
