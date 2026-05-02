package monkey

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
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
	results           []session.StepResultInput
	outcomeCalls      int
	lastOutcome       string
	lastOutcomeCtx    enginestate.TraversalContext
	lastOutcomeAction *types.ActionCommand
}

func (o *observingDecider) OnStepResult(result session.StepResultInput) error {
	o.results = append(o.results, result)
	return nil
}

func (o *observingDecider) ObserveTraversalOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
	o.outcomeCalls++
	o.lastOutcome = string(outcome)
	o.lastOutcomeCtx = ctx
	o.lastOutcomeAction = action
	return nil
}

type weightedDecider struct {
	candidates []WeightedCandidate
}

type recoveryAwareDecider struct {
	fakeDecider
	recoveryAction *types.ActionCommand
	recoveryCalls  int
}

type contextAwareRecoveryDecider struct {
	recoveryAwareDecider
	lastContext enginestate.TraversalContext
}

type plannerAwareRecoveryDecider struct {
	contextAwareRecoveryDecider
	memoryCandidates        []candidate.Candidate
	heuristicCandidates     []candidate.Candidate
	llmCandidates           []candidate.Candidate
	algorithmCandidates     []candidate.Candidate
	weightedCandidates      []WeightedCandidate
	persistedFailed         map[string]bool
	persistedSuccess        map[string]bool
	algorithmCalls          int
	memoryCalls             int
	heuristicCalls          int
	llmCalls                int
	lastLLMContext          enginestate.TraversalContext
	selectCalls             int
	selectedRecoveryCmd     *types.ActionCommand
	outcomeCalls            int
	lastOutcomeEscaped      bool
	lastOutcomeContext      enginestate.TraversalContext
	lastOutcomeItem         candidate.Candidate
	enhancementOutcomeCalls int
	lastEnhancementImproved bool
	lastEnhancementContext  enginestate.TraversalContext
	lastEnhancementItem     candidate.Candidate
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

func (d *recoveryAwareDecider) NextBlockRecoveryAction(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	d.recoveryCalls++
	if d.recoveryAction == nil {
		return nil, nil
	}
	return d.recoveryAction, nil
}

func (d *recoveryAwareDecider) NextBlockRecoveryActionWithContext(ctx enginestate.TraversalContext, input session.ActionInput) (*types.ActionCommand, error) {
	return d.NextBlockRecoveryAction(ctx.PageName, input)
}

func (d *contextAwareRecoveryDecider) NextBlockRecoveryActionWithContext(ctx enginestate.TraversalContext, input session.ActionInput) (*types.ActionCommand, error) {
	d.recoveryCalls++
	d.lastContext = ctx
	if d.recoveryAction == nil {
		return nil, nil
	}
	return d.recoveryAction, nil
}

func (d *plannerAwareRecoveryDecider) BuildMemoryRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	d.memoryCalls++
	return d.memoryCandidates, nil
}

func (d *plannerAwareRecoveryDecider) BuildAlgorithmCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	d.algorithmCalls++
	return d.algorithmCandidates, nil
}

func (d *plannerAwareRecoveryDecider) BuildHeuristicRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	d.heuristicCalls++
	return d.heuristicCandidates, nil
}

func (d *plannerAwareRecoveryDecider) BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	d.llmCalls++
	d.lastLLMContext = ctx
	return d.llmCandidates, nil
}

func (d *plannerAwareRecoveryDecider) NextWeightedActionsWithInput(pageName string, input session.ActionInput) ([]WeightedCandidate, error) {
	if len(d.weightedCandidates) > 0 {
		return d.weightedCandidates, nil
	}
	cmd, err := d.NextActionWithInput(pageName, input)
	if err != nil || cmd == nil {
		return nil, err
	}
	return []WeightedCandidate{{Command: cmd, Weight: 1}}, nil
}

func (d *plannerAwareRecoveryDecider) BuildKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	result := make(map[string]bool, len(d.persistedFailed))
	for key, value := range d.persistedFailed {
		result[key] = value
	}
	return result, nil
}

func (d *plannerAwareRecoveryDecider) BuildKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	result := make(map[string]bool, len(d.persistedSuccess))
	for key, value := range d.persistedSuccess {
		result[key] = value
	}
	return result, nil
}

func (d *plannerAwareRecoveryDecider) SelectRecoveryAction(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types.ActionCommand, error) {
	d.selectCalls++
	if d.selectedRecoveryCmd == nil {
		return nil, nil
	}
	return d.selectedRecoveryCmd, nil
}

func (d *plannerAwareRecoveryDecider) RecordRecoveryMemoryOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, escaped bool) error {
	d.outcomeCalls++
	d.lastOutcomeEscaped = escaped
	d.lastOutcomeContext = ctx
	d.lastOutcomeItem = item
	return nil
}

