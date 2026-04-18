package monkey

import (
	"context"
	"fmt"
	"strings"
	"time"
	"trek/internal/engine/core/types"
	"trek/pkg/driver/common"
	"trek/pkg/session"
)

const (
	defaultMaxSteps                     = 300
	defaultMaxDuration                  = 10 * time.Minute
	defaultStepInterval                 = 300 * time.Millisecond
	defaultMaxConsecutiveFailures       = 8
	defaultLongClickDuration            = 800 * time.Millisecond
	defaultScrollDuration               = 350 * time.Millisecond
	defaultScrollSteps            int64 = 20
	defaultScrollRepeat                 = 3
	defaultPageSourceType               = "uia"
)

// StopReason 表示 Monkey 运行停止原因。
type StopReason string

const (
	StopCompleted              StopReason = "completed"
	StopContextCanceled        StopReason = "context_canceled"
	StopTimeout                StopReason = "timeout"
	StopMaxConsecutiveFailures StopReason = "max_consecutive_failures"
	StopCrashDetectedLogcat    StopReason = "crash_logcat"
	StopANRDetectedLogcat      StopReason = "anr_logcat"

	// 兼容常量：保留旧名称，指向更精确的新原因。
	StopCrashDetected StopReason = StopCrashDetectedLogcat
	StopANRDetected   StopReason = StopANRDetectedLogcat
)

// PageNameResolver 从 XML 中提取页面名。
type PageNameResolver func(xml string) string

// Config 是 Smart Monkey Runner 配置。
type Config struct {
	PackageName            string
	AutoStartOnRun         *bool
	MaxSteps               int
	MaxDuration            time.Duration
	StepInterval           time.Duration
	MaxConsecutiveFailures int
	PageSourceType         string
	CaptureScreenshot      bool
	LongClickDuration      time.Duration
	ScrollDuration         time.Duration
	ScrollSteps            int64
	ScrollRepeat           int
	StopOnCrash            bool
	StopOnANR              bool
	KeepStepRecords        bool
	PageNameResolver       PageNameResolver
}

// StepRecord 是每一步执行记录。
type StepRecord struct {
	Step       int
	PageName   string
	Action     string
	DurationMs int64
	Err        string
}

// Report 是 Monkey 运行报告。
type Report struct {
	StartedAt           time.Time
	FinishedAt          time.Time
	DurationMs          int64
	StopReason          StopReason
	StepsPlanned        int
	StepsTotal          int
	StepsSucceeded      int
	StepsFailed         int
	ConsecutiveFailures int
	ActionCount         map[string]int
	PageVisitCount      map[string]int
	Records             []StepRecord
}

// Decider 是动作决策接口，*session.Session 可直接满足。
type Decider interface {
	NextActionWithInput(pageName string, input session.ActionInput) (*types.ActionCommand, error)
}

// Runner 执行 Smart Monkey 真机闭环。
type Runner struct {
	decider Decider
	driver  common.IDriver
	cfg     Config
}

// NewRunner 创建 Monkey Runner。
func NewRunner(decider Decider, driver common.IDriver, cfg Config) (*Runner, error) {
	if decider == nil {
		return nil, fmt.Errorf("decider 不能为空")
	}
	if driver == nil {
		return nil, fmt.Errorf("driver 不能为空")
	}

	cfg = normalizeConfig(cfg)
	return &Runner{decider: decider, driver: driver, cfg: cfg}, nil
}

