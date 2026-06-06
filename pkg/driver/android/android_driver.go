package android

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/core/primitives"
	"trek/logger"
	"trek/pkg/driver/android/adb"
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
	_ common.IDeviceState = (*AndroidDriver)(nil)

	surfaceOrientationRegex    = regexp.MustCompile(`(?m)SurfaceOrientation:\s*(\d+)`)
	displayOrientationRegex    = regexp.MustCompile(`(?m)orientation=(\d+)`)
	currentRotationNumberRegex = regexp.MustCompile(`(?m)mCurrentRotation=(\d+)`)
	currentRotationEnumRegex   = regexp.MustCompile(`(?m)mCurrentRotation=ROTATION_(\d+)`)
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
	device        *adb.Device
	touch         common.ITouch
	screenCapture common.IScreenCapture
	pageSources   map[PageType]common.IPageSource
	mu            sync.RWMutex

	uiaClient     *uia.UiaClient
	uiaServerConn net.Conn
	uiaPort       int
	uiaServerPort int
	isUIATouch    bool

	pocoPort        int
	forwardPocoPort int
	pocoEngine      poco.Engine

	adbKeyboardAvailable bool   // ADBKeyboard 是否可用
	previousIME          string // 切换前的输入法，用于恢复

	anrRegexCache sync.Map // map[string]*regexp.Regexp，按包名缓存编译后的 ANR 正则
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

func WithUIAServerPort(port int) AndroidDriverOption {
	return func(d *AndroidDriver) {
		if port > 0 {
			d.uiaServerPort = port
		}
	}
}

