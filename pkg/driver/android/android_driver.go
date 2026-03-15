package android

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/core/types"
	"trek/log"
	"trek/pkg/driver/android/gadb"
	"trek/pkg/driver/android/page"
	"trek/pkg/driver/android/page/poco"
	"trek/pkg/driver/android/screen"
	"trek/pkg/driver/android/touch"
	"trek/pkg/driver/android/uia"
	"trek/pkg/driver/android/utils"
	"trek/pkg/driver/common"
)

var _ common.IDriver = (*AndroidDriver)(nil)

type TouchType string

type PageType string

const (
	TouchTypeADB    TouchType = "adb"
	TouchTypeMotion TouchType = "motion"
	TouchTypeUIA    TouchType = "uia"

	PageTypeUIA  PageType = "uia"
	PageTypePoco PageType = "poco"

	uiaServerPort         = 6790
	uiaReadyLogMarker     = "io.appium.uiautomator2.server.test.AppiumUiAutomator2Server:"
	uiaStartupWaitTimeout = 30 * time.Second
	uiaStartupReadyDelay  = 2 * time.Second
	uiaInstrumentationCmd = "am instrument -w io.appium.uiautomator2.server.test/androidx.test.runner.AndroidJUnitRunner -e DISABLE_SUPPRESS_ACCESSIBILITY_SERVICES true -e disableAnalytics true"
)

type AndroidDriver struct {
	device        *gadb.Device
	touch         common.ITouch
	screenCapture common.IScreenCapture
	pageSources   map[PageType]common.IPageSource
	mu            sync.RWMutex
	uiaClient     *uia.UiaClient
	isUIATouch    bool
}

type AndroidDriverOption func(*AndroidDriver)

func WithPoco(engine poco.Engine, port int) AndroidDriverOption {
	return func(d *AndroidDriver) {
		if port > 0 && engine != "" {
			if ps, err := page.NewPocoPageSource(engine, port); err == nil {
				d.pageSources[PageTypePoco] = ps
			}
		}
	}
}

func WithTouch(touchType TouchType, opts ...interface{}) AndroidDriverOption {
	return func(d *AndroidDriver) {
		switch touchType {
		case TouchTypeADB:
			d.touch = touch.NewADBTouch(d.device)
		case TouchTypeMotion:
			d.touch = touch.NewMotionTouch(d.device)
		case TouchTypeUIA:
			d.isUIATouch = true
		}
	}
}

func NewAndroidDriver(options ...AndroidDriverOption) (*AndroidDriver, error) {
	return NewAndroidDriverWith("", options...)
}

func NewAndroidDriverWith(deviceSerial string, opts ...AndroidDriverOption) (*AndroidDriver, error) {
	if err := utils.EnsureADBServer(); err != nil {
		return nil, fmt.Errorf("uia driver: adb environment unavailable: %v", err)
	}

	device, err := utils.GetDevice(deviceSerial)

	if err != nil {
		return nil, err
	}

	androidDriver := &AndroidDriver{
		device:        device,
		touch:         touch.NewADBTouch(device),
		screenCapture: screen.NewScreenCapture(device),
		pageSources:   make(map[PageType]common.IPageSource),
	}

	err = androidDriver.initUIA()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		opt(androidDriver)
	}

	if androidDriver.isUIATouch {
		if androidDriver.uiaClient != nil {
			androidDriver.touch = touch.NewUIATouch(androidDriver.uiaClient)
		} else {
			log.Warn("UIA Touch Mode is not available, ADB Touch Mode will be used")
		}
	}

	return androidDriver, nil
}

// Click 执行点击操作
// point: 点击位置的坐标点
// 返回错误信息，如果操作成功则返回nil
func (a *AndroidDriver) Click(point types.Point) error {
	return a.touch.Click(point)
}

// LongClick 执行长按操作
// point: 长按位置的坐标点
// duration: 长按持续时间，单位为毫秒
// 返回错误信息，如果操作成功则返回nil
func (a *AndroidDriver) LongClick(point types.Point, duration int64) error {
	return a.touch.LongClick(point, duration)
}

