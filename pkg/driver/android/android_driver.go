package android

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/core/types"
	"trek/logger"
	"trek/pkg/driver/android/gadb"
	"trek/pkg/driver/android/page"
	"trek/pkg/driver/android/screen"
	"trek/pkg/driver/android/touch"
	"trek/pkg/driver/android/uia"
	"trek/pkg/driver/android/utils"
	"trek/pkg/driver/common"
	"trek/pkg/driver/common/page/poco"
)

var (
	_ common.IDriver      = (*AndroidDriver)(nil)
	_ common.IAppControl  = (*AndroidDriver)(nil)
	_ common.ITextInput   = (*AndroidDriver)(nil)
	_ common.IHealthCheck = (*AndroidDriver)(nil)
)

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
	uiaServerConn net.Conn
	uiaPort       int
	isUIATouch    bool

	pocoPort        int
	frowardPocoPort int
	pocoEngine      poco.Engine
}

type AndroidDriverOption func(*AndroidDriver)

func WithPoco(engine poco.Engine, pocoPort int) AndroidDriverOption {
	return func(d *AndroidDriver) {
		if pocoPort > 0 && engine != "" {
			d.pocoEngine = engine
			d.pocoPort = pocoPort
		}
	}
}

func WithTouch(touchType TouchType) AndroidDriverOption {
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
	logger.Infof("Starting AndroidDriver initialization, deviceSerial=%s", deviceSerial)
	if err := utils.EnsureADBServer(); err != nil {
		return nil, fmt.Errorf("uia driver: adb environment unavailable: %v", err)
	}
	logger.Info("ADB server check passed")

	device, err := utils.GetDevice(deviceSerial)
	if err != nil {
		return nil, err
	}
	logger.Infof("Selected device, serial=%s model=%s product=%s", device.Serial(), device.Model(), device.Product())

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
	logger.Infof("UIA initialization completed, serial=%s", device.Serial())

	for _, opt := range opts {
		opt(androidDriver)
	}
	logger.Infof("Driver options applied, isUIATouch=%t pocoPort=%d pocoEngine=%s", androidDriver.isUIATouch, androidDriver.pocoPort, androidDriver.pocoEngine)

	if androidDriver.pocoPort > 0 {
		logger.Infof("Starting Poco page source initialization, remotePort=%d engine=%s", androidDriver.pocoPort, androidDriver.pocoEngine)
		err = androidDriver.initPoco()
		if err != nil {
			return nil, err
		}
		logger.Infof("Poco page source initialization completed, localPort=%d remotePort=%d", androidDriver.frowardPocoPort, androidDriver.pocoPort)
	}

	if androidDriver.isUIATouch {
		if androidDriver.uiaClient != nil {
			androidDriver.touch = touch.NewUIATouch(androidDriver.uiaClient)
			logger.Info("Switched to UIA touch mode")
		} else {
			logger.Warn("UIA Touch Mode is not available, ADB Touch Mode will be used")
		}
	}

	logger.Infof("AndroidDriver initialization completed, serial=%s", device.Serial())
	return androidDriver, nil
}

func (a *AndroidDriver) Click(point types.Point) error {
	return a.touch.Click(point)
}

func (a *AndroidDriver) LongClick(point types.Point, duration int64) error {
	return a.touch.LongClick(point, duration)
}

func (a *AndroidDriver) Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error {
	return a.touch.Swipe(startPoint, endPoint, step, duration)
}

func (a *AndroidDriver) Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error {
	return a.touch.Pinch(centerPoint, startDistance, endDistance, duration)
}

func (a *AndroidDriver) TouchEvent(touchList ...common.TouchEvent) error {
	return a.touch.TouchEvent(touchList...)
}

// Back 执行系统返回键。
func (a *AndroidDriver) Back() error {
	if a.device == nil {
		return fmt.Errorf("device is nil")
	}
	_, err := a.device.RunShellCommand("input", "keyevent", "4")
	return err
}

