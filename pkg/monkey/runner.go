package monkey

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/core/types"
	"trek/logger"
	"trek/pkg/driver/common"
	"trek/pkg/session"

	"github.com/beevik/etree"
)

const (
	defaultMaxSteps                          = 300
	defaultMaxDuration                       = 10 * time.Minute
	defaultStepInterval                      = 300 * time.Millisecond
	defaultMaxConsecutiveFailures            = 8
	defaultFailureRecoveryInterval           = 3
	defaultLongClickDuration                 = 800 * time.Millisecond
	defaultScrollDuration                    = 350 * time.Millisecond
	defaultScrollSteps                 int64 = 20
	defaultScrollRepeat                      = 3
	defaultPageSourceType                    = "uia"
	defaultForegroundMonitorInterval         = 300 * time.Millisecond
	defaultHealthSignalMonitorInterval       = 500 * time.Millisecond
)

// StopReason 表示 Monkey 运行停止原因。
type StopReason string

const (
	StopCompleted              StopReason = "completed"
	StopContextCanceled        StopReason = "context_canceled"
	StopTimeout                StopReason = "timeout"
	StopPreflightFailed        StopReason = "preflight_failed"
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
	PackageName                 string
	DeviceSerial                string
	AutoStartOnRun              *bool
	ActionThrottleEnabled       *bool
	RandomizeThrottle           bool
	EnableFailureRecovery       *bool
	FailureRecoveryInterval     int
	MaxSteps                    int
	MaxDuration                 time.Duration
	StepInterval                time.Duration
	MaxConsecutiveFailures      int
	PageSourceType              string
	CaptureScreenshot           bool
	LongClickDuration           time.Duration
	ScrollDuration              time.Duration
	ScrollSteps                 int64
	ScrollRepeat                int
	StopOnCrash                 bool
	StopOnANR                   bool
	KeepStepRecords             bool
	PageNameResolver            PageNameResolver
	PageNameStrategy            string
	ForegroundMonitorInterval   time.Duration
	HealthSignalMonitorInterval time.Duration
	EffectiveTouchArea          *EffectiveTouchArea
}

type EffectiveTouchArea struct {
	Serial      string
	PackageName string
	Range       EffectiveTouchRange
}

type EffectiveTouchRange struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
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
	Preflight           *common.EnvironmentCheckResult
	PreflightError      string
	StepsPlanned        int
	StepsTotal          int
	StepsSucceeded      int
	StepsFailed         int
	ConsecutiveFailures int
	ActionCount         map[string]int
	PageVisitCount      map[string]int
	OutOfAppRecoveries  int
	Records             []StepRecord
}

// Decider 是动作决策接口，*session.Session 可直接满足。
type Decider interface {
	NextActionWithInput(pageName string, input session.ActionInput) (*types.ActionCommand, error)
}

// PageInfoTransformer 是可选接口：允许决策层按 XML/截图自定义页面名与页面内容。
type PageInfoTransformer interface {
	TransformPageInfoWithInput(pageName string, input session.ActionInput) (session.PageInfo, error)
}

// WeightedCandidate 表示一个带权重的候选动作。
type WeightedCandidate struct {
	Command *types.ActionCommand
	Weight  float64
}

// WeightedDecider 是可选接口，用于返回多候选动作并由 Runner 按权重采样。
type WeightedDecider interface {
	NextWeightedActionsWithInput(pageName string, input session.ActionInput) ([]WeightedCandidate, error)
}

// StepResultObserver 是可选接口，用于接收每步执行后的复盘信息。
type StepResultObserver interface {
	OnStepResult(result session.StepResultInput) error
}

type currentPackageProvider interface {
	GetCurrentPackage() (string, error)
}

type currentActivityProvider interface {
	GetCurrentActivity() (string, error)
}

type foregroundPackageMonitor struct {
	provider currentPackageProvider
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	pkg      string
	err      error
	updated  bool
}