// Swipe 执行滑动操作
// startPoint: 滑动起始位置
// endPoint: 滑动结束位置
// step: 滑动步数，数值越大滑动越平滑
// duration: 滑动持续时间，单位为毫秒
// 返回错误信息，如果操作成功则返回nil
func (a *AndroidDriver) Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error {
	return a.touch.Swipe(startPoint, endPoint, step, duration)
}

// Pinch 执行缩放手势操作
// centerPoint: 缩放手势的中心点
// startDistance: 起始距离（两指间的距离）
// endDistance: 结束距离（两指间的距离）
// duration: 缩放持续时间，单位为毫秒
// 返回错误信息，如果操作成功则返回nil
// 当endDistance > startDistance时为放大，endDistance < startDistance时为缩小
func (a *AndroidDriver) Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error {
	return a.touch.Pinch(centerPoint, startDistance, endDistance, duration)
}

func (a *AndroidDriver) TouchEvent(touchList ...common.TouchEvent) error {
	return a.touch.TouchEvent(touchList...)
}

func (a *AndroidDriver) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.touch != nil {
		a.touch.Close()
	}
	if a.screenCapture != nil {
		a.screenCapture.Close()
	}
	for _, ps := range a.pageSources {
		ps.Close()
	}
	a.pageSources = make(map[PageType]common.IPageSource)
	return nil
}

func (a *AndroidDriver) Screenshot() ([]byte, error) {
	return a.screenCapture.Screenshot()
}

func (a *AndroidDriver) SaveScreenshot(path string) error {
	return a.screenCapture.SaveScreenshot(path)
}

func (a *AndroidDriver) Record(path string) error {
	return a.screenCapture.Record(path)
}

func (a *AndroidDriver) StopRecording() error {
	return a.screenCapture.StopRecording()
}

func (a *AndroidDriver) GetPageSource(pageSourceType string) common.IPageSource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if ps, ok := a.pageSources[PageType(pageSourceType)]; ok {
		return ps
	}
	return nil
}

func (a *AndroidDriver) GetUIAClient(remoteUrl string) (*uia.UiaClient, error) {
	client, err := uia.NewUiaClient(remoteUrl)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("创建 UIA 客户端失败")
	}
	return client, nil
}

func (a *AndroidDriver) NewUIAPageSource(remoteUrl string) (common.IPageSource, error) {
	client, err := a.GetUIAClient(remoteUrl)
	if err != nil {
		return nil, err
	}

	uiPageSource := page.NewUIAPageSource(client)

	a.mu.Lock()
	a.pageSources[PageTypePoco] = uiPageSource
	a.mu.Unlock()

	return uiPageSource, nil
}

func (a *AndroidDriver) NewPocoPageSource(engine poco.Engine, port int) (common.IPageSource, error) {
	pocoPageSource, err := page.NewPocoPageSource(engine, port)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.pageSources["poco"] = pocoPageSource
	a.mu.Unlock()

	return pocoPageSource, nil
}

func (a *AndroidDriver) Name() string {
	if a.device != nil {
		return a.device.Serial()
	}
	return ""
}

func (a *AndroidDriver) GetInfo() map[string]interface{} {
	info := make(map[string]interface{})

	if a.device != nil {
		info["device"] = a.device.Serial()
		info["model"] = a.device.Model()
		info["product"] = a.device.Product()
	}

	return info
}

func (a *AndroidDriver) checkUiaApkVersion() (bool, error) {
	output, err := a.device.RunShellCommand("dumpsys package", uiaServerPackage)
	if err != nil {
		return false, err
	}
	return strings.Contains(output, "versionName="+uiaApkVersion), nil
}