// StartApp 启动指定包名应用。
func (a *AndroidDriver) StartApp(packageName string) error {
	if strings.TrimSpace(packageName) == "" {
		return fmt.Errorf("packageName is empty")
	}
	if a.device == nil {
		return fmt.Errorf("device is nil")
	}
	_, err := a.device.RunShellCommand("monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
	return err
}

// RestartApp 重启应用，clean=true 时先清理应用数据。
func (a *AndroidDriver) RestartApp(packageName string, clean bool) error {
	if strings.TrimSpace(packageName) == "" {
		return fmt.Errorf("packageName is empty")
	}
	if a.device == nil {
		return fmt.Errorf("device is nil")
	}
	if _, err := a.device.RunShellCommand("am", "force-stop", packageName); err != nil {
		return err
	}
	if clean {
		if _, err := a.device.RunShellCommand("pm", "clear", packageName); err != nil {
			return err
		}
	}
	return a.StartApp(packageName)
}

// ActivateApp 激活应用，当前行为与 StartApp 一致。
func (a *AndroidDriver) ActivateApp(packageName string) error {
	return a.StartApp(packageName)
}

// InputText 通过 UIA 会话向当前焦点输入文本。
func (a *AndroidDriver) InputText(text string, clear bool) error {
	if a.uiaClient == nil {
		return fmt.Errorf("uia client is nil")
	}
	return a.uiaClient.SendKeys(text, clear)
}

// ClearLogcat 清空 logcat 缓冲，避免历史日志干扰本轮检测。
func (a *AndroidDriver) ClearLogcat() error {
	if a.device == nil {
		return fmt.Errorf("device is nil")
	}
	_, err := a.device.RunShellCommand("logcat", "-c")
	return err
}

// CheckCrash 通过系统日志/状态检测是否出现 crash。
func (a *AndroidDriver) CheckCrash(packageName string) (bool, error) {
	if a.device == nil {
		return false, fmt.Errorf("device is nil")
	}

	logcatOut, err := a.device.RunShellCommand("logcat", "-d", "-t", "200")
	if err != nil {
		return false, err
	}
	logLower := strings.ToLower(logcatOut)
	pkgLower := strings.ToLower(strings.TrimSpace(packageName))
	if strings.Contains(logLower, "fatal exception") {
		if pkgLower == "" || strings.Contains(logLower, pkgLower) {
			return true, nil
		}
	}
	if pkgLower != "" && strings.Contains(logLower, "process "+pkgLower+" has died") {
		return true, nil
	}
	if strings.Contains(logLower, "am_crash") {
		if pkgLower == "" || strings.Contains(logLower, pkgLower) {
			return true, nil
		}
	}

	dumpsysOut, err := a.device.RunShellCommand("dumpsys", "activity")
	if err != nil {
		return false, err
	}
	dumpLower := strings.ToLower(dumpsysOut)
	if strings.Contains(dumpLower, "crash") {
		if pkgLower == "" || strings.Contains(dumpLower, pkgLower) {
			return true, nil
		}
	}
	return false, nil
}

// CheckANR 通过系统日志/状态检测是否出现 ANR。
func (a *AndroidDriver) CheckANR(packageName string) (bool, error) {
	if a.device == nil {
		return false, fmt.Errorf("device is nil")
	}

	logcatOut, err := a.device.RunShellCommand("logcat", "-d", "-t", "200")
	if err != nil {
		return false, err
	}
	logLower := strings.ToLower(logcatOut)
	pkgLower := strings.ToLower(strings.TrimSpace(packageName))
	if strings.Contains(logLower, "am_anr") {
		if pkgLower == "" || strings.Contains(logLower, pkgLower) {
			return true, nil
		}
	}
	if strings.Contains(logLower, "anr in ") {
		if pkgLower == "" || strings.Contains(logLower, pkgLower) {
			return true, nil
		}
	}
	if strings.Contains(logLower, "isn't responding") || strings.Contains(logLower, "not responding") {
		if pkgLower == "" || strings.Contains(logLower, pkgLower) {
			return true, nil
		}
	}

	dumpsysOut, err := a.device.RunShellCommand("dumpsys", "activity", "processes")
	if err != nil {
		return false, err
	}
	dumpLower := strings.ToLower(dumpsysOut)
	if strings.Contains(dumpLower, "notresponding=true") {
		if pkgLower == "" || strings.Contains(dumpLower, pkgLower) {
			return true, nil
		}
	}
	return false, nil
}

func (a *AndroidDriver) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	logger.Infof("Starting AndroidDriver shutdown, serial=%s", a.Name())

	if a.touch != nil {
		if err := a.touch.Close(); err != nil {
			logger.Warnf("Failed to close touch resource: %v", err)
		} else {
			logger.Info("Touch resource closed")
		}
	}
	if a.screenCapture != nil {
		if err := a.screenCapture.Close(); err != nil {
			logger.Warnf("Failed to close screen capture resource: %v", err)
		} else {
			logger.Info("Screen capture resource closed")
		}
	}
	for pageType, ps := range a.pageSources {
		if err := ps.Close(); err != nil {
			logger.Warnf("Failed to close page source, type=%s err=%v", pageType, err)
		} else {
			logger.Infof("Page source closed, type=%s", pageType)
		}
	}
	if a.uiaServerConn != nil {
		_ = a.uiaServerConn.Close()
		a.uiaServerConn = nil
		logger.Info("UIA instrumentation connection closed")
	}
	if a.uiaPort > 0 && a.device != nil {
		if err := a.device.ForwardKill(a.uiaPort); err != nil {
			logger.Warnf("remove UIA forward failed: %v", err)
		} else {
			logger.Infof("UIA port forwarding removed, localPort=%d", a.uiaPort)
		}
		a.uiaPort = 0
	}
	if a.frowardPocoPort > 0 && a.device != nil {
		if err := a.device.ForwardKill(a.frowardPocoPort); err != nil {
			logger.Warnf("remove POCO forward failed: %v", err)
		} else {
			logger.Infof("Poco port forwarding removed, localPort=%d", a.frowardPocoPort)
		}
		a.frowardPocoPort = 0
	}

	a.pageSources = make(map[PageType]common.IPageSource)
	logger.Infof("AndroidDriver shutdown completed, serial=%s", a.Name())
	return nil
}

