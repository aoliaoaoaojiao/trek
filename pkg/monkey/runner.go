package monkey

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/recovery"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	"trek/logger"
	"trek/pkg/driver/common"
	"trek/pkg/session"

	"github.com/beevik/etree"
)

const (
	defaultMaxSteps                                = 300
	defaultMaxDuration                             = 10 * time.Minute
	defaultStepInterval                            = 300 * time.Millisecond
	defaultMaxConsecutiveFailures                  = 8
	defaultFailureRecoveryInterval                 = 3
	defaultLongClickDuration                       = 800 * time.Millisecond
	defaultScrollDuration                          = 350 * time.Millisecond
	defaultScrollSteps                       int64 = 20
	defaultScrollRepeat                            = 3
	defaultPageSourceType                          = "uia"
	defaultForegroundMonitorInterval               = 300 * time.Millisecond
	defaultHealthSignalMonitorInterval             = 500 * time.Millisecond
	defaultBlockNoChangeThreshold                  = 3
	defaultRecoveryCooldownSteps                   = 2
	defaultTwoStateLoopThreshold                   = 2
	defaultHighVisitThreshold                      = 8
	defaultLowRewardWindow                         = 6
	defaultCandidateEnhancementMinStepGap          = 12
	defaultCandidateAmbiguityTopGapThreshold       = 0.15
	defaultHighValuePageVisitLimit                 = 2
	defaultCandidateRiskDropThreshold              = 2.1
	defaultCandidateMinFusionScore                 = -0.3
	maxRecentTraceEntries                          = 8
)

const (
	blockReasonScrollNoChange   = "scroll_no_change"
	blockReasonSamePageNoChange = "same_page_no_change"
	blockReasonTwoStatePingPong = "two_state_ping_pong"
	blockReasonHighVisitLowGain = "high_visit_low_reward"
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
	StopANRDetectedLogcat      StopReason = "anr_signal"

	// 兼容常量：保留旧名称，指向更精确的新原因。
	StopCrashDetected StopReason = StopCrashDetectedLogcat
	StopANRDetected   StopReason = StopANRDetectedLogcat
)

// PageNameResolver 从 XML 中提取页面名。
type PageNameResolver func(xml string) string

// Config 是 Smart Monkey Runner 配置。
type Config struct {
	PackageName                       string
	DeviceSerial                      string
	AutoStartOnRun                    *bool
	ActionThrottleEnabled             *bool
	RandomizeThrottle                 bool
	EnableFailureRecovery             *bool
	FailureRecoveryInterval           int
	MaxSteps                          int
	MaxDuration                       time.Duration
	StepInterval                      time.Duration
	MaxConsecutiveFailures            int
	PageSourceType                    string
	CaptureScreenshot                 bool
	LongClickDuration                 time.Duration
	ScrollDuration                    time.Duration
	ScrollSteps                       int64
	ScrollRepeat                      int
	StopOnCrash                       bool
	StopOnANR                         bool
	KeepStepRecords                   bool
	PageNameResolver                  PageNameResolver
	PageNameStrategy                  string
	ForegroundMonitorInterval         time.Duration
	HealthSignalMonitorInterval       time.Duration
	EffectiveTouchArea                *EffectiveTouchArea
	EnableBlockRecovery               *bool
	BlockNoChangeThreshold            int
	RecoveryCooldownSteps             int
	RecoveryLLMBudgetMaxCalls         int
	RecoveryLLMBudgetWindowStep       int
	EnableExploreLLMEnhancement       *bool
	CandidateEnhancementMinStepGap    int
	CandidateAmbiguityTopGapThreshold float64
	HighValuePageVisitLimit           int
	TwoStateLoopThreshold             int
	HighVisitThreshold                int
	LowRewardWindow                   int
	CandidateRiskDropThreshold        float64
	CandidateMinFusionScore           float64
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
	StartedAt                   time.Time
	FinishedAt                  time.Time
	DurationMs                  int64
	StopReason                  StopReason
	Preflight                   *common.EnvironmentCheckResult
	PreflightError              string
	StepsPlanned                int
	StepsTotal                  int
	StepsSucceeded              int
	StepsFailed                 int
	ConsecutiveFailures         int
	ActionCount                 map[string]int
	PageVisitCount              map[string]int
	OutOfAppRecoveries          int
	RecoveryCooldownEnterCount  int
	RecoveryCooldownStepCount   int
	CandidateEnhancementCalls   int
	CandidateEnhancementSelects int
	RecoveryLLMCalls            int
	RecoveryLLMBudgetDenied     int
	EnhancementLLMBudgetDenied  int
	Records                     []StepRecord
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

// TraversalOutcomeObserver 是可选接口，用于接收统一动作结果并回写算法在线学习。
type TraversalOutcomeObserver interface {
	ObserveTraversalOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error
}

// BlockRecoveryDecider 是可选接口：当 Runner 识别到“滚动无进展阻塞”时，
// 由决策层提供恢复动作；未实现时 Runner 使用 BACK 兜底。
type BlockRecoveryDecider interface {
	NextBlockRecoveryAction(pageName string, input session.ActionInput) (*types.ActionCommand, error)
}

// ContextAwareBlockRecoveryDecider 是面向第一阶段统一上下文的可选恢复接口。
type ContextAwareBlockRecoveryDecider interface {
	NextBlockRecoveryActionWithContext(ctx enginestate.TraversalContext, input session.ActionInput) (*types.ActionCommand, error)
}

// RecoveryMemoryProvider 提供恢复阶段 memory 候选。
type RecoveryMemoryProvider interface {
	BuildMemoryRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// RecoveryHeuristicProvider 提供恢复阶段 heuristic 候选。
type RecoveryHeuristicProvider interface {
	BuildHeuristicRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// RecoveryLLMProvider 提供恢复阶段 llm 候选。
type RecoveryLLMProvider interface {
	BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// RecoveryCandidateSelector 在恢复阶段从融合候选集中选择最终动作。
type RecoveryCandidateSelector interface {
	SelectRecoveryAction(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types.ActionCommand, error)
}

// AlgorithmCandidateProvider 提供主探索阶段的算法候选。
type AlgorithmCandidateProvider interface {
	BuildAlgorithmCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)
}

// RecoveryMemoryWriter 在恢复动作执行后写回成功/失败经验。
type RecoveryMemoryWriter interface {
	RecordRecoveryMemoryOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, escaped bool) error
}

// CandidateEnhancementMemoryWriter 写回候选增强动作的正负样本。
type CandidateEnhancementMemoryWriter interface {
	RecordCandidateEnhancementOutcome(ctx enginestate.TraversalContext, item candidate.Candidate, improved bool) error
}

// RecoveryFailedActionProvider 提供可持久化的已知失败恢复动作集合。
type RecoveryFailedActionProvider interface {
	BuildKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error)
}

// RecoverySuccessfulActionProvider 提供可持久化的已知成功恢复动作集合。
type RecoverySuccessfulActionProvider interface {
	BuildKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error)
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
	decider                Decider
	driver                 common.IDriver
	cfg                    Config
	rng                    *rand.Rand
	monitor                *foregroundPackageMonitor
	healthMonitor          *healthSignalMonitor
	blockDetector          *blockDetector
	recoveryState          *recoveryStateMachine
	recoveryPlanner        recovery.RecoveryPlanner
	candidateEnhanceBudget recovery.LLMBudget
	lastEnhancementStep    int
	recentTrace            []enginestate.ActionTrace
	pageVisitCount         map[string]int
	actionCount            map[string]int
	recoveryFailedAction   map[string]bool
	recoverySuccessAction  map[string]bool
	pendingBlockRecovery   bool
	lastRecoveryAttempt    *recoveryAttempt
	lastEnhancementAttempt *enhancementAttempt
	cooldownEnterCount     int
	cooldownStepCount      int
	enhancementCallCount   int
	enhancementHitCount    int
	recoveryLLMCallCount   int
	recoveryLLMDeniedCount int
	enhanceLLMDeniedCount  int
}