func (d *plannerAwareRecoveryDecider) RecordCandidateEnhancementOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, improved bool) error {
	d.enhancementOutcomeCalls++
	d.lastEnhancementImproved = improved
	d.lastEnhancementContext = ctx
	d.lastEnhancementItem = item
	return nil
}

type fakePageSource struct {
	xml  string
	xmls []string
	idx  int
}

func (f *fakePageSource) DumpPageSource() (string, error) {
	if len(f.xmls) > 0 {
		value := f.xmls[f.idx%len(f.xmls)]
		f.idx++
		return value, nil
	}
	return f.xml, nil
}
func (f *fakePageSource) Close() error { return nil }

type fakeDriver struct {
	pageSource       common.IPageSource
	clickCount       int
	lastClickPoint   types.Point
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
	screenRotation   int
	screenRotationErr error
	screenRotationCalls int
	lastSwipeStart   types.Point
	lastSwipeEnd     types.Point
	backCount        int
	envResult        *common.EnvironmentCheckResult
	envErr           error
	crashAfterClick  bool
	anrAfterClick    bool
	currentActivity  string
	currentActErr    error
}

func (f *fakeDriver) Click(point types.Point) error {
	f.clickCount++
	f.lastClickPoint = point
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
	f.lastSwipeStart = startPoint
	f.lastSwipeEnd = endPoint
	return nil
}
func (f *fakeDriver) Pinch(centerPoint types.Point, startDistance float64, endDistance float64, duration int64) error {
	return nil
}
func (f *fakeDriver) TouchEvent(touchList ...common.TouchEvent) error { return nil }
func (f *fakeDriver) Close() error                                    { return nil }
func (f *fakeDriver) Screenshot(ctx context.Context) ([]byte, error)   { return []byte{1}, nil }
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
func (f *fakeDriver) Back() error {
	f.backCount++
	return nil
}
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
func (f *fakeDriver) GetCurrentPackage(ctx context.Context) (string, error) {
	f.currentPkgCalls++
	if f.currentPkgErr != nil {
		return "", f.currentPkgErr
	}
	return f.currentPackage, nil
}
func (f *fakeDriver) GetCurrentActivity(ctx context.Context) (string, error) {
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
func (f *fakeDriver) GetScreenRotation() (int, error) {
	f.screenRotationCalls++
	if f.screenRotationErr != nil {
		return 0, f.screenRotationErr
	}
	return f.screenRotation, nil
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

func TestRunnerNotifiesTraversalOutcomeObserver(t *testing.T) {
	decider := &observingDecider{
		fakeDecider: fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}},
	}
	driver := &fakeDriver{
		pageSource: &fakePageSource{
			xmls: []string{
				`<node class="PageA"/>`, `<node class="PageB"/>`,
			},
		},
	}

	runner, err := NewRunner(decider, driver, Config{
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
	if decider.outcomeCalls != 1 {
		t.Fatalf("预期回调 1 次 traversal outcome，实际: %d", decider.outcomeCalls)
	}
	if decider.lastOutcome != string(traversal.OutcomeNewState) {
		t.Fatalf("预期 outcome=%s，实际: %s", traversal.OutcomeNewState, decider.lastOutcome)
	}
	if decider.lastOutcomeAction == nil || decider.lastOutcomeAction.Act != types.CLICK {
		t.Fatalf("预期记录 CLICK 动作，实际: %+v", decider.lastOutcomeAction)
	}
	if decider.lastOutcomeCtx.Mode != TraversalModeExplore {
		t.Fatalf("预期回调上下文 mode=Explore，实际: %s", decider.lastOutcomeCtx.Mode)
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
	if !strings.HasPrefix(page, pageFingerprintPrefix+":") {
		t.Fatalf("默认应使用 XML 结构指纹解析页面名，实际: %s", page)
	}
}

func TestResolvePageNameWithCustomResolver(t *testing.T) {
	xml := `<node class="com.demo.MainActivity"/>`
	page := ResolvePageName(xml, func(_ string) string { return "CustomPage" })
	if page != "CustomPage" {
		t.Fatalf("自定义解析页面名错误: %s", page)
	}
}

func TestResolvePageNameFingerprintSupportsPocoTree(t *testing.T) {
	xml := `<hierarchy><node name="Root"><node type="Button" name="btn_start"/></node></hierarchy>`
	page := ResolvePageName(xml, nil)
	if !strings.HasPrefix(page, pageFingerprintPrefix+":") {
		t.Fatalf("预期 Poco 树结构生成指纹页面名，实际: %s", page)
	}
	if page != ResolvePageName(xml, nil) {
		t.Fatalf("同一页面源应生成稳定页面名")
	}
}

func TestResolvePageNameFingerprintIgnoresAttributeOrder(t *testing.T) {
	xmlA := `<hierarchy><node type="Button" name="btn_start"/></hierarchy>`
	xmlB := `<hierarchy><node name="btn_start" type="Button"/></hierarchy>`
	if ResolvePageName(xmlA, nil) != ResolvePageName(xmlB, nil) {
		t.Fatalf("属性顺序不同不应影响页面指纹")
	}
}

func TestResolvePageNameFingerprintIgnoresGenericContentAttrs(t *testing.T) {
	xmlA := `<hierarchy><node label="start" widget="button"/></hierarchy>`
	xmlB := `<hierarchy><node label="setting" widget="button"/></hierarchy>`
	if ResolvePageName(xmlA, nil) != ResolvePageName(xmlB, nil) {
		t.Fatalf("内容属性变化不应影响结构页面指纹")
	}
}

func TestResolvePageNameFingerprintUsesTreeStructure(t *testing.T) {
	xmlA := `<hierarchy><node widget="button"/></hierarchy>`
	xmlB := `<hierarchy><node widget="button"><node widget="image"/></node></hierarchy>`
	if ResolvePageName(xmlA, nil) == ResolvePageName(xmlB, nil) {
		t.Fatalf("树结构变化应影响页面指纹")
	}
}

func TestResolvePageNameFingerprintUsesInteractionCapability(t *testing.T) {
	xmlA := `<hierarchy><node widget="button" clickable="true"/></hierarchy>`
	xmlB := `<hierarchy><node widget="button" clickable="false"/></hierarchy>`
	if ResolvePageName(xmlA, nil) == ResolvePageName(xmlB, nil) {
		t.Fatalf("交互能力变化应影响结构页面指纹")
	}
}

func TestResolvePageNameFingerprintIgnoresRuntimeAttrs(t *testing.T) {
	xmlA := `<hierarchy><node name="start" index="0" focused="false" window-id="10"/></hierarchy>`
	xmlB := `<hierarchy><node name="start" index="3" focused="true" window-id="99"/></hierarchy>`
	if ResolvePageName(xmlA, nil) != ResolvePageName(xmlB, nil) {
		t.Fatalf("运行态抖动属性变化不应影响页面指纹")
	}
}

func TestResolvePageNameByStrategyStructureFingerprintIgnoresActivity(t *testing.T) {
	xml := `<hierarchy><node widget="button"/></hierarchy>`
	page := ResolvePageNameByStrategy(xml, nil, PageNameStrategyStructureFingerprint, "poco", "com.unity3d.player")
	if !strings.HasPrefix(page, pageFingerprintPrefix+":") {
		t.Fatalf("结构指纹策略不应返回 Activity，实际: %s", page)
	}
}

func TestResolvePageNameByStrategyUIAActivityFirstUsesActivity(t *testing.T) {
	xml := `<hierarchy><node widget="button"/></hierarchy>`
	page := ResolvePageNameByStrategy(xml, nil, PageNameStrategyUIAActivityFirst, "uia", "com.demo.MainActivity")
	if page != "com.demo.MainActivity" {
		t.Fatalf("UIA Activity 优先策略应返回 Activity，实际: %s", page)
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

func TestRunnerUsesXMLPageNameWhenStrategyXMLOnly(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:      &fakePageSource{xml: `<node class="com.demo.FromXML"/>`},
		currentActivity: "com.demo.FromActivity",
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:         1,
		StepInterval:     0,
		PageSourceType:   "uia",
		PageNameStrategy: PageNameStrategyXMLOnly,
		KeepStepRecords:  true,
		StopOnCrash:      true,
		StopOnANR:        true,
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
	if !strings.HasPrefix(decider.lastPage, pageFingerprintPrefix+":") {
		t.Fatalf("预期 xml_only 使用 XML 结构指纹页面名，实际: %s", decider.lastPage)
	}
}

func TestRunnerUsesXMLPageNameWhenStrategyStructureFingerprint(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:      &fakePageSource{xml: `<hierarchy><node widget="button"/></hierarchy>`},
		currentActivity: "com.demo.FromActivity",
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:         1,
		StepInterval:     0,
		PageSourceType:   "uia",
		PageNameStrategy: PageNameStrategyStructureFingerprint,
		KeepStepRecords:  true,
		StopOnCrash:      true,
		StopOnANR:        true,
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
	if !strings.HasPrefix(decider.lastPage, pageFingerprintPrefix+":") {
		t.Fatalf("预期 structure_fingerprint 使用页面树结构指纹，实际: %s", decider.lastPage)
	}
}

func TestRunnerUsesUnknownWhenStrategyActivityOnlyAndNoActivity(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0, 0, 100, 100)}}}
	driver := &fakeDriver{
		pageSource:    &fakePageSource{xml: `<node class="com.demo.FromXML"/>`},
		currentActErr: errors.New("activity unavailable"),
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:         1,
		StepInterval:     0,
		PageSourceType:   "uia",
		PageNameStrategy: PageNameStrategyActivityOnly,
		KeepStepRecords:  true,
		StopOnCrash:      true,
		StopOnANR:        true,
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
	if decider.lastPage != "UnknownPage" {
		t.Fatalf("预期 activity_only 失败时回退 UnknownPage，实际: %s", decider.lastPage)
	}
}

func TestNormalizePocoScrollCommandFallbackToAncestorBounds(t *testing.T) {
	decider := &fakeDecider{}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<hierarchy/>`}}
	runner, err := NewRunner(decider, driver, Config{
		PageSourceType: "poco",
		StepInterval:   0,
		StopOnCrash:    true,
		StopOnANR:      true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	cmd := &types.ActionCommand{
		Act:        types.SCROLL_BOTTOM_UP,
		Pos:        *types.NewRect(0, 0, 0, 0),
		WidgetInfo: `Widget{path:/hierarchy/node/node, bounds:[0.000,0.000,0.000,0.000], actions:[SCROLL_BOTTOM_UP]}`,
	}
	xml := `<hierarchy bounds="[0,0][1,1]"><node bounds="[0,0][1,1]"><node bounds="[0,0][0,0]"/></node></hierarchy>`
	runner.normalizePocoScrollCommand(1, cmd, xml)
	if cmd.Pos.IsEmpty() {
		t.Fatalf("预期回退后应有可用滑动区域，实际: %s", cmd.Pos.String())
	}
	if cmd.Pos.Left != 0 || cmd.Pos.Top != 0 || cmd.Pos.Right != 1 || cmd.Pos.Bottom != 1 {
		t.Fatalf("预期回退到父节点区域 [0,0,1,1]，实际: %s", cmd.Pos.String())
	}
}

func TestNormalizePocoScrollCommandSupportsXPathLocator(t *testing.T) {
	decider := &fakeDecider{}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<hierarchy/>`}}
	runner, err := NewRunner(decider, driver, Config{
		PageSourceType: "poco",
		StepInterval:   0,
		StopOnCrash:    true,
		StopOnANR:      true,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	cmd := &types.ActionCommand{
		Act:        types.SCROLL_BOTTOM_UP,
		Pos:        *types.NewRect(0, 0, 0, 0),
		WidgetInfo: `Widget{path:/hierarchy/node/node, xpath:/hierarchy[1]/node[1]/node[1], bounds:[0.000,0.000,0.000,0.000], actions:[SCROLL_BOTTOM_UP]}`,
	}
	xml := `<hierarchy bounds="[0,0][1,1]"><node bounds="[0,0][1,1]"><node bounds="[0.2,0.3][0.7,0.9]"/></node></hierarchy>`
	runner.normalizePocoScrollCommand(1, cmd, xml)
	if cmd.Pos.IsEmpty() {
		t.Fatalf("预期 xpath 回退后应有可用滑动区域，实际: %s", cmd.Pos.String())
	}
	if cmd.Pos.Left != 0.2 || cmd.Pos.Top != 0.3 || cmd.Pos.Right != 0.7 || cmd.Pos.Bottom != 0.9 {
		t.Fatalf("预期命中 xpath 节点区域 [0.2,0.3,0.7,0.9]，实际: %s", cmd.Pos.String())
	}
}

func TestRunnerApplyEffectiveTouchAreaToClick(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0.4, 0.4, 0.6, 0.6)}}}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<hierarchy rotation="0"><node class="MainActivity"/></hierarchy>`}}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:     "com.example.app",
		DeviceSerial:    "192.168.2.198:5555",
		MaxSteps:        1,
		StepInterval:    0,
		KeepStepRecords: true,
		StopOnCrash:     true,
		StopOnANR:       true,
		EffectiveTouchAreas: []EffectiveTouchArea{
			{
				Serial:       "192.168.2.198:5555",
				PackageName:  "com.example.app",
				Orientations: []ScreenOrientation{ScreenOrientationPortrait},
				Range: EffectiveTouchRange{
					Left: 0.04, Top: 0, Right: 1, Bottom: 1,
				},
			},
		},
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

	const expectX = 0.52
	const expectY = 0.5
	if abs(driver.lastClickPoint.X-expectX) > 1e-6 || abs(driver.lastClickPoint.Y-expectY) > 1e-6 {
		t.Fatalf("点击坐标映射不符合预期: got=(%.6f, %.6f) expect=(%.6f, %.6f)", driver.lastClickPoint.X, driver.lastClickPoint.Y, expectX, expectY)
	}
}

func TestRunnerSkipEffectiveTouchAreaWhenSerialMismatch(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0.4, 0.4, 0.6, 0.6)}}}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<hierarchy rotation="0"><node class="MainActivity"/></hierarchy>`}}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:     "com.example.app",
		DeviceSerial:    "serial-A",
		MaxSteps:        1,
		StepInterval:    0,
		KeepStepRecords: true,
		StopOnCrash:     true,
		StopOnANR:       true,
		EffectiveTouchAreas: []EffectiveTouchArea{
			{
				Serial:       "serial-B",
				PackageName:  "com.example.app",
				Orientations: []ScreenOrientation{ScreenOrientationPortrait},
				Range: EffectiveTouchRange{
					Left: 0.04, Top: 0, Right: 1, Bottom: 1,
				},
			},
		},
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

	const expectX = 0.5
	const expectY = 0.5
	if abs(driver.lastClickPoint.X-expectX) > 1e-6 || abs(driver.lastClickPoint.Y-expectY) > 1e-6 {
		t.Fatalf("序列号不匹配时不应映射: got=(%.6f, %.6f) expect=(%.6f, %.6f)", driver.lastClickPoint.X, driver.lastClickPoint.Y, expectX, expectY)
	}
}

