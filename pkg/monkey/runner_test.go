package monkey

import (
	"context"
	"testing"
	"trek/internal/engine/core/types"
	"trek/pkg/driver/common"
	"trek/pkg/session"
)

type fakeDecider struct {
	commands []*types.ActionCommand
	idx      int
}

func (f *fakeDecider) NextActionWithInput(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	if len(f.commands) == 0 {
		return types.NewActionCommand(), nil
	}
	cmd := f.commands[f.idx%len(f.commands)]
	f.idx++
	return cmd, nil
}

type fakePageSource struct {
	xml string
}

func (f *fakePageSource) DumpPageSource() (string, error) { return f.xml, nil }
func (f *fakePageSource) Close() error                    { return nil }

type fakeDriver struct {
	pageSource common.IPageSource
	clickCount int
	startCount int
	crash      bool
	anr        bool
	clearCnt   int
}

func (f *fakeDriver) Click(point types.Point) error { f.clickCount++; return nil }
func (f *fakeDriver) LongClick(point types.Point, duration int64) error {
	return nil
}
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
func (f *fakeDriver) ActivateApp(packageName string) error            { return nil }
func (f *fakeDriver) InputText(text string, clear bool) error         { return nil }
func (f *fakeDriver) CheckCrash(packageName string) (bool, error)     { return f.crash, nil }
func (f *fakeDriver) CheckANR(packageName string) (bool, error)       { return f.anr, nil }
func (f *fakeDriver) ClearLogcat() error {
	f.clearCnt++
	return nil
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
}

func TestRunnerDetectCrash(t *testing.T) {
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
		t.Fatalf("预期 crash 停止，实际: %s", report.StopReason)
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
}