type recoveryAttempt struct {
	ctx       enginestate.TraversalContext
	candidate candidate.Candidate
}

type enhancementAttempt struct {
	ctx       enginestate.TraversalContext
	candidate candidate.Candidate
	step      int
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
	var enhanceBudget recovery.LLMBudget
	if cfg.RecoveryLLMBudgetMaxCalls > 0 {
		enhanceBudget = recovery.NewSlidingWindowLLMBudget(
			cfg.RecoveryLLMBudgetMaxCalls,
			cfg.RecoveryLLMBudgetWindowStep,
		)
	}
	return &Runner{
		decider:                decider,
		driver:                 driver,
		cfg:                    cfg,
		rng:                    rand.New(rand.NewSource(time.Now().UnixNano())),
		blockDetector:          newBlockDetector(cfg.BlockNoChangeThreshold, cfg.TwoStateLoopThreshold, cfg.HighVisitThreshold, cfg.LowRewardWindow),
		recoveryState:          newRecoveryStateMachineWithCooldown(cfg.RecoveryCooldownSteps),
		candidateEnhanceBudget: enhanceBudget,
		lastEnhancementStep:    -1,
		recentTrace:            make([]enginestate.ActionTrace, 0, maxRecentTraceEntries),
		pageVisitCount:         make(map[string]int),
		actionCount:            make(map[string]int),
		recoveryFailedAction:   make(map[string]bool),
		recoverySuccessAction:  make(map[string]bool),
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
	r.pageVisitCount = report.PageVisitCount
	r.actionCount = report.ActionCount
	r.cooldownEnterCount = 0
	r.cooldownStepCount = 0
	r.enhancementCallCount = 0
	r.enhancementHitCount = 0
	r.recoveryLLMCallCount = 0
	r.recoveryLLMDeniedCount = 0
	r.enhanceLLMDeniedCount = 0
	defer func() {
		report.RecoveryCooldownEnterCount = r.cooldownEnterCount
		report.RecoveryCooldownStepCount = r.cooldownStepCount
		report.CandidateEnhancementCalls = r.enhancementCallCount
		report.CandidateEnhancementSelects = r.enhancementHitCount
		report.RecoveryLLMCalls = r.recoveryLLMCallCount
		report.RecoveryLLMBudgetDenied = r.recoveryLLMDeniedCount
		report.EnhancementLLMBudgetDenied = r.enhanceLLMDeniedCount
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
		r.advanceRecoveryStateOnStep()
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

		input := session.ActionInput{
			XMLDescOfGuiTree: xml,
			Screenshot:       screenshot,
		}

		cmd, err := r.nextCommandWithRecovery(step, beforePage, pageName, input)
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
		r.recordActionTrace(beforePage, cmd)

		logger.Infof("monkey step=%d execute cmd={%s}%s%s", step, cmd.DetailLogString(), formatTapPointLog(cmd), formatSwipePointLog(cmd))

		if err = r.execute(cmd); err != nil {
			record.Err = err.Error()
			afterPage := r.capturePageSnapshot(pageSource, pageName)
			r.notifyTraversalOutcome(step, beforePage, afterPage, cmd, false)
			r.recordRecoveryOutcome(false)
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

		// RESTART/START 类动作执行成功后，重置连续失败计数并等待应用恢复。
		// 这些动作会重启应用，Poco 等页面源服务需要时间重新连接，
		// 连续失败计数不应将重启后的短暂连接中断计为失败。
		if cmd.Act == types.RESTART || cmd.Act == types.CLEAN_RESTART || cmd.Act == types.START {
			report.ConsecutiveFailures = 0
			logger.Infof("monkey step=%d executed app restart action=%s, resetting consecutive failures and waiting for recovery", step, cmd.Act.String())
			r.sleepStep(ctx, 5*time.Second)
		} else {
			report.ConsecutiveFailures = 0
		}

		afterPage := r.capturePageSnapshot(pageSource, pageName)
		escaped := afterPage != nil &&
			pageSignature(beforePage.PageName, beforePage.XML) != pageSignature(afterPage.PageName, afterPage.XML)
		r.notifyTraversalOutcome(step, beforePage, afterPage, cmd, true)
		if r.shouldEnableBlockRecovery() && r.blockDetector.Observe(cmd, beforePage, afterPage) {
			r.handleBlockDetected(r.blockDetector.LastReason())
			logger.Warnf("monkey step=%d detected block loop reason=%s mode=%s pending=%t",
				step, r.blockDetector.LastReason(), r.recoveryState.Mode(), r.pendingBlockRecovery)
			if r.pendingBlockRecovery {
				r.blockDetector.Reset()
			}
		} else if afterPage != nil && r.recoveryState != nil && r.recoveryState.Mode() == TraversalModeRecover {
			r.handleProgress(escaped)
		}
		r.recordRecoveryOutcome(escaped)
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

func (r *Runner) notifyTraversalOutcome(step int, before session.PageSnapshot, after *session.PageSnapshot, cmd *types.ActionCommand, success bool) {
	ctx := r.buildTraversalContext(step, before, nil, nil)
	outcome := r.deriveTraversalOutcome(ctx, cmd, before, after, success)
	if observer, ok := r.decider.(TraversalOutcomeObserver); ok && observer != nil {
		if err := observer.ObserveTraversalOutcome(ctx, cmd, outcome); err != nil {
			logger.Warnf("observe traversal outcome failed: step=%d outcome=%s err=%v", step, outcome, err)
		}
	}
	r.recordCandidateEnhancementOutcome(step, cmd, outcome)
}

func (r *Runner) deriveTraversalOutcome(
	ctx enginestate.TraversalContext,
	cmd *types.ActionCommand,
	before session.PageSnapshot,
	after *session.PageSnapshot,
	success bool,
) traversal.ActionOutcome {
	if cmd == nil || !success || after == nil {
		return traversal.OutcomeNoOp
	}
	if cmd.Act == types.NOP {
		return traversal.OutcomeNoOp
	}
	beforeSig := pageSignature(before.PageName, before.XML)
	afterSig := pageSignature(after.PageName, after.XML)
	if beforeSig == "" || afterSig == "" {
		return traversal.OutcomeNoOp
	}
	if beforeSig == afterSig {
		return traversal.OutcomeSameState
	}
	if ctx.Mode == TraversalModeRecover || ctx.Mode == TraversalModeSuspectBlocked {
		return traversal.OutcomeEscapeBlock
	}
	return traversal.OutcomeNewState
}

func (r *Runner) recordCandidateEnhancementOutcome(step int, cmd *types.ActionCommand, outcome traversal.ActionOutcome) {
	if r == nil || cmd == nil || r.lastEnhancementAttempt == nil {
		return
	}
	attempt := r.lastEnhancementAttempt
	if attempt.step != step {
		return
	}
	defer func() {
		r.lastEnhancementAttempt = nil
	}()
	if attempt.candidate.Command == nil || attempt.candidate.Command.ToJSON() != cmd.ToJSON() {
		return
	}
	writer, ok := r.decider.(CandidateEnhancementMemoryWriter)
	if !ok || writer == nil {
		return
	}
	improved := outcome == traversal.OutcomeNewState || outcome == traversal.OutcomeEscapeBlock
	if err := writer.RecordCandidateEnhancementOutcome(attempt.ctx, attempt.candidate, improved); err != nil {
		logger.Warnf("record candidate enhancement outcome failed: improved=%t err=%v", improved, err)
	}
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

var widgetXPathRegex = regexp.MustCompile(`(?:^|[,{ ])xpath:([^,}]+)`)
var widgetPathRegex = regexp.MustCompile(`(?:^|[,{ ])path:([^,}]+)`)

func resolvePocoScrollRectFromWidgetPath(widgetInfo string, xml string) (*types.Rect, bool) {
	targetPath, ok := extractWidgetLocatorPath(widgetInfo)
	if !ok {
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

	current := findElementByCompatiblePath(doc, root, targetPath)
	for current != nil {
		if rect, ok := parseRectFromBoundsValue(current.SelectAttrValue("bounds", "")); ok && !rect.IsEmpty() {
			return rect, true
		}
		current = current.Parent()
	}
	return nil, false
}

func extractWidgetLocatorPath(widgetInfo string) (string, bool) {
	if match := widgetXPathRegex.FindStringSubmatch(widgetInfo); len(match) >= 2 {
		xpath := strings.TrimSpace(match[1])
		if xpath != "" {
			return xpath, true
		}
	}
	if match := widgetPathRegex.FindStringSubmatch(widgetInfo); len(match) >= 2 {
		path := strings.TrimSpace(match[1])
		if path != "" {
			return path, true
		}
	}
	return "", false
}

func findElementByCompatiblePath(doc *etree.Document, root *etree.Element, targetPath string) *etree.Element {
	if doc == nil || root == nil {
		return nil
	}
	path := strings.TrimSpace(targetPath)
	if path == "" {
		return nil
	}
	if matched := doc.FindElement(path); matched != nil {
		return matched
	}
	if matched := root.FindElement(path); matched != nil {
		return matched
	}

	normalizedPath := strings.TrimPrefix(path, "/")
	if matched := root.FindElement(normalizedPath); matched != nil {
		return matched
	}

	rootPrefix := "/" + root.Tag
	if strings.HasPrefix(path, rootPrefix) {
		trimmed := strings.TrimPrefix(path, rootPrefix)
		if matched := root.FindElement(trimmed); matched != nil {
			return matched
		}
		trimmed = strings.TrimPrefix(trimmed, "[1]")
		if matched := root.FindElement(trimmed); matched != nil {
			return matched
		}
	}
	return nil
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
	cmd, _, err := r.nextCommandWithCandidates(pageName, input)
	return cmd, err
}

func (r *Runner) nextCommandWithCandidates(pageName string, input session.ActionInput) (*types.ActionCommand, []WeightedCandidate, error) {
	if wd, ok := r.decider.(WeightedDecider); ok {
		candidates, err := wd.NextWeightedActionsWithInput(pageName, input)
		if err != nil {
			return nil, nil, err
		}
		return r.pickWeightedCandidate(candidates), candidates, nil
	}
	cmd, err := r.decider.NextActionWithInput(pageName, input)
	return cmd, nil, err
}

func (r *Runner) nextCommandWithRecovery(step int, beforePage session.PageSnapshot, pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	r.lastEnhancementAttempt = nil
	if !r.pendingBlockRecovery {
		cmd, weighted, err := r.nextCommandWithCandidates(pageName, input)
		if err != nil || cmd == nil {
			return cmd, err
		}
		ctx := r.buildTraversalContext(step, beforePage, nil, nil)
		cmd, err = r.trySelectFromTraversalCandidates(ctx, cmd, weighted)
		if err != nil {
			logger.Warnf("select traversal candidate failed, fallback to base action: %v", err)
		}
		enhanced, enhanceErr := r.tryEnhanceCandidates(step, beforePage, cmd, weighted)
		if enhanceErr != nil {
			logger.Warnf("enhance candidates failed, fallback to base action: %v", enhanceErr)
			return cmd, nil
		}
		if enhanced != nil {
			return enhanced, nil
		}
		return cmd, nil
	}
	r.pendingBlockRecovery = false
	r.lastRecoveryAttempt = nil
	cmd, err := r.nextBlockRecoveryCommand(pageName, input)
	if err != nil {
		logger.Warnf("build block recovery command failed, fallback to normal command: %v", err)
		return r.nextCommand(pageName, input)
	}
	if cmd == nil {
		logger.Warnf("block recovery command is nil, fallback to normal command")
		return r.nextCommand(pageName, input)
	}
	logger.Infof("block recovery command selected: %s", cmd.DetailLogString())
	return cmd, nil
}

func (r *Runner) trySelectFromTraversalCandidates(
	ctx enginestate.TraversalContext,
	baseCmd *types.ActionCommand,
	weighted []WeightedCandidate,
) (*types.ActionCommand, error) {
	if r == nil || baseCmd == nil {
		return baseCmd, nil
	}
	provider, ok := r.decider.(AlgorithmCandidateProvider)
	if !ok || provider == nil {
		return baseCmd, nil
	}
	selector, ok := r.decider.(RecoveryCandidateSelector)
	if !ok || selector == nil {
		return baseCmd, nil
	}

	items, err := provider.BuildAlgorithmCandidates(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return baseCmd, nil
	}
	baseItems := weightedCandidatesToAlgorithmCandidates(weighted)
	if len(baseItems) == 0 {
		baseItems = []candidate.Candidate{candidateFromCommand(baseCmd, candidate.SourceAlgorithm)}
	}
	items = append(items, baseItems...)

	knownFailed, err := r.collectKnownFailedRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	fused := candidate.FuseCandidates(items, candidate.FusionOptions{
		KnownFailedActions:   knownFailed,
		RiskDropThreshold:    r.cfg.CandidateRiskDropThreshold,
		EnableMinScoreFilter: true,
		MinScoreThreshold:    r.cfg.CandidateMinFusionScore,
		KeepTopOnFiltered:    true,
	})
	selected, err := selector.SelectRecoveryAction(ctx, fused)
	if err != nil {
		return nil, err
	}
	if selected == nil || !selected.IsValid() {
		return baseCmd, nil
	}
	return selected, nil
}

func (r *Runner) tryEnhanceCandidates(step int, beforePage session.PageSnapshot, baseCmd *types.ActionCommand, weighted []WeightedCandidate) (*types.ActionCommand, error) {
	if r == nil || baseCmd == nil {
		return nil, nil
	}
	if !r.shouldEnableExploreLLMEnhancement() {
		return nil, nil
	}
	ctx := r.buildTraversalContext(step, beforePage, nil, nil)
	ctx.LocalCandidates = summarizeWeightedCandidates(weighted, baseCmd)
	knownFailed, err := r.collectKnownFailedRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	knownSuccess, err := r.collectKnownSuccessfulRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	ctx.KnownFailedActions = actionKeyList(knownFailed)
	ctx.KnownSuccessActions = actionKeyList(knownSuccess)
	if !r.shouldTriggerCandidateEnhancement(ctx, step, baseCmd, weighted) {
		return nil, nil
	}

	llmProvider, ok := r.decider.(RecoveryLLMProvider)
	if !ok || llmProvider == nil {
		return nil, nil
	}
	selector, ok := r.decider.(RecoveryCandidateSelector)
	if !ok || selector == nil {
		return nil, nil
	}
	if !r.allowCandidateEnhancementLLM(ctx) {
		r.enhanceLLMDeniedCount++
		return nil, nil
	}

	llmItems, err := llmProvider.BuildLLMRecoveryCandidates(ctx)
	if err != nil {
		return nil, err
	}
	r.enhancementCallCount++
	r.recordCandidateEnhancementLLMCall(ctx, step)
	if len(llmItems) == 0 {
		return nil, nil
	}

	items := make([]candidate.Candidate, 0, len(llmItems)+1)
	items = append(items, candidateFromCommand(baseCmd, candidate.SourceAlgorithm))
	items = append(items, llmItems...)
	fused := candidate.FuseCandidates(items, candidate.FusionOptions{
		KnownFailedActions:   knownFailed,
		RiskDropThreshold:    r.cfg.CandidateRiskDropThreshold,
		EnableMinScoreFilter: true,
		MinScoreThreshold:    r.cfg.CandidateMinFusionScore,
		KeepTopOnFiltered:    true,
	})
	selected, err := selector.SelectRecoveryAction(ctx, fused)
	if err != nil {
		return nil, err
	}
	if selected == nil {
		return nil, nil
	}
	if selected.ToJSON() == baseCmd.ToJSON() {
		return nil, nil
	}
	if chosen := findCandidateByCommand(fused, selected); chosen != nil {
		r.lastEnhancementAttempt = &enhancementAttempt{ctx: ctx, candidate: *chosen, step: step}
	} else {
		r.lastEnhancementAttempt = &enhancementAttempt{
			ctx:       ctx,
			candidate: candidateFromCommand(selected, candidate.SourceLLM),
			step:      step,
		}
	}
	r.enhancementHitCount++
	logger.Infof("candidate enhancement selected action: base=%s enhanced=%s", baseCmd.DetailLogString(), selected.DetailLogString())
	return selected, nil
}

func findCandidateByCommand(items []candidate.Candidate, cmd *types.ActionCommand) *candidate.Candidate {
	if cmd == nil {
		return nil
	}
	key := cmd.ToJSON()
	for _, item := range items {
		if item.Command == nil {
			continue
		}
		if item.Command.ToJSON() == key {
			copyItem := item
			return &copyItem
		}
	}
	return nil
}

func (r *Runner) handleBlockDetected(reason string) {
	if r == nil {
		return
	}
	if r.recoveryState == nil {
		r.recoveryState = newRecoveryStateMachineWithCooldown(r.cfg.RecoveryCooldownSteps)
	}
	beforeMode := r.recoveryState.Mode()
	r.recoveryState.OnBlockDetected(reason)
	r.pendingBlockRecovery = r.recoveryState.Mode() == TraversalModeRecover
	logger.Infof("recovery state transition on block: from=%s to=%s reason=%s",
		beforeMode, r.recoveryState.Mode(), r.recoveryState.BlockReason())
}

func (r *Runner) handleProgress(escaped bool) {
	if r == nil || r.recoveryState == nil {
		return
	}
	beforeMode := r.recoveryState.Mode()
	r.recoveryState.OnProgress(escaped)
	if beforeMode != TraversalModeCooldown && r.recoveryState.Mode() == TraversalModeCooldown {
		r.cooldownEnterCount++
	}
	if beforeMode != r.recoveryState.Mode() {
		logger.Infof("recovery state transition on progress: from=%s to=%s",
			beforeMode, r.recoveryState.Mode())
	}
}

func (r *Runner) advanceRecoveryStateOnStep() {
	if r == nil || r.recoveryState == nil {
		return
	}
	beforeMode := r.recoveryState.Mode()
	if beforeMode == TraversalModeCooldown {
		r.cooldownStepCount++
	}
	r.recoveryState.OnStepAdvance()
	if beforeMode != r.recoveryState.Mode() {
		logger.Infof("recovery state transition on step: from=%s to=%s",
			beforeMode, r.recoveryState.Mode())
	}
}

func (r *Runner) buildTraversalContext(step int, page session.PageSnapshot, pageVisitCount map[string]int, actionCount map[string]int) enginestate.TraversalContext {
	mode := TraversalModeExplore
	blockReason := ""
	if r != nil && r.recoveryState != nil {
		mode = r.recoveryState.Mode()
		blockReason = r.recoveryState.BlockReason()
	}
	if pageVisitCount == nil && r != nil {
		pageVisitCount = r.pageVisitCount
	}
	if actionCount == nil && r != nil {
		actionCount = r.actionCount
	}
	signature := pageSignature(page.PageName, page.XML)
	return enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:             step,
		Mode:             mode,
		PageName:         page.PageName,
		PageSignature:    signature,
		ClusterSignature: signature,
		XML:              page.XML,
		Screenshot:       page.Screenshot,
		BlockReason:      blockReason,
		RecentTrace:      r.cloneRecentTrace(),
		PageVisitCount:   pageVisitCount,
		ActionCount:      actionCount,
	})
}

func (r *Runner) recordActionTrace(page session.PageSnapshot, cmd *types.ActionCommand) {
	if r == nil || cmd == nil {
		return
	}
	trace := enginestate.ActionTrace{
		PageSignature: pageSignature(page.PageName, page.XML),
		ActionKey:     commandTraceKey(cmd),
	}
	if strings.TrimSpace(trace.PageSignature) == "" || strings.TrimSpace(trace.ActionKey) == "" {
		return
	}
	r.recentTrace = append(r.recentTrace, trace)
	if len(r.recentTrace) > maxRecentTraceEntries {
		r.recentTrace = r.recentTrace[len(r.recentTrace)-maxRecentTraceEntries:]
	}
}

func (r *Runner) cloneRecentTrace() []enginestate.ActionTrace {
	if r == nil || len(r.recentTrace) == 0 {
		return nil
	}
	result := make([]enginestate.ActionTrace, len(r.recentTrace))
	copy(result, r.recentTrace)
	return result
}

func commandTraceKey(cmd *types.ActionCommand) string {
	if cmd == nil {
		return ""
	}
	return cmd.Act.String()
}

func (r *Runner) nextBlockRecoveryCommand(pageName string, input session.ActionInput) (*types.ActionCommand, error) {
	ctx := r.buildTraversalContext(0, session.PageSnapshot{
		PageName:   pageName,
		XML:        input.XMLDescOfGuiTree,
		Screenshot: input.Screenshot,
	}, nil, nil)
	knownFailed, knownErr := r.collectKnownFailedRecoveryActions(ctx)
	if knownErr != nil {
		return nil, knownErr
	}
	knownSuccess, knownSuccessErr := r.collectKnownSuccessfulRecoveryActions(ctx)
	if knownSuccessErr != nil {
		return nil, knownSuccessErr
	}
	ctx.KnownFailedActions = actionKeyList(knownFailed)
	ctx.KnownSuccessActions = actionKeyList(knownSuccess)

	if planner := r.getRecoveryPlanner(); planner != nil {
		items, err := planner.BuildRecoveryCandidates(ctx)
		if err != nil {
			return nil, err
		}
		fused := candidate.FuseCandidates(items, candidate.FusionOptions{
			KnownFailedActions:   knownFailed,
			RiskDropThreshold:    r.cfg.CandidateRiskDropThreshold,
			EnableMinScoreFilter: true,
			MinScoreThreshold:    r.cfg.CandidateMinFusionScore,
			KeepTopOnFiltered:    true,
		})
		if selector, ok := r.decider.(RecoveryCandidateSelector); ok && selector != nil {
			selected, selectErr := selector.SelectRecoveryAction(ctx, fused)
			if selectErr != nil {
				return nil, selectErr
			}
			if selected != nil {
				r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, candidate: candidateFromCommand(selected, candidate.SourceAlgorithm)}
				return selected, nil
			}
		}
		if item := firstCandidateWithCommand(fused); item != nil {
			r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, candidate: *item}
			return item.Command, nil
		}
	}

	if provider, ok := r.decider.(ContextAwareBlockRecoveryDecider); ok && provider != nil {
		cmd, err := provider.NextBlockRecoveryActionWithContext(ctx, input)
		if err != nil {
			return nil, err
		}
		if cmd != nil {
			r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, candidate: candidateFromCommand(cmd, candidate.SourceHeuristic)}
			return cmd, nil
		}
	}
	if provider, ok := r.decider.(BlockRecoveryDecider); ok && provider != nil {
		cmd, err := provider.NextBlockRecoveryAction(pageName, input)
		if err != nil {
			return nil, err
		}
		if cmd != nil {
			r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, candidate: candidateFromCommand(cmd, candidate.SourceHeuristic)}
			return cmd, nil
		}
	}
	// 默认恢复动作：返回 BACK，避免直接应用级 RESTART。
	fallback := &types.ActionCommand{Act: types.BACK}
	r.lastRecoveryAttempt = &recoveryAttempt{ctx: ctx, candidate: candidateFromCommand(fallback, candidate.SourceHeuristic)}
	return fallback, nil
}