func (a *AndroidDriver) Screenshot() ([]byte, error) {
	logger.Debugf("Taking screenshot, serial=%s", a.Name())
	return a.screenCapture.Screenshot()
}

func (a *AndroidDriver) SaveScreenshot(path string) error {
	logger.Debugf("Saving screenshot, serial=%s path=%s", a.Name(), path)
	return a.screenCapture.SaveScreenshot(path)
}

func (a *AndroidDriver) Record(path string) error {
	logger.Debugf("Starting screen recording, serial=%s path=%s", a.Name(), path)
	return a.screenCapture.Record(path)
}

func (a *AndroidDriver) StopRecording() error {
	logger.Debugf("Stopping screen recording, serial=%s", a.Name())
	return a.screenCapture.StopRecording()
}

func (a *AndroidDriver) GetPageSource(pageSourceType string) common.IPageSource {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if ps, ok := a.pageSources[PageType(pageSourceType)]; ok {
		logger.Debugf("Page source found, serial=%s type=%s", a.Name(), pageSourceType)
		return ps
	}
	logger.Debugf("Page source not found, serial=%s type=%s", a.Name(), pageSourceType)
	return nil
}

func (a *AndroidDriver) GetUIAClient(remoteUrl string) (*uia.UiaClient, error) {
	client, err := uia.NewUiaClient(remoteUrl)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, fmt.Errorf("failed to create UIA client")
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
	pocoPageSource, err := poco.NewPocoPageSourceWith(engine, port)
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

func (a *AndroidDriver) initPoco() error {
	a.frowardPocoPort = common.GetRandomPort()
	logger.Infof("Starting Poco port forwarding, localPort=%d remotePort=%d", a.frowardPocoPort, a.pocoPort)
	err := a.device.FrowardTcp(a.frowardPocoPort, a.pocoPort)
	if err != nil {
		return err
	}
	pocoPage, err := poco.NewPocoPageSourceWith(a.pocoEngine, a.frowardPocoPort)
	if err != nil {
		return err
	}
	a.pageSources[PageTypePoco] = pocoPage
	logger.Infof("Poco page source registered, type=%s localPort=%d", PageTypePoco, a.frowardPocoPort)
	return nil
}

func (a *AndroidDriver) checkUiaApkVersion() (bool, error) {
	output, err := a.device.RunShellCommand("dumpsys package", uiaServerPackage)
	if err != nil {
		return false, err
	}
	return strings.Contains(output, "versionName="+uiaApkVersion), nil
}

func (a *AndroidDriver) initUIA() error {
	logger.Infof("Starting UIA initialization, serial=%s", a.Name())
	checkUIARes, err := a.checkUiaApkVersion()
	if err != nil {
		return err
	}

	if !checkUIARes {
		logger.Info("UIA APK version mismatch detected, reinstalling packages")
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
		logger.Info("UIA APK installation and permission setup completed")
	}

	uiaPort := common.GetRandomPort()
	logger.Infof("Preparing to start UIA server, localPort=%d remotePort=%d", uiaPort, uiaServerPort)

	if err := a.startUIAServer(uiaPort); err != nil {
		return err
	}

	a.uiaClient, err = uia.NewUiaClient(fmt.Sprintf("http://localhost:%d", uiaPort))
	if err != nil {
		return err
	}
	logger.Infof("UIA client created, baseUrl=%s", a.uiaClient.RemoteUrl)

	a.pageSources[PageTypeUIA] = page.NewUIAPageSource(a.uiaClient)
	logger.Infof("UIA page source registered, type=%s", PageTypeUIA)
	return nil
}

func (a *AndroidDriver) startUIAServer(uiaPort int) error {
	logger.Infof("Starting UIA instrumentation, localPort=%d remotePort=%d", uiaPort, uiaServerPort)
	if err := a.device.FrowardTcp(uiaPort, uiaServerPort); err != nil {
		return fmt.Errorf("forward uia port failed: %w", err)
	}
	logger.Infof("UIA port forwarding established, localPort=%d", uiaPort)

	conn, err := a.device.RunShellLoopCommandSock(uiaInstrumentationCmd)
	if err != nil {
		_ = a.device.ForwardKill(uiaPort)
		return fmt.Errorf("start uia instrumentation failed: %w", err)
	}
	logger.Info("UIA instrumentation command started, waiting for readiness")

	a.uiaServerConn = conn
	a.uiaPort = uiaPort

	readyCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go func() {
		reader := bufio.NewReader(conn)
		readySent := false
		for {
			line, readErr := reader.ReadString('\n')
			if line != "" {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					logger.Info(trimmed)
					if !readySent && strings.Contains(trimmed, uiaReadyLogMarker) {
						time.Sleep(uiaStartupReadyDelay)
						readySent = true
						select {
						case readyCh <- struct{}{}:
						default:
						}
					}
				}
			}

			if readErr != nil {
				if !readySent && readErr != io.EOF {
					select {
					case errCh <- fmt.Errorf("uia instrumentation output failed: %w", readErr):
					default:
					}
				}
				return
			}
		}
	}()

	deadline := time.Now().Add(uiaStartupWaitTimeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-readyCh:
			if err := waitForUIAServerHTTPReady(uiaPort, 10*time.Second); err != nil {
				_ = conn.Close()
				a.uiaServerConn = nil
				_ = a.device.ForwardKill(uiaPort)
				a.uiaPort = 0
				return err
			}
			logger.Infof("UIA server is ready by log marker and HTTP probe, localPort=%d", uiaPort)
			return nil
		case err := <-errCh:
			_ = conn.Close()
			a.uiaServerConn = nil
			_ = a.device.ForwardKill(uiaPort)
			a.uiaPort = 0
			return err
		case <-ticker.C:
			if err := checkUIAServerHTTPReady(uiaPort); err == nil {
				logger.Infof("UIA server is ready by HTTP probe, localPort=%d", uiaPort)
				return nil
			}
			if time.Now().After(deadline) {
				_ = conn.Close()
				a.uiaServerConn = nil
				_ = a.device.ForwardKill(uiaPort)
				a.uiaPort = 0
				return fmt.Errorf("wait for uia server startup timeout after %s", uiaStartupWaitTimeout)
			}
		}
	}
}

func waitForUIAServerHTTPReady(uiaPort int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := checkUIAServerHTTPReady(uiaPort); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("wait for uia http ready timeout after %s", timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func checkUIAServerHTTPReady(uiaPort int) error {
	client := &http.Client{Timeout: 2 * time.Second}
	urls := []string{
		fmt.Sprintf("http://localhost:%d/status", uiaPort),
		fmt.Sprintf("http://localhost:%d/wd/hub/status", uiaPort),
	}

	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
			return nil
		}
	}

	return fmt.Errorf("uia http endpoint is not ready")
}

func resolveUIAPluginsDir() (string, error) {
	projectRoot, err := common.GetPluginDirPath()
	if err != nil {
		return "", fmt.Errorf("resolve repo root failed: %w", err)
	}
	return filepath.Join(projectRoot, "plugins", "uia"), nil
}