func WithTouch(touchType TouchType) AndroidDriverOption {
	return func(d *AndroidDriver) {
		switch touchType {
		case TouchTypeADB:
			d.touch = touch.NewADBTouch(d.device)
		case TouchTypeMotion:
			mt, err := touch.NewMotionTouch(d.device)
			if err != nil {
				logger.Warnf("初始化 MotionTouch 失败，回退到 ADB Touch: %v", err)
				d.touch = touch.NewADBTouch(d.device)
			} else {
				d.touch = mt
			}
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
		screenCapture: screen.NewScreenCaptureWithScrcpy(device, nil),
		pageSources:   make(map[PageType]common.IPageSource),
		uiaServerPort: uiaServerPort,
	}

	for _, opt := range opts {
		opt(androidDriver)
	}
	logger.Infof("Driver options applied, isUIATouch=%t pocoPort=%d pocoEngine=%s", androidDriver.isUIATouch, androidDriver.pocoPort, androidDriver.pocoEngine)

	err = androidDriver.initUIA()
	if err != nil {
		return nil, err
	}
	logger.Infof("UIA initialization completed, serial=%s", device.Serial())

	if androidDriver.pocoPort > 0 {
		logger.Infof("Starting Poco page source initialization, remotePort=%d engine=%s", androidDriver.pocoPort, androidDriver.pocoEngine)
		err = androidDriver.initPoco()
		if err != nil {
			return nil, err
		}
		logger.Infof("Poco page source initialization completed, localPort=%d remotePort=%d", androidDriver.forwardPocoPort, androidDriver.pocoPort)
	}

	if androidDriver.isUIATouch {
		if androidDriver.uiaClient != nil {
			androidDriver.touch = touch.NewUIATouch(androidDriver.uiaClient)
			logger.Info("Switched to UIA touch mode")
		} else {
			logger.Warn("UIA Touch Mode is not available, ADB Touch Mode will be used")
		}
	}

	// 检测并启用 ADBKeyboard 输入法
	androidDriver.setupADBKeyboard(context.Background())

	logger.Infof("AndroidDriver initialization completed, serial=%s", device.Serial())
	return androidDriver, nil
}

func (a *AndroidDriver) Click(point primitives.Point) error {
	return a.touch.Click(point)
}

func (a *AndroidDriver) LongClick(point primitives.Point, duration int64) error {
	return a.touch.LongClick(point, duration)
}

func (a *AndroidDriver) Swipe(startPoint primitives.Point, endPoint primitives.Point, step int64, duration int64) error {
	return a.touch.Swipe(startPoint, endPoint, step, duration)
}

func (a *AndroidDriver) Pinch(centerPoint primitives.Point, startDistance float64, endDistance float64, duration int64) error {
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
	_, err := a.device.RunShellCommand(context.Background(), "input", "keyevent", "4")
	return err
}

func (a *AndroidDriver) GetCurrentPackage(ctx context.Context) (string, error) {
	if a.device == nil {
		return "", fmt.Errorf("device is nil")
	}
	return a.device.GetCurrentPackage(ctx)
}

func (a *AndroidDriver) GetCurrentActivity(ctx context.Context) (string, error) {
	if a.device == nil {
		return "", fmt.Errorf("device is nil")
	}
	return a.device.GetCurrentActivity(ctx)
}

// StartApp 启动指定包名应用。
func (a *AndroidDriver) StartApp(packageName string) error {
	if strings.TrimSpace(packageName) == "" {
		return fmt.Errorf("packageName is empty")
	}
	if a.device == nil {
		return fmt.Errorf("device is nil")
	}
	_, err := a.device.RunShellCommand(context.Background(), "monkey", "-p", packageName, "-c", "android.intent.category.LAUNCHER", "1")
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
	if _, err := a.device.RunShellCommand(context.Background(), "am", "force-stop", packageName); err != nil {
		return err
	}
	if clean {
		if _, err := a.device.RunShellCommand(context.Background(), "pm", "clear", packageName); err != nil {
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
// 若 ADBKeyboard 可用，优先使用它（支持 Unicode）。
func (a *AndroidDriver) InputText(text string, clear bool) error {
	if a.adbKeyboardAvailable {
		if clear {
			_ = a.sendADBKeyboardClear()
		}
		return a.sendADBKeyboardText(text)
	}
	if a.uiaClient == nil {
		return fmt.Errorf("uia client is nil")
	}
	return a.uiaClient.SendKeys(text, clear)
}

const adbKeyboardIME = "com.android.adbkeyboard/.AdbIME"
const adbKeyboardAPK = "plugins/ADBKeyBoard/keyboardservice-debug.apk"

// setupADBKeyboard 检测、安装并启用 ADBKeyboard 输入法。
func (a *AndroidDriver) setupADBKeyboard(ctx context.Context) {
	if a.device == nil {
		return
	}
	// 检查 ADBKeyboard 是否已安装
	output, err := a.device.RunShellCommand(ctx, "pm", "list", "packages", "com.android.adbkeyboard")
	if err != nil || !strings.Contains(output, "com.android.adbkeyboard") {
		// 未安装，尝试自动安装
		if !a.installADBKeyboard() {
			return
		}
	}
	// 保存当前输入法
	imeOutput, err := a.device.RunShellCommand(ctx, "settings", "get", "secure", "default_input_method")
	if err == nil {
		a.previousIME = strings.TrimSpace(imeOutput)
	}
	// 启用并切换到 ADBKeyboard
	_, _ = a.device.RunShellCommand(ctx, "ime", "enable", adbKeyboardIME)
	_, err = a.device.RunShellCommand(ctx, "ime", "set", adbKeyboardIME)
	if err != nil {
		logger.Warnf("切换到 ADBKeyboard 失败: %v", err)
		a.previousIME = ""
		return
	}
	// 某些设备 ime set 不生效，同时写入 settings 作为保障
	_, _ = a.device.RunShellCommand(ctx, "settings", "put", "secure", "default_input_method", adbKeyboardIME)
	// 验证是否切换成功
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 5*time.Second)
	defer verifyCancel()
	currentIME, err := a.device.RunShellCommand(verifyCtx, "settings", "get", "secure", "default_input_method")
	currentIME = strings.TrimSpace(currentIME)
	if err != nil || !strings.Contains(currentIME, "adbkeyboard") {
		logger.Warnf("ADBKeyboard 切换验证失败: current=%s err=%v", currentIME, err)
		a.previousIME = ""
		return
	}
	a.adbKeyboardAvailable = true
	logger.Infof("ADBKeyboard 已启用并验证，原始输入法: %s，当前: %s", a.previousIME, currentIME)
}

// installADBKeyboard 从 plugins 目录安装 ADBKeyboard APK。
func (a *AndroidDriver) installADBKeyboard() bool {
	absPath, err := filepath.Abs(adbKeyboardAPK)
	if err != nil {
		logger.Warnf("解析 ADBKeyboard APK 路径失败: %v", err)
		return false
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		logger.Infof("ADBKeyboard APK 不存在: %s，跳过安装", absPath)
		return false
	}
	logger.Infof("正在安装 ADBKeyboard: %s", absPath)
	if err := utils.InstallAPK(a.device.Serial(), absPath, true); err != nil {
		logger.Warnf("安装 ADBKeyboard 失败: %v", err)
		return false
	}
	logger.Infof("ADBKeyboard 安装成功")
	return true
}

// restoreIME 恢复到切换前的输入法。
func (a *AndroidDriver) restoreIME(ctx context.Context) {
	if !a.adbKeyboardAvailable || a.device == nil {
		return
	}
	if a.previousIME == "" {
		// 无原始输入法记录，直接重置
		_, _ = a.device.RunShellCommand(ctx, "ime", "reset")
		logger.Infof("输入法已重置为系统默认")
	} else {
		_, err := a.device.RunShellCommand(ctx, "ime", "set", a.previousIME)
		if err != nil {
			logger.Warnf("恢复输入法失败: %v，尝试重置", err)
			_, _ = a.device.RunShellCommand(ctx, "ime", "reset")
		} else {
			logger.Infof("输入法已恢复: %s", a.previousIME)
		}
	}
	a.adbKeyboardAvailable = false
	a.previousIME = ""
}

// sendADBKeyboardText 通过 ADBKeyboard broadcast 发送文本。
func (a *AndroidDriver) sendADBKeyboardText(text string) error {
	if a.device == nil {
		return fmt.Errorf("adb device is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := a.device.RunShellCommand(ctx, "am", "broadcast", "-a", "ADB_INPUT_TEXT", "--es", "msg", text)
	return err
}

// sendADBKeyboardClear 清空 ADBKeyboard 当前输入框内容。
func (a *AndroidDriver) sendADBKeyboardClear() error {
	if a.device == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := a.device.RunShellCommand(ctx, "am", "broadcast", "-a", "ADB_CLEAR_TEXT")
	return err
}

// ClearLogcat 清空 logcat 缓冲，避免历史日志干扰本轮检测。
func (a *AndroidDriver) ClearLogcat() error {
	if a.device == nil {
		return fmt.Errorf("device is nil")
	}
	_, err := a.device.RunShellCommand(context.Background(), "logcat", "-c")
	return err
}

// GetScreenRotation 通过 ADB 读取设备当前实际屏幕朝向。
func (a *AndroidDriver) GetScreenRotation() (int, error) {
	if a.device == nil {
		return 0, fmt.Errorf("device is nil")
	}

	inputOutput, inputErr := a.device.RunShellCommand(context.Background(), "dumpsys", "input")
	displayOutput, displayErr := a.device.RunShellCommand(context.Background(), "dumpsys", "display")
	windowOutput, windowErr := a.device.RunShellCommand(context.Background(), "dumpsys", "window")

	rotation, source, err := parseScreenRotation(inputOutput, displayOutput, windowOutput)
	if err == nil {
		logger.Debugf("GetScreenRotation success: serial=%s rotation=%d source=%s", a.Name(), rotation, source)
		return rotation, nil
	}

	errParts := make([]string, 0, 4)
	if inputErr != nil {
		errParts = append(errParts, fmt.Sprintf("dumpsys input: %v", inputErr))
	}
	if displayErr != nil {
		errParts = append(errParts, fmt.Sprintf("dumpsys display: %v", displayErr))
	}
	if windowErr != nil {
		errParts = append(errParts, fmt.Sprintf("dumpsys window: %v", windowErr))
	}
	errParts = append(errParts, err.Error())
	return 0, fmt.Errorf("获取屏幕朝向失败: %s", strings.Join(errParts, "; "))
}

func parseScreenRotation(inputOutput string, displayOutput string, windowOutput string) (int, string, error) {
	if rotation, ok := parseRotationWithRegex(inputOutput, surfaceOrientationRegex); ok {
		return rotation, "input", nil
	}
	if rotation, ok := parseRotationWithRegex(displayOutput, displayOrientationRegex); ok {
		return rotation, "display", nil
	}
	if rotation, ok := parseRotationWithRegex(displayOutput, currentRotationNumberRegex); ok {
		return rotation, "display", nil
	}
	if rotation, ok := parseRotationWithRegex(displayOutput, currentRotationEnumRegex); ok {
		return rotation, "display", nil
	}
	if rotation, ok := parseRotationWithRegex(windowOutput, currentRotationNumberRegex); ok {
		return rotation, "window", nil
	}
	if rotation, ok := parseRotationWithRegex(windowOutput, currentRotationEnumRegex); ok {
		return rotation, "window", nil
	}
	return 0, "", fmt.Errorf("未在 dumpsys input/display/window 中找到可用朝向信号")
}

func parseRotationWithRegex(output string, pattern *regexp.Regexp) (int, bool) {
	match := pattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, false
	}
	rotation, err := strconv.Atoi(strings.TrimSpace(match[1]))
	if err != nil {
		return 0, false
	}
	if rotation < 0 || rotation > 3 {
		return 0, false
	}
	return rotation, true
}

// CheckCrash 通过系统日志检测是否出现 crash。
// 仅依赖 logcat 中的 fatal exception 信号，不使用 pidof 判定。
// 原因：pidof 对后台进程可能返回空，导致系统正常回收进程时误判为 crash。
func (a *AndroidDriver) CheckCrash(packageName string) (bool, error) {
	if a.device == nil {
		return false, fmt.Errorf("device is nil")
	}

	pkgLower := strings.ToLower(strings.TrimSpace(packageName))

	logcatOut, err := a.device.RunShellCommand(context.Background(), "logcat", "-d", "-b", "main", "-t", "50", "AndroidRuntime:E", "*:S")
	if err != nil {
		return false, err
	}
	logLower := strings.ToLower(logcatOut)
	if strings.Contains(logLower, "fatal exception") {
		if pkgLower == "" || strings.Contains(logLower, pkgLower) {
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

	pkgLower := strings.ToLower(strings.TrimSpace(packageName))

	// 只基于当前系统进程状态判断 ANR，避免被 dropbox 历史记录误判。
	dumpsysOut, err := a.device.RunShellCommand(context.Background(), "dumpsys", "activity", "processes")
	if err != nil {
		return false, err
	}
	dumpLower := strings.ToLower(dumpsysOut)

	// 未指定包名时，沿用全局判定。
	if pkgLower == "" {
		return strings.Contains(dumpLower, "notresponding=true"), nil
	}

	// 精确匹配"目标进程块内"的 notResponding=true，避免"包名出现 + 其他进程 ANR"误判。
	// 说明：
	// 1. 先锚定包含目标包名的 ProcessRecord 行
	// 2. 在其后有限行范围（最多 120 行）内查找 notResponding=true
	// 3. 使用 DOTALL/不区分大小写匹配，兼容不同 ROM 输出格式
	re := a.getOrCompileANRRegex(pkgLower)
	if re.MatchString(dumpLower) {
		return true, nil
	}

	// 调试辅助：当出现全局 ANR 但目标包未命中时输出一次提示，便于排查是否他进程 ANR。
	if strings.Contains(dumpLower, "notresponding=true") {
		logger.Debugf("CheckANR detected global notResponding=true but target package not matched, package=%s", pkgLower)
	}

	return false, nil
}

// getOrCompileANRRegex 按包名缓存编译后的 ANR 检测正则，避免每步重复编译。
func (a *AndroidDriver) getOrCompileANRRegex(pkgLower string) *regexp.Regexp {
	if cached, ok := a.anrRegexCache.Load(pkgLower); ok {
		return cached.(*regexp.Regexp)
	}
	pattern := fmt.Sprintf(`(?is)processrecord\{[^\n]*%s[^\n]*\n(?:[^\n]*\n){0,120}?[^\n]*notresponding=true`, regexp.QuoteMeta(pkgLower))
	re := regexp.MustCompile(pattern)
	a.anrRegexCache.Store(pkgLower, re)
	return re
}

func (a *AndroidDriver) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	logger.Infof("Starting AndroidDriver shutdown, serial=%s", a.Name())

	// 恢复输入法
	a.restoreIME(context.Background())

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
		if err := a.device.ForwardKill(context.Background(), a.uiaPort); err != nil {
			logger.Warnf("remove UIA forward failed: %v", err)
		} else {
			logger.Infof("UIA port forwarding removed, localPort=%d", a.uiaPort)
		}
		a.uiaPort = 0
	}
	if a.forwardPocoPort > 0 && a.device != nil {
		if err := a.device.ForwardKill(context.Background(), a.forwardPocoPort); err != nil {
			logger.Warnf("remove POCO forward failed: %v", err)
		} else {
			logger.Infof("Poco port forwarding removed, localPort=%d", a.forwardPocoPort)
		}
		a.forwardPocoPort = 0
	}

	a.pageSources = make(map[PageType]common.IPageSource)
	logger.Infof("AndroidDriver shutdown completed, serial=%s", a.Name())
	return nil
}

func (a *AndroidDriver) Screenshot(ctx context.Context) ([]byte, error) {
	logger.Debugf("Taking screenshot, serial=%s", a.Name())
	return a.screenCapture.Screenshot(ctx)
}

// StartBackgroundScreenshot 启动后台截图线程，Screenshot() 之后直接返回最新帧。
func (a *AndroidDriver) StartBackgroundScreenshot(ctx context.Context, interval time.Duration) {
	if sc, ok := a.screenCapture.(*screen.ScreenCapture); ok {
		sc.StartBackground(ctx, interval)
	}
}

// StopBackgroundScreenshot 停止后台截图线程。
func (a *AndroidDriver) StopBackgroundScreenshot() {
	if sc, ok := a.screenCapture.(*screen.ScreenCapture); ok {
		sc.StopBackground()
	}
}

// MarkActionDone 记录动作完成时间，确保后续截图反映动作后的屏幕状态。
func (a *AndroidDriver) MarkActionDone() {
	if sc, ok := a.screenCapture.(*screen.ScreenCapture); ok {
		sc.MarkActionDone()
	}
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
	a.pageSources[PageTypeUIA] = uiPageSource
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

// CheckEnvironment 在运行前检测 ADB 与页面源（UIA）环境是否就绪。
func (a *AndroidDriver) CheckEnvironment(pageSourceType string) (*common.EnvironmentCheckResult, error) {
	result := &common.EnvironmentCheckResult{
		PageSourceType: pageSourceType,
		DeviceName:     a.Name(),
	}

	if err := utils.EnsureADBServer(); err != nil {
		result.Detail = "adb 环境不可用"
		return result, fmt.Errorf("adb 环境不可用: %w", err)
	}
	result.ADBReady = true
	if a.device == nil {
		result.Detail = "device is nil"
		return result, fmt.Errorf("device is nil")
	}
	if strings.TrimSpace(a.device.Serial()) == "" {
		result.Detail = "device serial is empty"
		return result, fmt.Errorf("device serial is empty")
	}
	if _, err := a.device.RunShellCommand(context.Background(), "echo", "ok"); err != nil {
		result.Detail = "adb shell 不可用"
		return result, fmt.Errorf("adb shell 不可用: %w", err)
	}
	result.DeviceReady = true

	if strings.EqualFold(strings.TrimSpace(pageSourceType), "screenshot") {
		result.PageSourceReady = true
		result.Detail = "screenshot 模式仅依赖设备截图"
		return result, nil
	}

	if a.GetPageSource(pageSourceType) == nil {
		result.Detail = "页面源不可用"
		return result, fmt.Errorf("页面源不可用: %s", pageSourceType)
	}
	result.PageSourceReady = true

	if PageType(pageSourceType) == PageTypeUIA {
		if a.uiaClient == nil {
			result.Detail = "uia client is nil"
			return result, fmt.Errorf("uia client is nil")
		}
		if err := a.uiaClient.CheckSessionId(); err != nil {
			result.Detail = "uia 会话不可用"
			return result, fmt.Errorf("uia 会话不可用: %w", err)
		}
		result.UIAReady = true
	} else {
		result.UIAReady = true
	}

	result.Detail = "ok"
	return result, nil
}

func (a *AndroidDriver) initPoco() error {
	port, err := common.GetRandomPort()
	if err != nil {
		return fmt.Errorf("获取 Poco 随机端口失败: %w", err)
	}
	a.forwardPocoPort = port
	logger.Infof("Starting Poco port forwarding, localPort=%d remotePort=%d", a.forwardPocoPort, a.pocoPort)
	err = a.device.ForwardTcp(context.Background(), a.forwardPocoPort, a.pocoPort)
	if err != nil {
		return err
	}
	pocoPage, err := poco.NewPocoPageSourceWith(a.pocoEngine, a.forwardPocoPort)
	if err != nil {
		return err
	}
	a.pageSources[PageTypePoco] = pocoPage
	logger.Infof("Poco page source registered, type=%s localPort=%d", PageTypePoco, a.forwardPocoPort)
	return nil
}

func (a *AndroidDriver) checkUiaApkVersion() (bool, error) {
	output, err := a.device.RunShellCommand(context.Background(), "dumpsys package", uiaServerPackage)
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
			if _, err := a.device.RunShellCommand(context.Background(), command); err != nil {
				return fmt.Errorf("execute command failed: %s: %w", command, err)
			}
		}
		logger.Info("UIA APK installation and permission setup completed")
	}

	uiaPort, err := common.GetRandomPort()
	if err != nil {
		return fmt.Errorf("获取 UIA 随机端口失败: %w", err)
	}
	logger.Infof("Preparing to start UIA server, localPort=%d remotePort=%d", uiaPort, a.uiaServerPort)

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
	logger.Infof("Starting UIA instrumentation, localPort=%d remotePort=%d", uiaPort, a.uiaServerPort)
	if err := a.device.ForwardTcp(context.Background(), uiaPort, a.uiaServerPort); err != nil {
		return fmt.Errorf("forward uia port failed: %w", err)
	}
	logger.Infof("UIA port forwarding established, localPort=%d", uiaPort)

	conn, err := a.device.RunShellLoopCommandSock(context.Background(), uiaInstrumentationCmd)
	if err != nil {
		_ = a.device.ForwardKill(context.Background(), uiaPort)
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
				_ = a.device.ForwardKill(context.Background(), uiaPort)
				a.uiaPort = 0
				return err
			}
			logger.Infof("UIA server is ready by log marker and HTTP probe, localPort=%d", uiaPort)
			return nil
		case err := <-errCh:
			_ = conn.Close()
			a.uiaServerConn = nil
			_ = a.device.ForwardKill(context.Background(), uiaPort)
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
				_ = a.device.ForwardKill(context.Background(), uiaPort)
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
