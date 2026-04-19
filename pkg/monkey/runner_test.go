package monkey

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"
	"trek/internal/engine/core/types"
	"trek/pkg/driver/common"
	"trek/pkg/session"
)

type fakeDecider struct {
	commands []*types.ActionCommand
	idx      int
	lastPage string
	lastXML  string
}

func (f *fakeDecider) NextActionWithInput(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	f.lastPage = pageName
	f.lastXML = input.XMLDescOfGuiTree
	if len(f.commands) == 0 {
		return types.NewActionCommand(), nil
	}
	cmd := f.commands[f.idx%len(f.commands)]
	f.idx++
	return cmd, nil
}

type failingDecider struct{}

func (f *failingDecider) NextActionWithInput(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	return nil, errors.New("decide failed")
}

type observingDecider struct {
	fakeDecider
	results []session.StepResultInput
}

func (o *observingDecider) OnStepResult(result session.StepResultInput) error {
	o.results = append(o.results, result)
	return nil
}

type weightedDecider struct {
	candidates []WeightedCandidate
}

type transformingDecider struct {
	fakeDecider
	pageName       string
	xml            string
	sawScreenshot  bool
	transformError error
}

func (d *transformingDecider) TransformPageInfoWithInput(pageName string, input session.ActionInput) (session.PageInfo, error) {
	if len(input.Screenshot) > 0 {
		d.sawScreenshot = true
	}
	if d.transformError != nil {
		return session.PageInfo{}, d.transformError
	}
	return session.PageInfo{
		PageName: d.pageName,
		XML:      d.xml,
	}, nil
}

func (w *weightedDecider) NextActionWithInput(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	if len(w.candidates) == 0 {
		return nil, nil
	}
	return w.candidates[0].Command, nil
}

func (w *weightedDecider) NextWeightedActionsWithInput(pageName string, input session.ActionInput) ([]WeightedCandidate, error) {
	return w.candidates, nil
}

type fakePageSource struct {
	xml string
}

func (f *fakePageSource) DumpPageSource() (string, error) { return f.xml, nil }
func (f *fakePageSource) Close() error                    { return nil }

type fakeDriver struct {
	pageSource       common.IPageSource
	clickCount       int
	startCount       int
	activateCount    int
	activateErr      error
	currentPkgErr    error
	clearCnt         int
	envCheckCnt      int
	crash            bool
	anr              bool
	currentPackage   string
	targetOnActivate string
	currentPkgCalls  int
	envResult        *common.EnvironmentCheckResult
	envErr           error
	crashAfterClick  bool
	anrAfterClick    bool
	currentActivity  string
	currentActErr    error
}