type healthSignalMonitor struct {
	driver      common.IDriver
	packageName string
	interval    time.Duration
	stopCh      chan struct{}
	doneCh      chan struct{}
	mu          sync.RWMutex
	crash       bool
	anr         bool
	updated     bool
}

func newHealthSignalMonitor(driver common.IDriver, packageName string, interval time.Duration) *healthSignalMonitor {
	return &healthSignalMonitor{
		driver:      driver,
		packageName: packageName,
		interval:    interval,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

func (m *healthSignalMonitor) start() {
	m.refresh()
	go func() {
		ticker := time.NewTicker(m.interval)
		defer func() {
			ticker.Stop()
			close(m.doneCh)
		}()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.refresh()
			}
		}
	}()
}

func (m *healthSignalMonitor) stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *healthSignalMonitor) refresh() {
	crash, crashErr := m.driver.CheckCrash(m.packageName)
	anr, anrErr := m.driver.CheckANR(m.packageName)
	m.mu.Lock()
	if crashErr == nil {
		m.crash = crash
	}
	if anrErr == nil {
		m.anr = anr
	}
	m.updated = true
	m.mu.Unlock()
}

func (m *healthSignalMonitor) snapshot() (crash bool, anr bool, updated bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.crash, m.anr, m.updated
}

func newForegroundPackageMonitor(provider currentPackageProvider, interval time.Duration) *foregroundPackageMonitor {
	return &foregroundPackageMonitor{
		provider: provider,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

func (m *foregroundPackageMonitor) start() {
	m.refresh()
	go func() {
		ticker := time.NewTicker(m.interval)
		defer func() {
			ticker.Stop()
			close(m.doneCh)
		}()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.refresh()
			}
		}
	}()
}

func (m *foregroundPackageMonitor) stop() {
	close(m.stopCh)
	<-m.doneCh
}

func (m *foregroundPackageMonitor) refresh() {
	pkg, err := m.provider.GetCurrentPackage()
	m.mu.Lock()
	m.pkg = strings.TrimSpace(pkg)
	m.err = err
	m.updated = true
	m.mu.Unlock()
}

func (m *foregroundPackageMonitor) snapshot() (string, error, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pkg, m.err, m.updated
}

func (m *foregroundPackageMonitor) setCurrentPackage(pkg string) {
	m.mu.Lock()
	m.pkg = strings.TrimSpace(pkg)
	m.err = nil
	m.updated = true
	m.mu.Unlock()
}