func (r *Runner) getRecoveryPlanner() recovery.RecoveryPlanner {
	if r == nil {
		return nil
	}
	if r.recoveryPlanner != nil {
		return r.recoveryPlanner
	}

	config := recovery.PlannerConfig{}
	if provider, ok := r.decider.(RecoveryMemoryProvider); ok && provider != nil {
		config.Memory = recoveryProviderFunc(provider.BuildMemoryRecoveryCandidates)
	}
	if provider, ok := r.decider.(RecoveryHeuristicProvider); ok && provider != nil {
		config.Heuristic = recoveryProviderFunc(provider.BuildHeuristicRecoveryCandidates)
	}
	if provider, ok := r.decider.(RecoveryLLMProvider); ok && provider != nil {
		config.LLM = recoveryProviderFunc(provider.BuildLLMRecoveryCandidates)
	}
	if r.cfg.RecoveryLLMBudgetMaxCalls > 0 {
		config.LLMBudget = recovery.NewSlidingWindowLLMBudget(
			r.cfg.RecoveryLLMBudgetMaxCalls,
			r.cfg.RecoveryLLMBudgetWindowStep,
		)
	}
	config.OnLLMCall = func(ctx enginestate.TraversalContext) {
		r.recoveryLLMCallCount++
	}
	config.OnLLMBudgetDenied = func(ctx enginestate.TraversalContext) {
		r.recoveryLLMDeniedCount++
	}

	if config.Memory == nil && config.Heuristic == nil && config.LLM == nil {
		return nil
	}

	r.recoveryPlanner = recovery.NewPlanner(config)
	return r.recoveryPlanner
}