func (f *fakeDriver) Click(point types.Point) error {
	f.clickCount++
	if f.crashAfterClick {
		f.crash = true
	}
	if f.anrAfterClick {
		f.anr = true
	}
	return nil
}
func (f *fakeDriver) LongClick(point types.Point, duration int64) error { return nil }
func (f *fakeDriver) Swipe(startPoint types.Point, endPoint types.Point, step int64, duration int64) error {
	return nil
}
func (f *fakeDriver) Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error {
	return nil
}
func (f *fakeDriver) TouchEvent(touchList ...common.TouchEvent) error { return nil }
func (f *fakeDriver) Close() error                                    { return nil }
func (f *fakeDriver) Screenshot() ([]byte, error)                     { return []byte{1}, nil }
func (f *fakeDriver) SaveScreenshot(path string) error                { return nil }
func (f *fakeDriver) Record(path string) error                        { return nil }
func (f *fakeDriver) StopRecording() error                            { return nil }
func (f *fakeDriver) GetPageSource(pageSourceType string) common.IPageSource {
	if pageSourceType != "uia" {
		return nil
	}
	return f.pageSource
}
func (f *fakeDriver) Name() string { return "fake-device" }
func (f *fakeDriver) GetInfo() map[string]interface{} {
	return map[string]interface{}{"device": "fake"}
}
func (f *fakeDriver) Back() error                                     { return nil }
func (f *fakeDriver) StartApp(packageName string) error               { f.startCount++; return nil }
func (f *fakeDriver) RestartApp(packageName string, clean bool) error { return nil }
func (f *fakeDriver) ActivateApp(packageName string) error {
	f.activateCount++
	if f.activateErr != nil {
		return f.activateErr
	}
	f.currentPackage = packageName
	f.targetOnActivate = packageName
	return nil
}
func (f *fakeDriver) GetCurrentPackage() (string, error) {
	f.currentPkgCalls++
	if f.currentPkgErr != nil {
		return "", f.currentPkgErr
	}
	return f.currentPackage, nil
}
func (f *fakeDriver) GetCurrentActivity() (string, error) {
	if f.currentActErr != nil {
		return "", f.currentActErr
	}
	return f.currentActivity, nil
}
func (f *fakeDriver) InputText(text string, clear bool) error     { return nil }
func (f *fakeDriver) CheckCrash(packageName string) (bool, error) { return f.crash, nil }
func (f *fakeDriver) CheckANR(packageName string) (bool, error)   { return f.anr, nil }
func (f *fakeDriver) ClearLogcat() error {
	f.clearCnt++
	return nil
}
func (f *fakeDriver) CheckEnvironment(pageSourceType string) (*common.EnvironmentCheckResult, error) {
	f.envCheckCnt++
	if f.envResult != nil {
		return f.envResult, f.envErr
	}
	return &common.EnvironmentCheckResult{
		ADBReady:        true,
		DeviceReady:     true,
		PageSourceReady: true,
		UIAReady:        true,
		PageSourceType:  pageSourceType,
		DeviceName:      "fake-device",
		Detail:          "ok",
	}, f.envErr
}

func TestRunnerRunCompleted(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{MaxSteps: 3, StepInterval: 0, KeepStepRecords: true, StopOnCrash: true, StopOnANR: true})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if report.StepsSucceeded != 3 {
		t.Fatalf("成功步数错误: %d", report.StepsSucceeded)
	}
	if driver.clickCount != 3 {
		t.Fatalf("点击执行次数错误: %d", driver.clickCount)
	}
	if report.Preflight == nil || !report.Preflight.ADBReady {
		t.Fatalf("预期记录前置检测结果")
	}
}

func TestRunnerReportsStepResultWithAfterPageCrashANRAndScreenshot(t *testing.T) {
	decider := &observingDecider{
		fakeDecider: fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}},
	}
	driver := &fakeDriver{
		pageSource:      &fakePageSource{xml: `<node class="MainActivity"/>`},
		crashAfterClick: true,
		anrAfterClick:   true,
	}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:       "com.example.app",
		MaxSteps:          1,
		StepInterval:      0,
		KeepStepRecords:   true,
		CaptureScreenshot: true,
		StopOnCrash:       true,
		StopOnANR:         true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if len(decider.results) != 1 {
		t.Fatalf("预期收到 1 条 step result，实际: %d", len(decider.results))
	}
	result := decider.results[0]
	if !result.Success || !result.Crash || !result.ANR {
		t.Fatalf("执行结果状态不符合预期: %+v", result)
	}
	if result.Before.XML == "" || result.After == nil || result.After.XML == "" {
		t.Fatalf("预期包含执行前后 xml: %+v", result)
	}
	if len(result.Before.Screenshot) == 0 || len(result.After.Screenshot) == 0 {
		t.Fatalf("预期包含执行前后截图: %+v", result)
	}
}

func TestRunnerDetectCrashBySystemSignal(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}, crash: true}

	runner, err := NewRunner(decider, driver, Config{MaxSteps: 5, StepInterval: 0, KeepStepRecords: true, StopOnCrash: true, StopOnANR: true})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCrashDetectedLogcat {
		t.Fatalf("预期系统信号 crash 停止，实际: %s", report.StopReason)
	}
	if driver.clearCnt == 0 {
		t.Fatalf("预期启动前清理 logcat")
	}
}