// Runner 执行 Smart Monkey 真机闭环。
type Runner struct {
	decider       Decider
	driver        common.IDriver
	cfg           Config
	rng           *rand.Rand
	monitor       *foregroundPackageMonitor
	healthMonitor *healthSignalMonitor
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
	return &Runner{
		decider: decider,
		driver:  driver,
		cfg:     cfg,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// Run 启动闭环执行，返回运行报告。
func (r *Runner) Run(ctx context.Context) (*Report, error) {
	report := &Report{
		StartedAt:      time.Now(),
		StepsPlanned:   r.cfg.MaxSteps,
		ActionCount:    make(map[string]int),
		PageVisitCount: make(map[string]int),
	}
	defer func() {
		report.FinishedAt = time.Now()
		report.DurationMs = report.FinishedAt.Sub(report.StartedAt).Milliseconds()
		logger.Infof("monkey run finished: reason=%s total=%d success=%d failed=%d duration_ms=%d",
			report.StopReason, report.StepsTotal, report.StepsSucceeded, report.StepsFailed, report.DurationMs)
	}()

	logger.Infof("monkey run start: package=%s max_steps=%d max_duration=%s page_source=%s",
		r.cfg.PackageName, r.cfg.MaxSteps, r.cfg.MaxDuration, r.cfg.PageSourceType)

	checkResult, err := r.driver.CheckEnvironment(r.cfg.PageSourceType)
	report.Preflight = checkResult
	if err != nil {
		report.StopReason = StopPreflightFailed
		report.PreflightError = err.Error()
		logger.Errorf("monkey preflight failed: err=%v detail=%+v", err, checkResult)
		return report, nil
	}
	logger.Infof("monkey preflight ok: detail=%+v", checkResult)

	pageSource := r.driver.GetPageSource(r.cfg.PageSourceType)
	if pageSource == nil {
		return nil, fmt.Errorf("页面源不可用: %s", r.cfg.PageSourceType)
	}

	if pkg := strings.TrimSpace(r.cfg.PackageName); pkg != "" && isAutoStartOnRunEnabled(r.cfg) {
		if err := r.driver.StartApp(pkg); err != nil {
			return nil, fmt.Errorf("启动被测应用失败: %w", err)
		}
		logger.Infof("monkey app started: package=%s", pkg)
	}

	_ = r.driver.ClearLogcat()
	r.startForegroundPackageMonitor()
	defer r.stopForegroundPackageMonitor()
	r.startHealthSignalMonitor()
	defer r.stopHealthSignalMonitor()
	deadline := report.StartedAt.Add(r.cfg.MaxDuration)

	for step := 1; step <= r.cfg.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			report.StopReason = StopContextCanceled
			logger.Warnf("monkey canceled by context")
			return report, nil
		}
		if time.Now().After(deadline) {
			report.StopReason = StopTimeout
			logger.Warnf("monkey timeout reached")
			return report, nil
		}

		stepStart := time.Now()
		record := StepRecord{Step: step}

		recovered, recoverErr := r.ensureTargetPackageForeground(step)
		if recoverErr != nil {
			record.Err = recoverErr.Error()
			r.markFailed(report, record, stepStart)
			logger.Warnf("monkey step=%d ensure target package failed: %v", step, recoverErr)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}
		if recovered {
			report.OutOfAppRecoveries++
			record.Err = "检测到已离开被测应用，已自动拉回前台"
			r.appendRecord(report, record, stepStart)
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}

		xml, err := pageSource.DumpPageSource()
		if err != nil {
			record.Err = err.Error()
			r.markFailed(report, record, stepStart)
			logger.Warnf("monkey step=%d dump page source failed: %v", step, err)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.tryRecover(report.ConsecutiveFailures)
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}

		cachedCrash, cachedANR, cachedReady := r.snapshotHealthSignals()
		if r.cfg.StopOnCrash && cachedReady && cachedCrash {
			report.StopReason = StopCrashDetectedLogcat
			record.Err = "检测到系统 crash 信号"
			r.appendRecord(report, record, stepStart)
			logger.Errorf("monkey stop on crash signal at step=%d", step)
			return report, nil
		}
		if r.cfg.StopOnANR && cachedReady && cachedANR {
			report.StopReason = StopANRDetectedLogcat
			record.Err = "检测到系统 ANR 信号"
			r.appendRecord(report, record, stepStart)
			logger.Errorf("monkey stop on anr signal at step=%d", step)
			return report, nil
		}

		var screenshot []byte
		if r.cfg.CaptureScreenshot {
			screenshot, _ = r.driver.Screenshot()
		}
		pageName, xml := r.resolvePageInfo(xml, screenshot)

		beforePage := session.PageSnapshot{
			PageName:   pageName,
			XML:        xml,
			Screenshot: screenshot,
		}

		record.PageName = pageName
		report.PageVisitCount[pageName]++

		cmd, err := r.nextCommand(pageName, session.ActionInput{
			XMLDescOfGuiTree: xml,
			Screenshot:       screenshot,
		})
		if err != nil {
			record.Err = err.Error()
			r.markFailed(report, record, stepStart)
			logger.Warnf("monkey step=%d next command failed: %v", step, err)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.tryRecover(report.ConsecutiveFailures)
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}
		if cmd == nil {
			record.Err = "决策返回空动作"
			r.markFailed(report, record, stepStart)
			logger.Warnf("monkey step=%d got nil command", step)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.tryRecover(report.ConsecutiveFailures)
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}
		r.normalizePocoScrollCommand(step, cmd, xml)
		r.applyEffectiveTouchArea(step, cmd)

		record.Action = cmd.Act.String()
		report.ActionCount[record.Action]++
		report.StepsTotal++

		logger.Infof("monkey step=%d execute cmd={%s}%s%s", step, cmd.DetailLogString(), formatTapPointLog(cmd), formatSwipePointLog(cmd))

		if err = r.execute(cmd); err != nil {
			record.Err = err.Error()
			afterPage := r.capturePageSnapshot(pageSource, pageName)
			crash, anr := r.currentHealthSignals()
			r.notifyStepResult(step, cmd, false, err.Error(), time.Since(stepStart).Milliseconds(), crash, anr, beforePage, afterPage)
			r.markFailed(report, record, stepStart)
			logger.Warnf("monkey step=%d execute action=%s failed: %v", step, cmd.Act.String(), err)
			if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
				report.StopReason = StopMaxConsecutiveFailures
				return report, nil
			}
			r.tryRecover(report.ConsecutiveFailures)
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}

		report.StepsSucceeded++
		report.ConsecutiveFailures = 0
		afterPage := r.capturePageSnapshot(pageSource, pageName)
		crash, anr := r.currentHealthSignals()
		r.notifyStepResult(step, cmd, true, "", time.Since(stepStart).Milliseconds(), crash, anr, beforePage, afterPage)
		r.appendRecord(report, record, stepStart)
		logger.Debugf("monkey step=%d execute action=%s success", step, cmd.Act.String())
		r.sleepStep(ctx, r.resolveStepDelay(cmd))
	}

	report.StopReason = StopCompleted
	return report, nil
}