func (a *AndroidDriver) initUIA() error {
	checkUIARes, err := a.checkUiaApkVersion()
	if err != nil {
		return err
	}

	if !checkUIARes {
		utils.UninstallPackage(a.device.Serial(), uiaServerPackage, false)
		utils.UninstallPackage(a.device.Serial(), uiaServerTestPackage, false)

		pluginsDir, err := resolveUIAPluginsDir()
		if err != nil {
			return err
		}

		serverAPKPath := filepath.Join(pluginsDir, uiaServerAPK)
		if err := utils.InstallAPK(a.device.Serial(), serverAPKPath, true); err != nil {
			return fmt.Errorf("install %s failed: %w", serverAPKPath, err)
		}

		serverTestAPKPath := filepath.Join(pluginsDir, uiaServerTestAPK)
		if err := utils.InstallAPK(a.device.Serial(), serverTestAPKPath, true); err != nil {
			return fmt.Errorf("install %s failed: %w", serverTestAPKPath, err)
		}

		commands := []string{
			"appops set " + uiaServerPackage + " RUN_IN_BACKGROUND allow",
			"appops set " + uiaServerTestPackage + " RUN_IN_BACKGROUND allow",
			"dumpsys deviceidle whitelist +" + uiaServerPackage,
			"dumpsys deviceidle whitelist +" + uiaServerTestPackage,
		}

		for _, command := range commands {
			if _, err := a.device.RunShellCommand(command); err != nil {
				return fmt.Errorf("execute command failed: %s: %w", command, err)
			}
		}
	}

	uiaPort := common.GetRandomPort()

	if err := a.startUIAServer(uiaPort); err != nil {
		return err
	}

	a.uiaClient, err = uia.NewUiaClient(fmt.Sprintf("http://localhost:%d", uiaPort))

	if err != nil {
		return err
	}

	a.pageSources[PageTypeUIA] = page.NewUIAPageSource(a.uiaClient)

	return nil
}

func (a *AndroidDriver) startUIAServer(uiaPort int) error {
	if err := a.device.FrowardTcp(uiaPort, uiaServerPort); err != nil {
		return fmt.Errorf("forward uia port failed: %w", err)
	}
	//
	//readyCh := make(chan struct{}, 1)
	//errCh := make(chan error, 1)

	//go func() {
	//	defer func() {
	//		if err := a.device.ForwardKill(uiaPort); err != nil {
	//			log.Warnf("remove UIA forward failed: %v", err)
	//		}
	//	}()
	//
	//	conn, err := a.device.RunShellLoopCommandSock(uiaInstrumentationCmd)
	//	if err != nil {
	//		select {
	//		case errCh <- fmt.Errorf("start uia instrumentation failed: %w", err):
	//		default:
	//		}
	//		return
	//	}
	//	defer conn.Close()
	//
	//	reader := bufio.NewReader(conn)
	//	readySent := false
	//	for {
	//		line, err := reader.ReadString('\n')
	//		if line != "" {
	//			trimmed := strings.TrimSpace(line)
	//			if trimmed != "" {
	//				log.Info(trimmed)
	//				if !readySent && strings.Contains(trimmed, uiaReadyLogMarker) {
	//					time.Sleep(uiaStartupReadyDelay)
	//					readySent = true
	//					select {
	//					case readyCh <- struct{}{}:
	//					default:
	//					}
	//				}
	//			}
	//		}
	//
	//		if err != nil {
	//			if !readySent && err != io.EOF {
	//				select {
	//				case errCh <- fmt.Errorf("uia instrumentation output failed: %w", err):
	//				default:
	//				}
	//			}
	//			return
	//		}
	//	}
	//}()
	//
	//select {
	//case <-readyCh:
	//	return nil
	//case err := <-errCh:
	//	return err
	//case <-time.After(uiaStartupWaitTimeout):
	//	return fmt.Errorf("wait for uia server startup timeout after %s", uiaStartupWaitTimeout)
	//}
	return nil
}

func resolveUIAPluginsDir() (string, error) {
	projectRoot, err := common.GetPluginDirPath()
	if err != nil {
		return "", fmt.Errorf("resolve repo root failed: %w", err)
	}

	return filepath.Join(projectRoot, "plugins", "uia"), nil
}