func TestRunnerAutoStartOnRunDefaultEnabled(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:     "com.example.app",
		MaxSteps:        1,
		StepInterval:    0,
		KeepStepRecords: true,
		StopOnCrash:     true,
		StopOnANR:       true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if driver.startCount != 1 {
		t.Fatalf("默认应自动启动一次应用，实际: %d", driver.startCount)
	}
}

func TestRunnerAutoStartOnRunDisabled(t *testing.T) {
	disabled := false
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:     "com.example.app",
		AutoStartOnRun:  &disabled,
		MaxSteps:        1,
		StepInterval:    0,
		KeepStepRecords: true,
		StopOnCrash:     true,
		StopOnANR:       true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if driver.startCount != 0 {
		t.Fatalf("关闭自动启动后不应启动应用，实际: %d", driver.startCount)
	}
	if driver.envCheckCnt == 0 {
		t.Fatalf("关闭自动启动后仍应执行前置检测")
	}
}

func TestRunnerRecoversWhenOutOfTargetPackage(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:     &fakePageSource{xml: `<node class="MainActivity"/>`},
		currentPackage: "com.android.settings",
	}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:     "com.example.app",
		AutoStartOnRun:  boolPtr(false),
		MaxSteps:        2,
		StepInterval:    0,
		KeepStepRecords: true,
		StopOnCrash:     true,
		StopOnANR:       true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if report.OutOfAppRecoveries != 1 {
		t.Fatalf("预期发生一次离开应用恢复，实际: %d", report.OutOfAppRecoveries)
	}
	if driver.activateCount != 1 || driver.targetOnActivate != "com.example.app" {
		t.Fatalf("预期激活被测应用一次，实际 activateCount=%d target=%s", driver.activateCount, driver.targetOnActivate)
	}
	if driver.clickCount != 1 {
		t.Fatalf("预期恢复后执行一次点击，实际: %d", driver.clickCount)
	}
}

func TestRunnerOutOfTargetPackageActivationFailed(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:     &fakePageSource{xml: `<node class="MainActivity"/>`},
		currentPackage: "com.android.settings",
		activateErr:    errors.New("activate failed"),
	}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:            "com.example.app",
		AutoStartOnRun:         boolPtr(false),
		MaxSteps:               1,
		MaxConsecutiveFailures: 1,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopMaxConsecutiveFailures {
		t.Fatalf("预期恢复失败触发最大连续失败停止，实际: %s", report.StopReason)
	}
	if report.StepsFailed != 1 {
		t.Fatalf("预期记录 1 次失败，实际: %d", report.StepsFailed)
	}
	if driver.clickCount != 0 {
		t.Fatalf("激活失败时不应继续执行点击，实际: %d", driver.clickCount)
	}
}

func TestRunnerCurrentPackageMonitorError(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:    &fakePageSource{xml: `<node class="MainActivity"/>`},
		currentPkgErr: errors.New("pkg query failed"),
	}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:            "com.example.app",
		AutoStartOnRun:         boolPtr(false),
		MaxSteps:               1,
		MaxConsecutiveFailures: 1,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopMaxConsecutiveFailures {
		t.Fatalf("预期前台包监控异常触发连续失败停止，实际: %s", report.StopReason)
	}
	if report.StepsFailed != 1 {
		t.Fatalf("预期记录 1 次失败，实际: %d", report.StepsFailed)
	}
	if driver.clickCount != 0 {
		t.Fatalf("前台包获取失败时不应继续执行点击，实际: %d", driver.clickCount)
	}
}