func (r *Runner) capturePageSnapshot(pageSource common.IPageSource, fallbackPageName string) *session.PageSnapshot {
	if pageSource == nil {
		return nil
	}
	xml, err := pageSource.DumpPageSource()
	if err != nil {
		return nil
	}
	pageName := r.cfg.PageNameResolver(xml)
	if strings.TrimSpace(pageName) == "" {
		pageName = fallbackPageName
	}
	var screenshot []byte
	if r.cfg.CaptureScreenshot {
		screenshot, _ = r.driver.Screenshot()
	}
	nextPageName, nextXML := r.resolvePageInfo(xml, screenshot)
	if strings.TrimSpace(nextPageName) == "" {
		nextPageName = pageName
	}
	return &session.PageSnapshot{
		PageName:   nextPageName,
		XML:        nextXML,
		Screenshot: screenshot,
	}
}

func (r *Runner) notifyStepResult(step int, cmd *types.ActionCommand, success bool, errText string, durationMs int64, crash bool, anr bool, before session.PageSnapshot, after *session.PageSnapshot) {
	observer, ok := r.decider.(StepResultObserver)
	if !ok || observer == nil {
		return
	}
	_ = observer.OnStepResult(session.StepResultInput{
		Step:       step,
		Action:     cmd,
		Success:    success,
		Error:      errText,
		DurationMs: durationMs,
		Crash:      crash,
		ANR:        anr,
		Before:     before,
		After:      after,
	})
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

func (r *Runner) normalizePocoScrollCommand(step int, cmd *types.ActionCommand, xml string) {
	if cmd == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.cfg.PageSourceType), "poco") {
		return
	}
	switch cmd.Act {
	case types.SCROLL_TOP_DOWN, types.SCROLL_BOTTOM_UP, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT, types.SCROLL_BOTTOM_UP_N:
		if cmd.Pos.IsEmpty() {
			if rect, ok := resolvePocoScrollRectFromWidgetPath(cmd.WidgetInfo, xml); ok {
				cmd.Pos = *rect
				logger.Warnf("monkey step=%d action=%s bounds empty under poco, fallback to ancestor rect=%s", step, cmd.Act.String(), cmd.Pos.String())
				return
			}
			cmd.Pos = *types.NewRect(0, 0, 1, 1)
			logger.Warnf("monkey step=%d action=%s bounds empty under poco, fallback to normalized full-screen rect", step, cmd.Act.String())
		}
	}
}