func firstCandidateWithCommand(items []candidate.Candidate) *candidate.Candidate {
	for _, item := range items {
		if item.Command != nil {
			copyItem := item
			return &copyItem
		}
	}
	return nil
}

func candidateFromCommand(cmd *types.ActionCommand, source string) candidate.Candidate {
	if cmd == nil {
		return candidate.Candidate{Source: source}
	}
	cmdCopy := *cmd
	return candidate.Candidate{
		Command: &cmdCopy,
		Source:  source,
	}
}

func weightedCandidatesToAlgorithmCandidates(weighted []WeightedCandidate) []candidate.Candidate {
	if len(weighted) == 0 {
		return nil
	}
	total := 0.0
	for _, item := range weighted {
		if item.Command == nil || item.Weight <= 0 {
			continue
		}
		total += item.Weight
	}
	result := make([]candidate.Candidate, 0, len(weighted))
	for _, item := range weighted {
		if item.Command == nil {
			continue
		}
		c := candidateFromCommand(item.Command, candidate.SourceAlgorithm)
		if total > 0 && item.Weight > 0 {
			c.Confidence = item.Weight / total
		}
		result = append(result, c)
	}
	return result
}

func summarizeWeightedCandidates(weighted []WeightedCandidate, baseCmd *types.ActionCommand) []enginestate.CandidateSummary {
	result := make([]enginestate.CandidateSummary, 0, len(weighted)+1)
	total := 0.0
	for _, item := range weighted {
		if item.Command == nil || item.Weight <= 0 {
			continue
		}
		total += item.Weight
	}
	for _, item := range weighted {
		if item.Command == nil {
			continue
		}
		confidence := 0.0
		if total > 0 && item.Weight > 0 {
			confidence = item.Weight / total
		}
		result = append(result, enginestate.CandidateSummary{
			ActionKey:  item.Command.ToJSON(),
			ActionType: item.Command.Act.String(),
			Source:     candidate.SourceAlgorithm,
			Confidence: confidence,
		})
	}
	if len(result) == 0 && baseCmd != nil {
		result = append(result, enginestate.CandidateSummary{
			ActionKey:  baseCmd.ToJSON(),
			ActionType: baseCmd.Act.String(),
			Source:     candidate.SourceAlgorithm,
			Confidence: 1,
		})
	}
	return result
}