func TestRunnerSkipEffectiveTouchAreaWhenOrientationMismatch(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0.4, 0.4, 0.6, 0.6)}}}
	driver := &fakeDriver{
		pageSource:     &fakePageSource{xml: `<hierarchy rotation="1"><node class="MainActivity"/></hierarchy>`},
		screenRotation: 1,
	}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:          "com.example.app",
		DeviceSerial:         "serial-A",
		MaxSteps:             1,
		StepInterval:         0,
		KeepStepRecords:      true,
		StopOnCrash:          true,
		StopOnANR:            true,
		EffectiveTouchAreas: []EffectiveTouchArea{
			{
				Serial:       "serial-A",
				PackageName:  "com.example.app",
				Orientations: []ScreenOrientation{ScreenOrientationPortrait},
				Range: EffectiveTouchRange{
					Left: 0.04, Top: 0, Right: 1, Bottom: 1,
				},
			},
		},
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

	const expectX = 0.5
	const expectY = 0.5
	if abs(driver.lastClickPoint.X-expectX) > 1e-6 || abs(driver.lastClickPoint.Y-expectY) > 1e-6 {
		t.Fatalf("朝向不匹配时不应映射: got=(%.6f, %.6f) expect=(%.6f, %.6f)", driver.lastClickPoint.X, driver.lastClickPoint.Y, expectX, expectY)
	}
}