func (r *Runner) applyEffectiveTouchArea(step int, cmd *types.ActionCommand) {
	if cmd == nil || r.cfg.EffectiveTouchArea == nil {
		return
	}
	if !matchesEffectiveTouchScope(r.cfg.EffectiveTouchArea, r.cfg.DeviceSerial, r.cfg.PackageName) {
		return
	}
	switch cmd.Act {
	case types.CLICK, types.LONG_CLICK, types.SCROLL_TOP_DOWN, types.SCROLL_BOTTOM_UP, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT, types.SCROLL_BOTTOM_UP_N:
	default:
		return
	}
	if cmd.Pos.IsEmpty() {
		return
	}
	if !isNormalizedRect(cmd.Pos) {
		return
	}
	oldRect := cmd.Pos
	mapped, ok := mapRectToEffectiveRange(cmd.Pos, r.cfg.EffectiveTouchArea.Range)
	if !ok {
		return
	}
	cmd.Pos = mapped
	logger.Debugf(
		"monkey step=%d apply effective_touch_area scope=%s::%s from=%s to=%s",
		step,
		strings.TrimSpace(r.cfg.EffectiveTouchArea.Serial),
		strings.TrimSpace(r.cfg.EffectiveTouchArea.PackageName),
		oldRect.String(),
		cmd.Pos.String(),
	)
}

var widgetPathRegex = regexp.MustCompile(`path:([^,}]+)`)

func resolvePocoScrollRectFromWidgetPath(widgetInfo string, xml string) (*types.Rect, bool) {
	pathMatch := widgetPathRegex.FindStringSubmatch(widgetInfo)
	if len(pathMatch) < 2 {
		return nil, false
	}
	path := strings.TrimSpace(pathMatch[1])
	if path == "" {
		return nil, false
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		return nil, false
	}
	root := doc.Root()
	if root == nil {
		return nil, false
	}

	// 优先直接定位到目标路径节点。
	current := root.FindElement(path)
	// 兜底：路径里带根节点时，FindElement 可能返回空，尝试补全为绝对路径风格。
	if current == nil {
		normalizedPath := strings.TrimPrefix(path, "/")
		current = root.FindElement(normalizedPath)
	}
	for current != nil {
		if rect, ok := parseRectFromBoundsValue(current.SelectAttrValue("bounds", "")); ok && !rect.IsEmpty() {
			return rect, true
		}
		current = current.Parent()
	}
	return nil, false
}

func parseRectFromBoundsValue(bounds string) (*types.Rect, bool) {
	text := strings.TrimSpace(bounds)
	if text == "" {
		return nil, false
	}
	parts := strings.Split(text, "][")
	if len(parts) != 2 {
		return nil, false
	}
	leftTop := strings.Trim(parts[0], "[]")
	rightBottom := strings.Trim(parts[1], "[]")
	lt := strings.Split(leftTop, ",")
	rb := strings.Split(rightBottom, ",")
	if len(lt) != 2 || len(rb) != 2 {
		return nil, false
	}
	left, err1 := strconv.ParseFloat(strings.TrimSpace(lt[0]), 64)
	top, err2 := strconv.ParseFloat(strings.TrimSpace(lt[1]), 64)
	right, err3 := strconv.ParseFloat(strings.TrimSpace(rb[0]), 64)
	bottom, err4 := strconv.ParseFloat(strings.TrimSpace(rb[1]), 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, false
	}
	return types.NewRect(left, top, right, bottom), true
}

func formatTapPointLog(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	if cmd.Act != types.CLICK && cmd.Act != types.LONG_CLICK {
		return ""
	}
	pt, err := centerPoint(cmd.Pos)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" tap_point=[%.3f,%.3f]", pt.X, pt.Y)
}