func actionKeyList(actions map[string]bool) []string {
	if len(actions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(actions))
	for key, value := range actions {
		if value && strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (r *Runner) recordRecoveryOutcome(escaped bool) {
	if r == nil || r.lastRecoveryAttempt == nil {
		return
	}
	attempt := *r.lastRecoveryAttempt
	defer func() {
		r.lastRecoveryAttempt = nil
	}()
	r.markRecoveryActionOutcome(attempt.candidate, escaped)

	writer, ok := r.decider.(RecoveryMemoryWriter)
	if !ok || writer == nil {
		return
	}
	if err := writer.RecordRecoveryMemoryOutcome(attempt.ctx, attempt.candidate, escaped); err != nil {
		logger.Warnf("record recovery memory outcome failed: escaped=%t err=%v", escaped, err)
	}
}

func (r *Runner) markRecoveryActionOutcome(item candidate.Candidate, escaped bool) {
	if r == nil || item.Command == nil {
		return
	}
	key := item.Command.ToJSON()
	if strings.TrimSpace(key) == "" {
		return
	}
	if r.recoveryFailedAction == nil {
		r.recoveryFailedAction = make(map[string]bool)
	}
	if r.recoverySuccessAction == nil {
		r.recoverySuccessAction = make(map[string]bool)
	}
	if escaped {
		delete(r.recoveryFailedAction, key)
		r.recoverySuccessAction[key] = true
		return
	}
	r.recoveryFailedAction[key] = true
	delete(r.recoverySuccessAction, key)
}

func (r *Runner) collectKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	known := make(map[string]bool, len(r.recoveryFailedAction))
	for key, value := range r.recoveryFailedAction {
		if value {
			known[key] = true
		}
	}

	provider, ok := r.decider.(RecoveryFailedActionProvider)
	if !ok || provider == nil {
		return known, nil
	}
	persisted, err := provider.BuildKnownFailedRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range persisted {
		if value {
			known[key] = true
		}
	}
	return known, nil
}

func (r *Runner) collectKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error) {
	known := make(map[string]bool, len(r.recoverySuccessAction))
	for key, value := range r.recoverySuccessAction {
		if value {
			known[key] = true
		}
	}
	provider, ok := r.decider.(RecoverySuccessfulActionProvider)
	if !ok || provider == nil {
		return known, nil
	}
	persisted, err := provider.BuildKnownSuccessfulRecoveryActions(ctx)
	if err != nil {
		return nil, err
	}
	for key, value := range persisted {
		if value {
			known[key] = true
		}
	}
	return known, nil
}