func TestRunnerApplyEffectiveTouchAreaUsesCachedOrientationMonitor(t *testing.T) {
	decider := &fakeDecider{commands: []*types.ActionCommand{{Act: types.CLICK, Pos: *types.NewRect(0.4, 0.4, 0.6, 0.6)}}}
	driver := &fakeDriver{
		pageSource:      &fakePageSource{xml: `<hierarchy rotation="1"><node class="MainActivity"/></hierarchy>`},
		screenRotation:  0,
		screenRotationErr: errors.New("rotation unavailable during execute"),
	}

	runner, err := NewRunner(decider, driver, Config{
		PackageName:                 "com.example.app",
		DeviceSerial:                "serial-A",
		MaxSteps:                    1,
		StepInterval:                0,
		KeepStepRecords:             true,
		StopOnCrash:                 true,
		StopOnANR:                   true,
		OrientationMonitorInterval:  10 * time.Millisecond,
		EffectiveTouchAreas: []EffectiveTouchArea{
			{
				Serial:       "serial-A",
				PackageName:  "com.example.app",
				Orientations: []ScreenOrientation{ScreenOrientationPortrait},
				Range: EffectiveTouchRange{
					Left: 0.04, Top: 0, Right: 1, Bottom: 1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}

	runner.orientationMonitor = &screenOrientationMonitor{
		orientation: ScreenOrientationPortrait,
		updated:     true,
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}

	const expectX = 0.52
	const expectY = 0.5
	if abs(driver.lastClickPoint.X-expectX) > 1e-6 || abs(driver.lastClickPoint.Y-expectY) > 1e-6 {
		t.Fatalf("预期命中缓存朝向并完成映射: got=(%.6f, %.6f) expect=(%.6f, %.6f)", driver.lastClickPoint.X, driver.lastClickPoint.Y, expectX, expectY)
	}
}

func TestRunnerDetectNoChangeScrollAndTriggerRecovery(t *testing.T) {
	decider := &recoveryAwareDecider{
		fakeDecider: fakeDecider{
			commands: []*types.ActionCommand{
				{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)},
			},
		},
		recoveryAction: &types.ActionCommand{Act: types.BACK},
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               5,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 3,
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
	if decider.recoveryCalls == 0 {
		t.Fatalf("预期触发一次 block recovery")
	}
	if driver.backCount == 0 {
		t.Fatalf("预期执行恢复 BACK 动作")
	}
}

func TestRunnerNoRecoveryForNoChangeClick(t *testing.T) {
	decider := &recoveryAwareDecider{
		fakeDecider: fakeDecider{
			commands: []*types.ActionCommand{
				{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
			},
		},
		recoveryAction: &types.ActionCommand{Act: types.BACK},
	}
	driver := &fakeDriver{pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`}}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               4,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 3,
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
	if decider.recoveryCalls != 0 {
		t.Fatalf("CLICK 无变化不应触发 block recovery，实际调用: %d", decider.recoveryCalls)
	}
	if driver.backCount != 0 {
		t.Fatalf("CLICK 无变化不应执行 BACK，实际: %d", driver.backCount)
	}
}

func TestRunnerDetectTwoStatePingPongAndTriggerRecovery(t *testing.T) {
	decider := &recoveryAwareDecider{
		fakeDecider: fakeDecider{
			commands: []*types.ActionCommand{
				{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
			},
		},
		recoveryAction: &types.ActionCommand{Act: types.BACK},
	}
	// 调用顺序：before1,after1,before2,after2,...
	// 构造成 after 序列 A,B,A,B，且每步 before!=after，触发两状态往返检测。
	pageSource := &fakePageSource{
		xmls: []string{
			`<node class="PageB"/>`, `<node class="PageA"/>`,
			`<node class="PageA"/>`, `<node class="PageB"/>`,
			`<node class="PageB"/>`, `<node class="PageA"/>`,
			`<node class="PageA"/>`, `<node class="PageB"/>`,
			`<node class="PageB"/>`, `<node class="PageA"/>`,
			`<node class="PageA"/>`, `<node class="PageB"/>`,
		},
	}
	driver := &fakeDriver{pageSource: pageSource}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               7,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 3,
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
	if decider.recoveryCalls == 0 {
		t.Fatalf("预期两状态往返触发 block recovery")
	}
	if driver.backCount == 0 {
		t.Fatalf("预期执行恢复 BACK 动作")
	}
}

func TestRunnerReportContainsCooldownStats(t *testing.T) {
	decider := &recoveryAwareDecider{
		fakeDecider: fakeDecider{
			commands: []*types.ActionCommand{
				{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)},
			},
		},
		recoveryAction: &types.ActionCommand{Act: types.BACK},
	}
	// 顺序：
	// step1 before=A after=A
	// step2 before=A after=A -> 进入 SuspectBlocked
	// step3 before=A after=A -> 进入 Recover（pending）
	// step4 before=A after=B -> 执行恢复动作并进入 Cooldown
	// step5 before=B after=B -> 命中一次 Cooldown 步进
	driver := &fakeDriver{
		pageSource: &fakePageSource{
			xmls: []string{
				`<node class="PageA"/>`, `<node class="PageA"/>`,
				`<node class="PageA"/>`, `<node class="PageA"/>`,
				`<node class="PageA"/>`, `<node class="PageA"/>`,
				`<node class="PageA"/>`, `<node class="PageB"/>`,
				`<node class="PageB"/>`, `<node class="PageB"/>`,
			},
		},
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               5,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 2,
		RecoveryCooldownSteps:  2,
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
	if report.RecoveryCooldownEnterCount <= 0 {
		t.Fatalf("预期记录 cooldown 进入次数，实际: %d", report.RecoveryCooldownEnterCount)
	}
	if report.RecoveryCooldownStepCount <= 0 {
		t.Fatalf("预期记录 cooldown 步进命中次数，实际: %d", report.RecoveryCooldownStepCount)
	}
}

func TestRunnerRecoveryPlannerUsesSelectorWhenAvailable(t *testing.T) {
	decider := &plannerAwareRecoveryDecider{
		selectedRecoveryCmd: &types.ActionCommand{Act: types.BACK},
		memoryCandidates: []candidate.Candidate{
			{Command: &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}, Source: candidate.SourceMemory},
		},
	}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xmls: []string{
			`<node class="PageA"/>`,
			`<node class="PageB"/>`,
		}},
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:               5,
		StepInterval:           0,
		KeepStepRecords:        true,
		StopOnCrash:            true,
		StopOnANR:              true,
		BlockNoChangeThreshold: 2,
	})
	if err != nil {
		t.Fatalf("创建 runner 失败: %v", err)
	}
	// 直接标记两次阻塞，进入 recover pending。
	runner.handleBlockDetected(blockReasonSamePageNoChange)
	runner.handleBlockDetected(blockReasonSamePageNoChange)

	report, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("运行 monkey 失败: %v", err)
	}
	if report.StopReason != StopCompleted {
		t.Fatalf("停止原因错误: %s", report.StopReason)
	}
	if decider.selectCalls == 0 {
		t.Fatalf("预期调用 SelectRecoveryAction")
	}
	if driver.backCount == 0 {
		t.Fatalf("预期按 selector 结果执行 BACK")
	}
}

func TestRunnerExploreUsesTraversalCandidatesBeforeEnhancement(t *testing.T) {
	decider := &plannerAwareRecoveryDecider{
		contextAwareRecoveryDecider: contextAwareRecoveryDecider{
			recoveryAwareDecider: recoveryAwareDecider{
				fakeDecider: fakeDecider{
					commands: []*types.ActionCommand{
						{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
					},
				},
			},
		},
		algorithmCandidates: []candidate.Candidate{
			{Command: &types.ActionCommand{Act: types.BACK}, Source: candidate.SourceAlgorithm, Confidence: 0.7},
		},
		selectedRecoveryCmd: &types.ActionCommand{Act: types.BACK},
	}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xmls: []string{
			`<node class="PageA"/>`,
			`<node class="PageB"/>`,
		}},
	}

	runner, err := NewRunner(decider, driver, Config{
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
	if decider.algorithmCalls == 0 {
		t.Fatalf("预期探索阶段调用算法候选提供器")
	}
	if decider.selectCalls == 0 {
		t.Fatalf("预期探索阶段调用候选选择器")
	}
	if driver.backCount == 0 {
		t.Fatalf("预期探索阶段融合后执行 BACK")
	}
}

func TestRunnerExploreLLMEnhancementEnabled(t *testing.T) {
	enabled := true
	decider := &plannerAwareRecoveryDecider{
		contextAwareRecoveryDecider: contextAwareRecoveryDecider{
			recoveryAwareDecider: recoveryAwareDecider{
				fakeDecider: fakeDecider{
					commands: []*types.ActionCommand{
						{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
					},
				},
			},
		},
		weightedCandidates: []WeightedCandidate{
			{Command: &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}, Weight: 0.5},
			{Command: &types.ActionCommand{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)}, Weight: 0.5},
		},
		selectedRecoveryCmd: &types.ActionCommand{Act: types.BACK},
		llmCandidates: []candidate.Candidate{
			{Command: &types.ActionCommand{Act: types.BACK}, Source: candidate.SourceLLM, Confidence: 0.9},
		},
	}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xmls: []string{
			`<node class="PageA"/>`,
			`<node class="PageB"/>`,
		}},
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:                       1,
		StepInterval:                   0,
		KeepStepRecords:                true,
		StopOnCrash:                    true,
		StopOnANR:                      true,
		EnableExploreLLMEnhancement:    &enabled,
		CandidateEnhancementMinStepGap: 1,
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
	if decider.llmCalls == 0 {
		t.Fatalf("预期启用后触发 llm 候选增强")
	}
	if decider.selectCalls == 0 {
		t.Fatalf("预期启用后调用 selector")
	}
	if driver.backCount == 0 {
		t.Fatalf("预期增强后执行 BACK")
	}
	if report.CandidateEnhancementCalls <= 0 || report.CandidateEnhancementSelects <= 0 {
		t.Fatalf("预期报告记录增强调用/命中次数，实际 calls=%d hits=%d", report.CandidateEnhancementCalls, report.CandidateEnhancementSelects)
	}
	if decider.enhancementOutcomeCalls == 0 {
		t.Fatalf("预期写回候选增强结果")
	}
	if !decider.lastEnhancementImproved {
		t.Fatalf("预期增强后 outcome 记为 improved=true")
	}
	if len(decider.lastLLMContext.LocalCandidates) == 0 {
		t.Fatalf("预期增强 llm 上下文包含本地候选摘要")
	}
}

func TestRunnerExploreLLMEnhancementDisabledByDefault(t *testing.T) {
	decider := &plannerAwareRecoveryDecider{
		contextAwareRecoveryDecider: contextAwareRecoveryDecider{
			recoveryAwareDecider: recoveryAwareDecider{
				fakeDecider: fakeDecider{
					commands: []*types.ActionCommand{
						{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
					},
				},
			},
		},
		selectedRecoveryCmd: &types.ActionCommand{Act: types.BACK},
		llmCandidates: []candidate.Candidate{
			{Command: &types.ActionCommand{Act: types.BACK}, Source: candidate.SourceLLM, Confidence: 0.9},
		},
	}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`},
	}

	runner, err := NewRunner(decider, driver, Config{
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
	if decider.llmCalls != 0 || decider.selectCalls != 0 {
		t.Fatalf("默认关闭时不应触发增强，llm=%d selector=%d", decider.llmCalls, decider.selectCalls)
	}
	if driver.backCount != 0 {
		t.Fatalf("默认关闭时不应执行增强 BACK，实际 back=%d", driver.backCount)
	}
	if report.CandidateEnhancementCalls != 0 || report.CandidateEnhancementSelects != 0 {
		t.Fatalf("默认关闭时报告中增强计数应为 0，实际 calls=%d hits=%d", report.CandidateEnhancementCalls, report.CandidateEnhancementSelects)
	}
}

func TestRunnerExploreLLMEnhancementSkipsWhenCandidatesDistinct(t *testing.T) {
	enabled := true
	decider := &plannerAwareRecoveryDecider{
		contextAwareRecoveryDecider: contextAwareRecoveryDecider{
			recoveryAwareDecider: recoveryAwareDecider{
				fakeDecider: fakeDecider{
					commands: []*types.ActionCommand{
						{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)},
					},
				},
			},
		},
		weightedCandidates: []WeightedCandidate{
			{Command: &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}, Weight: 0.9},
			{Command: &types.ActionCommand{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)}, Weight: 0.1},
		},
		selectedRecoveryCmd: &types.ActionCommand{Act: types.BACK},
		llmCandidates: []candidate.Candidate{
			{Command: &types.ActionCommand{Act: types.BACK}, Source: candidate.SourceLLM, Confidence: 0.9},
		},
	}
	driver := &fakeDriver{
		pageSource: &fakePageSource{xml: `<node class="MainActivity"/>`},
	}

	runner, err := NewRunner(decider, driver, Config{
		MaxSteps:                       1,
		StepInterval:                   0,
		KeepStepRecords:                true,
		StopOnCrash:                    true,
		StopOnANR:                      true,
		EnableExploreLLMEnhancement:    &enabled,
		CandidateEnhancementMinStepGap: 1,
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
	if decider.llmCalls != 0 || decider.selectCalls != 0 {
		t.Fatalf("候选区分度高时不应触发增强，llm=%d selector=%d", decider.llmCalls, decider.selectCalls)
	}
	if driver.backCount != 0 {
		t.Fatalf("候选区分度高时不应执行增强 BACK，实际 back=%d", driver.backCount)
	}
	if report.CandidateEnhancementCalls != 0 || report.CandidateEnhancementSelects != 0 {
		t.Fatalf("候选区分度高时增强计数应为 0，实际 calls=%d hits=%d", report.CandidateEnhancementCalls, report.CandidateEnhancementSelects)
	}
}

func TestBlockDetectorUsesStableReasonForScrollNoChange(t *testing.T) {
	detector := newBlockDetector(3, 0, 0, 0)
	cmd := &types.ActionCommand{Act: types.SCROLL_BOTTOM_UP, Pos: *types.NewRect(0, 0, 1, 1)}
	before := session.PageSnapshot{PageName: "MainActivity", XML: `<node class="MainActivity"/>`}
	after := &session.PageSnapshot{PageName: "MainActivity", XML: `<node class="MainActivity"/>`}

	triggered := false
	for i := 0; i < 3; i++ {
		triggered = detector.Observe(cmd, before, after)
	}
	if !triggered {
		t.Fatalf("预期触发 scroll_no_change")
	}
	if detector.LastReason() != blockReasonScrollNoChange {
		t.Fatalf("预期 reason=%s，实际: %s", blockReasonScrollNoChange, detector.LastReason())
	}
}

func TestBlockDetectorDetectsSamePageNoChange(t *testing.T) {
	detector := newBlockDetector(3, 0, 0, 0)
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}
	before := session.PageSnapshot{PageName: "MainActivity", XML: `<node class="MainActivity"/>`}
	after := &session.PageSnapshot{PageName: "MainActivity", XML: `<node class="MainActivity"/>`}

	triggered := false
	for i := 0; i < 3; i++ {
		triggered = detector.Observe(cmd, before, after)
	}
	if !triggered {
		t.Fatalf("预期触发 same_page_no_change")
	}
	if detector.LastReason() != blockReasonSamePageNoChange {
		t.Fatalf("预期 reason=%s，实际: %s", blockReasonSamePageNoChange, detector.LastReason())
	}
}

func TestBlockDetectorDetectsHighVisitLowReward(t *testing.T) {
	detector := newBlockDetector(20, 0, 0, 0)
	cmd := &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}
	after := &session.PageSnapshot{PageName: "PageA", XML: `<node class="PageA"/>`}

	triggered := false
	for i := 0; i < defaultHighVisitThreshold; i++ {
		before := session.PageSnapshot{
			PageName: "PageB",
			XML:      `<node class="PageB_` + strings.Repeat("x", i+1) + `"/>`,
		}
		triggered = detector.Observe(cmd, before, after)
	}
	if !triggered {
		t.Fatalf("预期触发 high_visit_low_reward")
	}
	if detector.LastReason() != blockReasonHighVisitLowGain {
		t.Fatalf("预期 reason=%s，实际: %s", blockReasonHighVisitLowGain, detector.LastReason())
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func boolPtr(v bool) *bool { return &v }