func formatSwipePointLog(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	start, end, err := resolveSwipePoints(cmd.Pos, cmd.Act)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" swipe_start=[%.3f,%.3f] swipe_end=[%.3f,%.3f]", start.X, start.Y, end.X, end.Y)
}

func (r *Runner) swipeByAction(rect types.Rect, act types.ActionType) error {
	start, end, err := resolveSwipePoints(rect, act)
	if err != nil {
		return err
	}
	return r.driver.Swipe(start, end, r.cfg.ScrollSteps, r.cfg.ScrollDuration.Milliseconds())
}

func resolveSwipePoints(rect types.Rect, act types.ActionType) (types.Point, types.Point, error) {
	if rect.IsEmpty() {
		return types.Point{}, types.Point{}, fmt.Errorf("滑动区域为空")
	}
	switch act {
	case types.SCROLL_BOTTOM_UP:
		return pointByRatio(rect, 0.5, 0.82), pointByRatio(rect, 0.5, 0.22), nil
	case types.SCROLL_TOP_DOWN:
		return pointByRatio(rect, 0.5, 0.22), pointByRatio(rect, 0.5, 0.82), nil
	case types.SCROLL_LEFT_RIGHT:
		return pointByRatio(rect, 0.22, 0.5), pointByRatio(rect, 0.82, 0.5), nil
	case types.SCROLL_RIGHT_LEFT:
		return pointByRatio(rect, 0.82, 0.5), pointByRatio(rect, 0.22, 0.5), nil
	default:
		return types.Point{}, types.Point{}, fmt.Errorf("不支持的滑动动作: %s", act.String())
	}
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

func (r *Runner) sleepStep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (r *Runner) resolveStepDelay(cmd *types.ActionCommand) time.Duration {
	delay := r.cfg.StepInterval
	if cmd == nil || !isActionThrottleEnabled(r.cfg) {
		return delay
	}

	if cmd.WaitTime > 0 {
		waitDelay := time.Duration(cmd.WaitTime) * time.Millisecond
		if waitDelay > delay {
			delay = waitDelay
		}
	}
	if cmd.Throttle > 0 {
		throttleMs := int64(math.Ceil(float64(cmd.Throttle)))
		throttleDelay := time.Duration(throttleMs) * time.Millisecond
		if throttleDelay > delay {
			delay = throttleDelay
		}
	}

	if r.cfg.RandomizeThrottle && delay > 1*time.Millisecond && r.rng != nil {
		n := r.rng.Int63n(delay.Milliseconds()) + 1
		delay = time.Duration(n) * time.Millisecond
	}
	return delay
}

func (r *Runner) tryRecover(consecutiveFailures int) {
	if !isFailureRecoveryEnabled(r.cfg) {
		return
	}
	if strings.TrimSpace(r.cfg.PackageName) == "" {
		return
	}
	if consecutiveFailures <= 0 || consecutiveFailures%r.cfg.FailureRecoveryInterval != 0 {
		return
	}
	_ = r.driver.ActivateApp(r.cfg.PackageName)
}

func (r *Runner) nextCommand(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	if wd, ok := r.decider.(WeightedDecider); ok {
		candidates, err := wd.NextWeightedActionsWithInput(pageName, input)
		if err != nil {
			return nil, err
		}
		return r.pickWeightedCandidate(candidates), nil
	}
	return r.decider.NextActionWithInput(pageName, input)
}

func (r *Runner) pickWeightedCandidate(candidates []WeightedCandidate) *types.ActionCommand {
	if len(candidates) == 0 {
		return nil
	}

	// 第一层：统计正权重候选总和。
	total := 0.0
	for _, c := range candidates {
		if c.Command != nil && c.Weight > 0 {
			total += c.Weight
		}
	}

	// 第二层：有正权重则按权重随机抽样。
	if total > 0 && r.rng != nil {
		target := r.rng.Float64() * total
		acc := 0.0
		for _, c := range candidates {
			if c.Command == nil || c.Weight <= 0 {
				continue
			}
			acc += c.Weight
			if acc >= target {
				return c.Command
			}
		}
	}

	// 兜底：返回第一个非空动作。
	for _, c := range candidates {
		if c.Command != nil {
			return c.Command
		}
	}
	return nil
}

func normalizeConfig(cfg Config) Config {
	cfg.DeviceSerial = strings.TrimSpace(cfg.DeviceSerial)
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
	if cfg.FailureRecoveryInterval <= 0 {
		cfg.FailureRecoveryInterval = defaultFailureRecoveryInterval
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
	if cfg.ForegroundMonitorInterval <= 0 {
		cfg.ForegroundMonitorInterval = defaultForegroundMonitorInterval
	}
	if cfg.HealthSignalMonitorInterval <= 0 {
		cfg.HealthSignalMonitorInterval = defaultHealthSignalMonitorInterval
	}
	if !cfg.StopOnCrash && !cfg.StopOnANR {
		cfg.StopOnCrash = true
		cfg.StopOnANR = true
	}
	if cfg.EffectiveTouchArea != nil {
		cfg.EffectiveTouchArea.Serial = strings.TrimSpace(cfg.EffectiveTouchArea.Serial)
		cfg.EffectiveTouchArea.PackageName = strings.TrimSpace(cfg.EffectiveTouchArea.PackageName)
		if cfg.EffectiveTouchArea.Range.Right <= cfg.EffectiveTouchArea.Range.Left ||
			cfg.EffectiveTouchArea.Range.Bottom <= cfg.EffectiveTouchArea.Range.Top {
			cfg.EffectiveTouchArea = nil
		}
	}
	return cfg
}

// ResolvePageName 使用 Runner 同款逻辑解析页面名，便于调试和外部调用。
func ResolvePageName(xml string, resolver PageNameResolver) string {
	if resolver == nil {
		resolver = defaultPageNameResolver
	}
	return resolver(xml)
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

func matchesEffectiveTouchScope(area *EffectiveTouchArea, serial string, packageName string) bool {
	if area == nil {
		return false
	}
	areaSerial := strings.TrimSpace(area.Serial)
	if areaSerial != "" && !strings.EqualFold(areaSerial, strings.TrimSpace(serial)) {
		return false
	}
	areaPackage := strings.TrimSpace(area.PackageName)
	if areaPackage != "" && !strings.EqualFold(areaPackage, strings.TrimSpace(packageName)) {
		return false
	}
	return true
}

func isNormalizedRect(rect types.Rect) bool {
	return rect.Left >= 0 && rect.Top >= 0 && rect.Right <= 1 && rect.Bottom <= 1
}

func mapRectToEffectiveRange(rect types.Rect, area EffectiveTouchRange) (types.Rect, bool) {
	width := area.Right - area.Left
	height := area.Bottom - area.Top
	if width <= 0 || height <= 0 {
		return rect, false
	}
	return types.Rect{
		Left:   area.Left + width*rect.Left,
		Top:    area.Top + height*rect.Top,
		Right:  area.Left + width*rect.Right,
		Bottom: area.Top + height*rect.Bottom,
	}, true
}

func isAutoStartOnRunEnabled(cfg Config) bool {
	if cfg.AutoStartOnRun == nil {
		return true
	}
	return *cfg.AutoStartOnRun
}

func isActionThrottleEnabled(cfg Config) bool {
	if cfg.ActionThrottleEnabled == nil {
		return true
	}
	return *cfg.ActionThrottleEnabled
}

func isFailureRecoveryEnabled(cfg Config) bool {
	if cfg.EnableFailureRecovery == nil {
		return true
	}
	return *cfg.EnableFailureRecovery
}

func (r *Runner) detectCrashBySystem() bool {
	crash, err := r.driver.CheckCrash(r.cfg.PackageName)
	return err == nil && crash
}

func (r *Runner) detectANRBySystem() bool {
	anr, err := r.driver.CheckANR(r.cfg.PackageName)
	return err == nil && anr
}

func (r *Runner) ensureTargetPackageForeground(step int) (bool, error) {
	targetPackage := strings.TrimSpace(r.cfg.PackageName)
	if targetPackage == "" {
		return false, nil
	}
	if r.monitor == nil {
		return false, nil
	}
	currentPackage, err, ok := r.monitor.snapshot()
	if !ok {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("获取当前前台应用失败: %w", err)
	}
	if currentPackage == "" || currentPackage == targetPackage {
		return false, nil
	}
	logger.Warnf("monkey step=%d out of target package: current=%s target=%s, activating target app", step, currentPackage, targetPackage)
	if err := r.driver.ActivateApp(targetPackage); err != nil {
		return false, fmt.Errorf("拉回被测应用失败: %w", err)
	}
	r.monitor.setCurrentPackage(targetPackage)
	return true, nil
}

func (r *Runner) startForegroundPackageMonitor() {
	targetPackage := strings.TrimSpace(r.cfg.PackageName)
	if targetPackage == "" {
		return
	}
	provider, ok := r.driver.(currentPackageProvider)
	if !ok || provider == nil {
		return
	}
	monitor := newForegroundPackageMonitor(provider, r.cfg.ForegroundMonitorInterval)
	monitor.start()
	r.monitor = monitor
}

func (r *Runner) stopForegroundPackageMonitor() {
	if r.monitor == nil {
		return
	}
	r.monitor.stop()
	r.monitor = nil
}

func (r *Runner) startHealthSignalMonitor() {
	if !r.cfg.StopOnCrash && !r.cfg.StopOnANR {
		return
	}
	targetPackage := strings.TrimSpace(r.cfg.PackageName)
	monitor := newHealthSignalMonitor(r.driver, targetPackage, r.cfg.HealthSignalMonitorInterval)
	monitor.start()
	r.healthMonitor = monitor
}

func (r *Runner) stopHealthSignalMonitor() {
	if r.healthMonitor == nil {
		return
	}
	r.healthMonitor.stop()
	r.healthMonitor = nil
}

func (r *Runner) snapshotHealthSignals() (crash bool, anr bool, ready bool) {
	if r.healthMonitor == nil {
		return false, false, false
	}
	return r.healthMonitor.snapshot()
}

func (r *Runner) resolvePageInfo(xml string, screenshot []byte) (string, string) {
	pageName := r.resolveBasePageName(xml)
	transformer, ok := r.decider.(PageInfoTransformer)
	if !ok || transformer == nil {
		return pageName, xml
	}
	pageInfo, err := transformer.TransformPageInfoWithInput(pageName, session.ActionInput{
		XMLDescOfGuiTree: xml,
		Screenshot:       screenshot,
	})
	if err != nil {
		logger.Warnf("transform page info failed, use fallback: %v", err)
		return pageName, xml
	}
	nextPageName := strings.TrimSpace(pageInfo.PageName)
	if nextPageName == "" {
		nextPageName = pageName
	}
	nextXML := strings.TrimSpace(pageInfo.XML)
	if nextXML == "" {
		nextXML = xml
	}
	return nextPageName, nextXML
}

func (r *Runner) resolveBasePageName(xml string) string {
	return resolveBasePageNameByStrategy(r, xml)
}

func (r *Runner) currentHealthSignals() (crash bool, anr bool) {
	cachedCrash, cachedANR, ready := r.snapshotHealthSignals()
	if ready {
		if r.cfg.StopOnCrash {
			crash = cachedCrash
		}
		if r.cfg.StopOnANR {
			anr = cachedANR
		}
	}
	if r.cfg.StopOnCrash && !crash {
		crash = r.detectCrashBySystem()
	}
	if r.cfg.StopOnANR && !anr {
		anr = r.detectANRBySystem()
	}
	return crash, anr
}