// Run 启动闭环执行，返回运行报告。
func (r *Runner) Run(ctx context.Context) (*Report, error) {
	pageSource := r.driver.GetPageSource(r.cfg.PageSourceType)
	if pageSource == nil {
		return nil, fmt.Errorf("页面源不可用: %s", r.cfg.PageSourceType)
	}

	if pkg := strings.TrimSpace(r.cfg.PackageName); pkg != "" && isAutoStartOnRunEnabled(r.cfg) {
		if err := r.driver.StartApp(pkg); err != nil {
			return nil, fmt.Errorf("启动被测应用失败: %w", err)
		}
	}

	_ = r.driver.ClearLogcat()

	report := &Report{
		StartedAt:      time.Now(),
		StepsPlanned:   r.cfg.MaxSteps,
		ActionCount:    make(map[string]int),
		PageVisitCount: make(map[string]int),
	}
	defer func() {
		report.FinishedAt = time.Now()
		report.DurationMs = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
	}()

	deadline := report.StartedAt.Add(r.cfg.MaxDuration)

	for step := 1; step <= r.cfg.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			report.StopReason = StopContextCanceled
			return report, nil
		}
		if time.Now().After(deadline) {
			report.StopReason = StopTimeout
			return report, nil
		}

		stepStart := time.Now()
		record := StepRecord{Step: step}

		xml, err := pageSource.DumpPageSource()
		if err != nil {
			record.Err = err.Error()
			r.markFailed(report, record, stepStart)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.sleepStep(ctx)
			continue
		}

		if r.cfg.StopOnCrash && r.detectCrashBySystem() {
			report.StopReason = StopCrashDetectedLogcat
			record.Err = "检测到系统 crash 信号"
			r.appendRecord(report, record, stepStart)
			return report, nil
		}
		if r.cfg.StopOnANR && r.detectANRBySystem() {
			report.StopReason = StopANRDetectedLogcat
			record.Err = "检测到系统 ANR 信号"
			r.appendRecord(report, record, stepStart)
			return report, nil
		}

		pageName := r.cfg.PageNameResolver(xml)
		record.PageName = pageName
		report.PageVisitCount[pageName]++

		var screenshot []byte
		if r.cfg.CaptureScreenshot {
			screenshot, _ = r.driver.Screenshot()
		}

		cmd, err := r.decider.NextActionWithInput(pageName, session.ActionInput{
			XMLDescOfGuiTree: xml,
			Screenshot:       screenshot,
		})
		if err != nil {
			record.Err = err.Error()
			r.markFailed(report, record, stepStart)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.sleepStep(ctx)
			continue
		}
		if cmd == nil {
			record.Err = "决策返回空动作"
			r.markFailed(report, record, stepStart)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.sleepStep(ctx)
			continue
		}

		record.Action = cmd.Act.String()
		report.ActionCount[record.Action]++
		report.StepsTotal++

		if err = r.execute(cmd); err != nil {
			record.Err = err.Error()
			r.markFailed(report, record, stepStart)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.sleepStep(ctx)
			continue
		}

		report.StepsSucceeded++
		report.ConsecutiveFailures = 0
		r.appendRecord(report, record, stepStart)

		r.sleepStep(ctx)
	}

	report.StopReason = StopCompleted
	return report, nil
}

func (r *Runner) execute(cmd *types.ActionCommand) error {
	switch cmd.Act {
	case types.NOP:
		return nil
	case types.CLICK:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return err
		}
		if err = r.driver.Click(pt); err != nil {
			return err
		}
		return r.tryInputText(cmd)
	case types.LONG_CLICK:
		pt, err := centerPoint(cmd.Pos)
		if err != nil {
			return err
		}
		if err = r.driver.LongClick(pt, r.cfg.LongClickDuration.Milliseconds()); err != nil {
			return err
		}
		return r.tryInputText(cmd)
	case types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT:
		return r.swipeByAction(cmd.Pos, cmd.Act)
	case types.SCROLL_BOTTOM_UP_N:
		repeat := r.cfg.ScrollRepeat
		if repeat <= 0 {
			repeat = defaultScrollRepeat
		}
		for i := 0; i < repeat; i++ {
			if err := r.swipeByAction(cmd.Pos, types.SCROLL_BOTTOM_UP); err != nil {
				return err
			}
		}
		return nil
	case types.BACK:
		return r.driver.Back()
	case types.START:
		return r.driver.StartApp(r.cfg.PackageName)
	case types.RESTART:
		return r.driver.RestartApp(r.cfg.PackageName, false)
	case types.CLEAN_RESTART:
		return r.driver.RestartApp(r.cfg.PackageName, true)
	case types.ACTIVATE:
		return r.driver.ActivateApp(r.cfg.PackageName)
	default:
		return fmt.Errorf("暂不支持动作: %s", cmd.Act.String())
	}
}