func TestRunnerPreflightCheckFailed(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`},
		envResult: &common.EnvironmentCheckResult{
			ADBReady:        false,
			DeviceReady:     false,
			PageSourceReady: false,
			UIAReady:        false,
			PageSourceType:  "uia",
			Detail:          "adb unavailable",
		},
		envErr: errors.New("adb unavailable"),
	}

	runner, err := NewRunner(decider, driver, Config{MaxSteps: 1, StepInterval: 0, KeepStepRecords: true, StopOnCrash: true, StopOnANR: true})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, runErr := runner.Run(context.Background())
	if runErr != nil {
		t.Fatalf("预期返回报告而不是 error: %v", runErr)
	}
	if report.StopReason != StopPreflightFailed {
		t.Fatalf("预期前置检测失败停止，实际: %s", report.StopReason)
	}
	if report.PreflightError == "" {
		t.Fatalf("预期记录前置检测错误原因")
	}
	if report.Preflight == nil || report.Preflight.Detail != "adb unavailable" {
		t.Fatalf("预期记录前置检测明细")
	}
}

func TestRunnerPreflightPageSourceUnavailable(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`},
		envResult: &common.EnvironmentCheckResult{
			ADBReady:        true,
			DeviceReady:     true,
			PageSourceReady: false,
			UIAReady:        false,
			PageSourceType:  "uia",
			Detail:          "页面源不可用",
		},
		envErr: errors.New("页面源不可用"),
	}

	runner, _ := NewRunner(decider, driver, Config{MaxSteps: 1, StepInterval: 0, KeepStepRecords: true, StopOnCrash: true, StopOnANR: true})
	report, _ := runner.Run(context.Background())
	if report.StopReason != StopPreflightFailed {
		t.Fatalf("预期前置检测失败停止，实际: %s", report.StopReason)
	}
	if report.Preflight == nil || report.Preflight.PageSourceReady {
		t.Fatalf("预期记录页面源未就绪")
	}
}

func TestRunnerPreflightUIASessionUnavailable(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`},
		envResult: &common.EnvironmentCheckResult{
			ADBReady:        true,
			DeviceReady:     true,
			PageSourceReady: true,
			UIAReady:        false,
			PageSourceType:  "uia",
			Detail:          "uia 会话不可用",
		},
		envErr: errors.New("uia 会话不可用"),
	}

	runner, _ := NewRunner(decider, driver, Config{MaxSteps: 1, StepInterval: 0, KeepStepRecords: true, StopOnCrash: true, StopOnANR: true})
	report, _ := runner.Run(context.Background())
	if report.StopReason != StopPreflightFailed {
		t.Fatalf("预期前置检测失败停止，实际: %s", report.StopReason)
	}
	if report.Preflight == nil || report.Preflight.UIAReady {
		t.Fatalf("预期记录 UIA 会话未就绪")
	}
}

func TestResolveStepDelayPrefersActionThrottle(t *testing.T) {
	runner, err := NewRunner(&fakeDecider{}, &fakeDriver{pageSource: &fakePageSource{xml: `<node/>`}}, Config{StepInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	cmd := &types.ActionCommand{Throttle: 500, WaitTime: 200}
	d := runner.resolveStepDelay(cmd)
	if d != 500*time.Millisecond {
		t.Fatalf("预期取动作节流 500ms，实际: %s", d)
	}
}

func TestTryRecoverOnConsecutiveFailures(t *testing.T) {
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node/>`}}
	runner, err := NewRunner(&failingDecider{}, driver, Config{
		PackageName:             "com.example.app",
		MaxSteps:                4,
		StepInterval:            0,
		MaxConsecutiveFailures:  10,
		FailureRecoveryInterval: 2,
		StopOnCrash:             true,
		StopOnANR:               true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, runErr := runner.Run(context.Background())
	if runErr != nil {
		t.Fatalf("运行 monkey 失败: %v", runErr)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("预期执行完成，实际: %s", report.StopReason)
	}
	if driver.activateCount < 2 {
		t.Fatalf("预期在连续失败时触发恢复动作，实际次数: %d", driver.activateCount)
	}
}

func TestPickWeightedCandidate(t *testing.T) {
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node/>`}}
	c1 := &types.ActionCommand{Act: types.BACK}
	c2 := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0, 0, 10, 10)}
	decider := &weightedDecider{
		candidates: []WeightedCandidate{
			{Command: c1, Weight: 0},
			{Command: c2, Weight: 10},
		},
	}

	runner, err := NewRunner(decider, driver, Config{StepInterval: 0, MaxSteps: 1, StopOnCrash: true, StopOnANR: true})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}
	runner.rng = rand.New(rand.NewSource(1))

	cmd, cmdErr := runner.nextCommand("MainActivity", session.ActionInput{XMLDescOfGuiTree: "<node/>"})
	if cmdErr != nil {
		t.Fatalf("获取加权动作失败: %v", cmdErr)
	}
	if cmd == nil || cmd.Act != types.CLICK {
		t.Fatalf("预期按权重选中 CLICK，实际: %+v", cmd)
	}
}

func TestPickWeightedCandidateFallbackFirstNonNil(t *testing.T) {
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node/>`}}
	c1 := &types.ActionCommand{Act: types.BACK}
	decider := &weightedDecider{
		candidates: []WeightedCandidate{
			{Command: nil, Weight: 10},
			{Command: c1, Weight: 0},
		},
	}

	runner, err := NewRunner(decider, driver, Config{StepInterval: 0, MaxSteps: 1, StopOnCrash: true, StopOnANR: true})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	cmd, cmdErr := runner.nextCommand("MainActivity", session.ActionInput{XMLDescOfGuiTree: "<node/>"})
	if cmdErr != nil {
		t.Fatalf("获取加权动作失败: %v", cmdErr)
	}
	if cmd == nil || cmd.Act != types.BACK {
		t.Fatalf("预期兜底返回 BACK，实际: %+v", cmd)
	}
}