type recoveryProviderFunc func(ctx enginestate.TraversalContext) ([]candidate.Candidate, error)

func (f recoveryProviderFunc) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	return f(ctx)
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
	if cfg.BlockNoChangeThreshold <= 0 {
		cfg.BlockNoChangeThreshold = defaultBlockNoChangeThreshold
	}
	if cfg.RecoveryCooldownSteps <= 0 {
		cfg.RecoveryCooldownSteps = defaultRecoveryCooldownSteps
	}
	if cfg.RecoveryLLMBudgetMaxCalls < 0 {
		cfg.RecoveryLLMBudgetMaxCalls = 0
	}
	if cfg.RecoveryLLMBudgetWindowStep < 0 {
		cfg.RecoveryLLMBudgetWindowStep = 0
	}
	if cfg.CandidateEnhancementMinStepGap <= 0 {
		cfg.CandidateEnhancementMinStepGap = defaultCandidateEnhancementMinStepGap
	}
	if cfg.CandidateAmbiguityTopGapThreshold <= 0 {
		cfg.CandidateAmbiguityTopGapThreshold = defaultCandidateAmbiguityTopGapThreshold
	}
	if cfg.HighValuePageVisitLimit <= 0 {
		cfg.HighValuePageVisitLimit = defaultHighValuePageVisitLimit
	}
	if cfg.TwoStateLoopThreshold <= 0 {
		cfg.TwoStateLoopThreshold = defaultTwoStateLoopThreshold
	}
	if cfg.HighVisitThreshold <= 0 {
		cfg.HighVisitThreshold = defaultHighVisitThreshold
	}
	if cfg.LowRewardWindow <= 0 {
		cfg.LowRewardWindow = defaultLowRewardWindow
	}
	if cfg.CandidateRiskDropThreshold <= 0 {
		cfg.CandidateRiskDropThreshold = defaultCandidateRiskDropThreshold
	}
	if cfg.CandidateMinFusionScore == 0 {
		cfg.CandidateMinFusionScore = defaultCandidateMinFusionScore
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

func (r *Runner) shouldEnableBlockRecovery() bool {
	if r == nil {
		return false
	}
	if r.cfg.EnableBlockRecovery == nil {
		return true
	}
	return *r.cfg.EnableBlockRecovery
}

func (r *Runner) shouldEnableExploreLLMEnhancement() bool {
	if r == nil || r.cfg.EnableExploreLLMEnhancement == nil {
		return false
	}
	return *r.cfg.EnableExploreLLMEnhancement
}

func (r *Runner) shouldTriggerCandidateEnhancement(ctx enginestate.TraversalContext, step int, baseCmd *types.ActionCommand, weighted []WeightedCandidate) bool {
	if r == nil || baseCmd == nil {
		return false
	}
	if ctx.Mode == TraversalModeRecover || ctx.Mode == TraversalModeCooldown {
		return false
	}
	if isAppRestartAction(baseCmd.Act) || baseCmd.Act == types.NOP {
		return false
	}
	if step-r.lastEnhancementStep < r.cfg.CandidateEnhancementMinStepGap {
		return false
	}
	if !r.isHighValuePage(ctx) {
		return false
	}
	return r.hasLowCandidateDistinction(ctx, weighted)
}

func (r *Runner) isHighValuePage(ctx enginestate.TraversalContext) bool {
	if ctx.Mode == TraversalModeSuspectBlocked {
		return true
	}
	visitCount := 0
	if r != nil && r.pageVisitCount != nil {
		visitCount = r.pageVisitCount[ctx.PageName]
	}
	// Explore 下仅在页面早期访问阶段触发，保持保守。
	return visitCount > 0 && visitCount <= r.cfg.HighValuePageVisitLimit
}

func (r *Runner) hasLowCandidateDistinction(ctx enginestate.TraversalContext, weighted []WeightedCandidate) bool {
	if len(weighted) == 0 {
		// 未提供候选权重时，仅在 SuspectBlocked 允许触发。
		return ctx.Mode == TraversalModeSuspectBlocked
	}
	positive := make([]float64, 0, len(weighted))
	for _, item := range weighted {
		if item.Command == nil || item.Weight <= 0 {
			continue
		}
		positive = append(positive, item.Weight)
	}
	if len(positive) < 2 {
		return false
	}
	sort.SliceStable(positive, func(i, j int) bool { return positive[i] > positive[j] })
	total := 0.0
	for _, w := range positive {
		total += w
	}
	if total <= 0 {
		return false
	}
	top1 := positive[0] / total
	top2 := positive[1] / total
	// 规则 1：前三个候选占比接近且头部差距小，视为区分度低。
	if len(positive) >= 3 && top1 <= 0.5 {
		return true
	}
	// 规则 2：前两名差距过小，视为难以区分。
	return (top1 - top2) <= r.cfg.CandidateAmbiguityTopGapThreshold
}

func (r *Runner) allowCandidateEnhancementLLM(ctx enginestate.TraversalContext) bool {
	if r == nil {
		return false
	}
	if r.candidateEnhanceBudget == nil {
		return true
	}
	return r.candidateEnhanceBudget.Allow(ctx)
}

func (r *Runner) recordCandidateEnhancementLLMCall(ctx enginestate.TraversalContext, step int) {
	if r == nil {
		return
	}
	r.lastEnhancementStep = step
	if r.candidateEnhanceBudget != nil {
		r.candidateEnhanceBudget.Record(ctx)
	}
}

type blockDetector struct {
	noChangeThreshold         int
	twoStateLoopThreshold     int
	highVisitThreshold        int
	lowRewardWindow           int
	consecutiveNoChangeScroll int
	consecutiveSamePageNoMove int
	consecutiveTwoStateLoops  int
	recentAfterSignatures     []string
	recentObservedSignatures  []string
	pageVisitCount            map[string]int
	lastReason                string
}

func newBlockDetector(noChangeThreshold int, twoStateLoopThreshold int, highVisitThreshold int, lowRewardWindow int) *blockDetector {
	if noChangeThreshold <= 0 {
		noChangeThreshold = defaultBlockNoChangeThreshold
	}
	if twoStateLoopThreshold <= 0 {
		twoStateLoopThreshold = defaultTwoStateLoopThreshold
	}
	if highVisitThreshold <= 0 {
		highVisitThreshold = defaultHighVisitThreshold
	}
	if lowRewardWindow <= 0 {
		lowRewardWindow = defaultLowRewardWindow
	}
	return &blockDetector{
		noChangeThreshold:        noChangeThreshold,
		twoStateLoopThreshold:    twoStateLoopThreshold,
		highVisitThreshold:       highVisitThreshold,
		lowRewardWindow:          lowRewardWindow,
		recentAfterSignatures:    make([]string, 0, 8),
		recentObservedSignatures: make([]string, 0, 16),
		pageVisitCount:           make(map[string]int),
	}
}

func (d *blockDetector) Observe(cmd *types.ActionCommand, before session.PageSnapshot, after *session.PageSnapshot) bool {
	if d == nil || cmd == nil || after == nil {
		d.Reset()
		return false
	}

	triggerNoChangeScroll := false
	triggerSamePageNoMove := false
	triggerTwoStateLoop := false
	triggerHighVisitLowGain := false

	beforeSig := pageSignature(before.PageName, before.XML)
	afterSig := pageSignature(after.PageName, after.XML)
	if !isBlockDetectorIgnoredAction(cmd.Act) && afterSig != "" {
		d.pageVisitCount[afterSig]++
		d.pushObservedSignature(afterSig)
	}

	if cmd.IsScrollAction() && beforeSig != "" && beforeSig == afterSig {
		d.consecutiveNoChangeScroll++
	} else {
		d.consecutiveNoChangeScroll = 0
	}
	if d.consecutiveNoChangeScroll >= d.noChangeThreshold {
		triggerNoChangeScroll = true
		d.lastReason = blockReasonScrollNoChange
	}

	if !isBlockDetectorIgnoredAction(cmd.Act) && !cmd.IsScrollAction() && beforeSig != "" && beforeSig == afterSig {
		d.consecutiveSamePageNoMove++
	} else {
		d.consecutiveSamePageNoMove = 0
	}
	if d.consecutiveSamePageNoMove >= d.noChangeThreshold {
		triggerSamePageNoMove = true
		d.lastReason = blockReasonSamePageNoChange
	}

	if !isBlockDetectorIgnoredAction(cmd.Act) && beforeSig != "" && afterSig != "" && beforeSig != afterSig {
		d.pushAfterSignature(afterSig)
		if d.isTailABAB() {
			d.consecutiveTwoStateLoops++
		} else {
			d.consecutiveTwoStateLoops = 0
		}
	} else {
		d.consecutiveTwoStateLoops = 0
	}
	if d.consecutiveTwoStateLoops >= d.twoStateLoopThreshold {
		triggerTwoStateLoop = true
		d.lastReason = blockReasonTwoStatePingPong
	}

	if d.isHighVisitLowReward(afterSig) {
		triggerHighVisitLowGain = true
		d.lastReason = blockReasonHighVisitLowGain
	}

	if triggerTwoStateLoop {
		d.lastReason = blockReasonTwoStatePingPong
	} else if triggerNoChangeScroll {
		d.lastReason = blockReasonScrollNoChange
	} else if triggerSamePageNoMove {
		d.lastReason = blockReasonSamePageNoChange
	} else if triggerHighVisitLowGain {
		d.lastReason = blockReasonHighVisitLowGain
	}

	return triggerNoChangeScroll || triggerSamePageNoMove || triggerTwoStateLoop || triggerHighVisitLowGain
}

func (d *blockDetector) Reset() {
	if d == nil {
		return
	}
	d.consecutiveNoChangeScroll = 0
	d.consecutiveSamePageNoMove = 0
	d.consecutiveTwoStateLoops = 0
	d.recentAfterSignatures = d.recentAfterSignatures[:0]
	d.recentObservedSignatures = d.recentObservedSignatures[:0]
	clear(d.pageVisitCount)
	d.lastReason = ""
}

func (d *blockDetector) LastReason() string {
	if d == nil || strings.TrimSpace(d.lastReason) == "" {
		return "unknown"
	}
	return d.lastReason
}

func (d *blockDetector) pushAfterSignature(sig string) {
	if d == nil || strings.TrimSpace(sig) == "" {
		return
	}
	d.recentAfterSignatures = append(d.recentAfterSignatures, sig)
	if len(d.recentAfterSignatures) > 8 {
		d.recentAfterSignatures = d.recentAfterSignatures[len(d.recentAfterSignatures)-8:]
	}
}

func (d *blockDetector) pushObservedSignature(sig string) {
	if d == nil || strings.TrimSpace(sig) == "" {
		return
	}
	d.recentObservedSignatures = append(d.recentObservedSignatures, sig)
	if len(d.recentObservedSignatures) > 16 {
		d.recentObservedSignatures = d.recentObservedSignatures[len(d.recentObservedSignatures)-16:]
	}
}

func (d *blockDetector) isTailABAB() bool {
	if d == nil || len(d.recentAfterSignatures) < 4 {
		return false
	}
	n := len(d.recentAfterSignatures)
	a := d.recentAfterSignatures[n-4]
	b := d.recentAfterSignatures[n-3]
	c := d.recentAfterSignatures[n-2]
	e := d.recentAfterSignatures[n-1]
	return a == c && b == e && a != b
}

func (d *blockDetector) isHighVisitLowReward(afterSig string) bool {
	if d == nil || strings.TrimSpace(afterSig) == "" {
		return false
	}
	if d.pageVisitCount[afterSig] < d.highVisitThreshold {
		return false
	}
	if d.lowRewardWindow <= 0 || len(d.recentObservedSignatures) < d.lowRewardWindow {
		return false
	}
	start := len(d.recentObservedSignatures) - d.lowRewardWindow
	tail := d.recentObservedSignatures[start:]
	unique := make(map[string]struct{}, len(tail))
	for _, sig := range tail {
		if sig == "" {
			continue
		}
		unique[sig] = struct{}{}
	}
	// 低收益定义：最近窗口主要停留在极少数页面簇（<=2）。
	return len(unique) <= 2
}

func isBlockDetectorIgnoredAction(act types.ActionType) bool {
	return act == types.NOP || act == types.START || act == types.RESTART || act == types.CLEAN_RESTART || act == types.ACTIVATE
}

func isAppRestartAction(act types.ActionType) bool {
	return act == types.START || act == types.RESTART || act == types.CLEAN_RESTART
}

func pageSignature(pageName string, xml string) string {
	name := strings.TrimSpace(pageName)
	content := strings.TrimSpace(xml)
	if name == "" && content == "" {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum64())
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