func (r *Runner) swipeByAction(rect types.Rect, act types.ActionType) error {
	if rect.IsEmpty() {
		return fmt.Errorf("滑动区域为空")
	}

	var start, end types.Point
	switch act {
	case types.SCROLL_BOTTOM_UP:
		start = pointByRatio(rect, 0.5, 0.82)
		end = pointByRatio(rect, 0.5, 0.22)
	case types.SCROLL_TOP_DOWN:
		start = pointByRatio(rect, 0.5, 0.22)
		end = pointByRatio(rect, 0.5, 0.82)
	case types.SCROLL_LEFT_RIGHT:
		start = pointByRatio(rect, 0.22, 0.5)
		end = pointByRatio(rect, 0.82, 0.5)
	case types.SCROLL_RIGHT_LEFT:
		start = pointByRatio(rect, 0.82, 0.5)
		end = pointByRatio(rect, 0.22, 0.5)
	default:
		return fmt.Errorf("不支持的滑动动作: %s", act.String())
	}

	return r.driver.Swipe(start, end, r.cfg.ScrollSteps, r.cfg.ScrollDuration.Milliseconds())
}

func (r *Runner) tryInputText(cmd *types.ActionCommand) error {
	if strings.TrimSpace(cmd.Text) == "" {
		return nil
	}
	return r.driver.InputText(cmd.Text, cmd.Clear)
}

func (r *Runner) markFailed(report *Report, record StepRecord, stepStart time.Time) {
	report.StepsTotal++
	report.StepsFailed++
	report.ConsecutiveFailures++
	r.appendRecord(report, record, stepStart)
}

func (r *Runner) appendRecord(report *Report, record StepRecord, stepStart time.Time) {
	if !r.cfg.KeepStepRecords {
		return
	}
	record.DurationMs = time.Since(stepStart).Milliseconds()
	report.Records = append(report.Records, record)
}

func (r *Runner) sleepStep(ctx context.Context) {
	if r.cfg.StepInterval <= 0 {
		return
	}
	timer := time.NewTimer(r.cfg.StepInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func normalizeConfig(cfg Config) Config {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = defaultMaxSteps
	}
	if cfg.MaxDuration <= 0 {
		cfg.MaxDuration = defaultMaxDuration
	}
	if cfg.StepInterval < 0 {
		cfg.StepInterval = 0
	} else if cfg.StepInterval == 0 {
		cfg.StepInterval = defaultStepInterval
	}
	if cfg.MaxConsecutiveFailures <= 0 {
		cfg.MaxConsecutiveFailures = defaultMaxConsecutiveFailures
	}
	if strings.TrimSpace(cfg.PageSourceType) == "" {
		cfg.PageSourceType = defaultPageSourceType
	}
	if cfg.LongClickDuration <= 0 {
		cfg.LongClickDuration = defaultLongClickDuration
	}
	if cfg.ScrollDuration <= 0 {
		cfg.ScrollDuration = defaultScrollDuration
	}
	if cfg.ScrollSteps <= 0 {
		cfg.ScrollSteps = defaultScrollSteps
	}
	if cfg.ScrollRepeat <= 0 {
		cfg.ScrollRepeat = defaultScrollRepeat
	}
	if cfg.PageNameResolver == nil {
		cfg.PageNameResolver = defaultPageNameResolver
	}
	if !cfg.StopOnCrash && !cfg.StopOnANR {
		cfg.StopOnCrash = true
		cfg.StopOnANR = true
	}
	return cfg
}

func defaultPageNameResolver(xml string) string {
	if classIdx := strings.Index(xml, `class="`); classIdx >= 0 {
		rest := xml[classIdx+len(`class="`):]
		if end := strings.Index(rest, `"`); end > 0 {
			return rest[:end]
		}
	}
	return "UnknownPage"
}

func centerPoint(rect types.Rect) (types.Point, error) {
	if rect.IsEmpty() {
		return types.Point{}, fmt.Errorf("动作坐标为空")
	}
	p := rect.Center()
	return types.Point{X: p.X, Y: p.Y}, nil
}

func pointByRatio(rect types.Rect, rx, ry float64) types.Point {
	return types.Point{
		X: rect.Left + (rect.Right-rect.Left)*rx,
		Y: rect.Top + (rect.Bottom-rect.Top)*ry,
	}
}

func isAutoStartOnRunEnabled(cfg Config) bool {
	if cfg.AutoStartOnRun == nil {
		return true
	}
	return *cfg.AutoStartOnRun
}

func (r *Runner) detectCrashBySystem() bool {
	crash, err := r.driver.CheckCrash(r.cfg.PackageName)
	return err == nil && crash
}

func (r *Runner) detectANRBySystem() bool {
	anr, err := r.driver.CheckANR(r.cfg.PackageName)
	return err == nil && anr
}