func TestResolvePageNameDefault(t *testing.T) {
	xml := `<node class="com.demo.MainActivity"/>`
	page := ResolvePageName(xml, nil)
	if page != "com.demo.MainActivity" {
		t.Fatalf("默认解析页面名错误: %s", page)
	}
}

func TestResolvePageNameWithCustomResolver(t *testing.T) {
	xml := `<node class="com.demo.MainActivity"/>`
	page := ResolvePageName(xml, func(_ string) string { return "CustomPage" })
	if page != "CustomPage" {
		t.Fatalf("自定义解析页面名错误: %s", page)
	}
}

func TestRunnerUsesGojaTransformedPageInfoInWholeChain(t *testing.T) {
	decider := &transformingDecider{
		fakeDecider: fakeDecider{
			commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}},
		},
		pageName: "Goja.Custom.Page",
		xml:      `<node class="Goja.Custom.Page"/>`,
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:          1,
		StepInterval:      0,
		KeepStepRecords:   true,
		CaptureScreenshot: true,
		StopOnCrash:       true,
		StopOnANR:         true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if decider.lastPage != "Goja.Custom.Page" {
		t.Fatalf("预期决策链路使用 goja 页面名，实际: %s", decider.lastPage)
	}
	if decider.lastXML != `<node class="Goja.Custom.Page"/>` {
		t.Fatalf("预期决策链路使用 goja 页面 xml，实际: %s", decider.lastXML)
	}
	if report.PageVisitCount["Goja.Custom.Page"] != 1 {
		t.Fatalf("预期统计链路使用 goja 页面名，实际: %+v", report.PageVisitCount)
	}
	if len(report.Records) != 1 || report.Records[0].PageName != "Goja.Custom.Page" {
		t.Fatalf("预期记录链路使用 goja 页面名，实际: %+v", report.Records)
	}
	if !decider.sawScreenshot {
		t.Fatalf("预期 transformPage 能收到截图输入")
	}
}

func TestRunnerUsesCurrentActivityAsPageNameWhenUIA(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:      &fakePageSource{xml: `<node class="android.widget.FrameLayout"/>`},
		currentActivity: "com.demo.LoginActivity",
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:        1,
		StepInterval:    0,
		PageSourceType:  "uia",
		KeepStepRecords: true,
		StopOnCrash:     true,
		StopOnANR:       true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if decider.lastPage != "com.demo.LoginActivity" {
		t.Fatalf("预期使用当前 Activity 作为页面名，实际: %s", decider.lastPage)
	}
	if report.PageVisitCount["com.demo.LoginActivity"] != 1 {
		t.Fatalf("预期使用 Activity 名参与页面统计，实际: %+v", report.PageVisitCount)
	}
}

func boolPtr(v bool) *bool { return &v }
