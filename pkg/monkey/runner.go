package monkey

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/core/types"
	"trek/internal/engine/perception"
	"trek/internal/engine/recovery"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
	"trek/logger"
	"trek/pkg/coordinator"
	"trek/pkg/driver/android"
	"trek/pkg/driver/common"
)

const (
	defaultMaxSteps                                = 300
	defaultMaxDuration                             = 10 * time.Minute
	defaultStepInterval                            = 100 * time.Millisecond
	defaultMaxConsecutiveFailures                  = 8
	defaultFailureRecoveryInterval                 = 3
	defaultLongClickDuration                       = 800 * time.Millisecond
	defaultScrollDuration                          = 350 * time.Millisecond
	defaultScrollSteps                       int64 = 20
	defaultScrollRepeat                            = 3
	defaultPageSourceType                          = "uia"
	defaultForegroundMonitorInterval               = 300 * time.Millisecond
	defaultHealthSignalMonitorInterval             = 500 * time.Millisecond
	defaultOrientationMonitorInterval              = 300 * time.Millisecond
	defaultBlockNoChangeThreshold                  = 3
	defaultRecoveryCooldownSteps                   = 2
	defaultCandidateEnhancementMinStepGap          = 12
	defaultCandidateAmbiguityTopGapThreshold       = 0.15
	defaultHighValuePageVisitLimit                 = 2
	defaultCandidateRiskDropThreshold              = 2.1
	defaultCandidateMinFusionScore                 = -0.3
	defaultMaxRecoveryAttempts                     = 5
	maxRecentTraceEntries                          = 8
)

const (
	blockReasonSameActionNoChange = "same_action_no_change"
	blockReasonSamePageNoChange   = "same_page_no_change"
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

// PageNameResolverEx 扩展版页面名解析器，支持截图输入。
// 实现此接口可完全自定义页面名生成逻辑（如基于图片指纹）。
type PageNameResolverEx interface {
	ResolvePageName(xml string, screenshot []byte) string
}

// GojaPageNameResolver 通过 Goja 插件的 resolvePageName 钩子解析页面名。
type GojaPageNameResolver interface {
	ResolvePageNameWithInput(pageName string, input coordinator.ActionInput) (string, error)
}

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
	MixedMode                         bool
	PageControlStrategy               string
	CaptureScreenshot                 bool
	LongClickDuration                 time.Duration
	ScrollDuration                    time.Duration
	ScrollSteps                       int64
	ScrollRepeat                      int
	StopOnCrash                       bool
	StopOnANR                         bool
	KeepStepRecords                   bool
	PageNameResolver                  PageNameResolver
	PageNameResolverEx                PageNameResolverEx
	PageNameStrategy                  string
	ForegroundMonitorInterval         time.Duration
	HealthSignalMonitorInterval       time.Duration
	OrientationMonitorInterval        time.Duration
	EffectiveTouchAreas               []EffectiveTouchArea
	EnableBlockRecovery               *bool
	BlockNoChangeThreshold            int
	RecoveryCooldownSteps             int
	LLMBudgetMaxCalls                 int
	LLMBudgetWindowStep               int
	EnableExploreLLMEnhancement       *bool
	CandidateEnhancementMinStepGap    int
	CandidateAmbiguityTopGapThreshold float64
	HighValuePageVisitLimit           int
	CandidateRiskDropThreshold        float64
	CandidateMinFusionScore           float64
	MaxRecoveryAttempts               int // 恢复周期内最大尝试次数（0=不限制，默认 10）
	ImageSignatureFunc                func([]byte) string
	ImageFingerprintRegions           []ImageFingerprintRegion
	ImageFingerprintHammingThreshold  int
	InputCharset                      string
	DeviceWidth  int
	DeviceHeight int
	ArtifactDir                       string // 产物实时写入目录；为空则不实时写盘
}

type EffectiveTouchArea struct {
	Serial       string
	PackageName  string
	Orientations []ScreenOrientation
	Range        EffectiveTouchRange
}

type EffectiveTouchRange struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

type ImageFingerprintRegion struct {
	Left   float64
	Top    float64
	Right  float64
	Bottom float64
}

type ScreenOrientation string

const (
	ScreenOrientationPortrait        ScreenOrientation = "portrait"
	ScreenOrientationLandscapeLeft   ScreenOrientation = "landscape_left"
	ScreenOrientationPortraitReverse ScreenOrientation = "portrait_reverse"
	ScreenOrientationLandscapeRight  ScreenOrientation = "landscape_right"
)

// StepRecord 是每一步执行记录。
type StepRecord struct {
	Step               int
	PageName           string
	Action             string
	ActionTargetBounds string `json:"action_target_bounds,omitempty"`
	ActionWidgetInfo   string `json:"action_widget_info,omitempty"`
	TapPoint           string `json:"tap_point,omitempty"` // MotionTouch 实际触控坐标
	DurationMs         int64
	Err                string
	PageControlStrategy string          `json:"page_control_strategy,omitempty"`
	CacheHit           bool            `json:"cache_hit,omitempty"`
	ScriptTransformed  bool            `json:"script_transformed,omitempty"`
	BlockDetected      bool            `json:"block_detected,omitempty"`
	BlockReason        string          `json:"block_reason,omitempty"`
	BeforePageName     string          `json:"before_page_name,omitempty"`
	AfterPageName      string          `json:"after_page_name,omitempty"`
	BeforeXML          string          `json:"-"`
	AfterXML           string          `json:"-"`
	BeforeElement      types.IElement  `json:"-"`
	AfterElement       types.IElement  `json:"-"`
	BeforeScreenshot   []byte          `json:"-"`
	AfterScreenshot    []byte          `json:"-"`
	BeforeArtifactRef  *StepArtifactRef `json:"before_artifact,omitempty"`
	AfterArtifactRef   *StepArtifactRef `json:"after_artifact,omitempty"`
}

// StepArtifactRef 表示步骤关联产物在磁盘上的相对路径。
type StepArtifactRef struct {
	PageDir        string `json:"page_dir,omitempty"`
	ScreenshotFile string `json:"screenshot_file,omitempty"`
	XMLFile        string `json:"xml_file,omitempty"`
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
	BlockDetectionCount         int
	BlockReasonCount            map[string]int
	OutOfAppRecoveries          int
	RecoveryCooldownEnterCount  int
	RecoveryCooldownStepCount   int
	CandidateEnhancementCalls   int
	CandidateEnhancementSelects int
	RecoveryLLMCalls            int
	RecoveryLLMBudgetDenied     int
	EnhancementLLMBudgetDenied  int
	PlanCacheHits               int // LLM 响应缓存命中次数
	PlanCacheMisses             int // LLM 响应缓存未命中次数
	Records                     []StepRecord
}

// Decider 是动作决策接口，*coordinator.Coordinator 可直接满足。
type Decider interface {
	NextActionWithInput(pageName string, input coordinator.ActionInput) (*types.ActionCommand, error)
}

// PageInfoTransformer 是可选接口：允许决策层按 XML/截图自定义页面名与页面内容。
type PageInfoTransformer interface {
	TransformPageInfoWithInput(pageName string, input coordinator.ActionInput) (coordinator.PageInfo, error)
}

// WeightedCandidate 表示一个带权重的候选动作。
type WeightedCandidate struct {
	Command *types.ActionCommand
	Weight  float64
}

// WeightedDecider 是可选接口，用于返回多候选动作并由 Runner 按权重采样。
type WeightedDecider interface {
	NextWeightedActionsWithInput(pageName string, input coordinator.ActionInput) ([]WeightedCandidate, error)
}

// StepResultObserver 是可选接口，用于接收每步执行后的复盘信息。
type StepResultObserver interface {
	OnStepResult(result coordinator.StepResultInput) error
}

// TraversalOutcomeObserver 是可选接口，用于接收统一动作结果并回写算法在线学习。
type TraversalOutcomeObserver interface {
	ObserveTraversalOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error
}

// ContextAwareBlockRecoveryDecider 是可选恢复接口：当 Runner 识别到阻塞时，
// 由决策层提供恢复动作；未实现时 Runner 使用 BACK 兜底。
type ContextAwareBlockRecoveryDecider interface {
	NextBlockRecoveryActionWithContext(ctx enginestate.TraversalContext, input coordinator.ActionInput) (*types.ActionCommand, error)
}

// RecoveryCandidateProvider 聚合恢复阶段的候选来源。
// 当前仅使用 memory / heuristic；LLM 决策接口仅为兼容保留，不再参与恢复决策。
type RecoveryCandidateProvider interface {
	BuildMemoryRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
	BuildHeuristicRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
	BuildLLMRecoveryCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
}

// RecoveryCandidateSelector 在恢复阶段从融合候选集中选择最终动作。
type RecoveryCandidateSelector interface {
	SelectRecoveryAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error)
}

// AlgorithmCandidateProvider 提供主探索阶段的算法候选。
type AlgorithmCandidateProvider interface {
	BuildAlgorithmCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error)
}

// RecoveryMemoryWriter 在恢复动作执行后写回成功/失败经验，以及候选增强动作的正负样本。
type RecoveryMemoryWriter interface {
	RecordRecoveryMemoryOutcome(ctx enginestate.TraversalContext, item perception.Candidate, escaped bool) error
	RecordCandidateEnhancementOutcome(ctx enginestate.TraversalContext, item perception.Candidate, improved bool) error
}

// RecoveryActionHistoryProvider 提供可持久化的已知失败/成功恢复动作集合。
type RecoveryActionHistoryProvider interface {
	BuildKnownFailedRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error)
	BuildKnownSuccessfulRecoveryActions(ctx enginestate.TraversalContext) (map[string]bool, error)
}

type currentActivityProvider interface {
	GetCurrentActivity(ctx context.Context) (string, error)
}

type pageControlCacheInvalidator interface {
	InvalidatePageControlCache(screenshot []byte)
}

type pageControlCacheConsumptionMarker interface {
	MarkCacheConsumed(screenshot []byte)
	ResetConsumedMarks()
}

type planCacheInvalidator interface {
	InvalidatePlanCache(pageSignature string)
}

// Runner 执行 Smart Monkey 真机闭环。
type Runner struct {
	decider                Decider
	driver                 common.IDriver
	cfg                    Config
	rng                    *rand.Rand
	monitor                *foregroundPackageMonitor
	healthMonitor          *healthSignalMonitor
	orientationMonitor     *screenOrientationMonitor
	blockDetector          *blockDetector
	fuzzyMatcher           *FuzzyPageNameMatcher
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
	cachedScreenW          int
	cachedScreenH          int
	recoveryTriedBack      bool
	stepContextBuffer      []stepContext // 环形缓冲区，保存最近 2*threshold+4 步
	pendingDirectLLMPlan   bool          // 标记需要直接 LLM 规划
	directLLMHistorySteps  int           // 需要回溯的步数
	directLLMUsed          bool          // 本次阻塞周期是否已使用过直接 LLM
	backUselessPages       map[string]bool // 页面名 → BACK 从未成功逃离
	pageBlockCycles        map[string]int  // 页面名 → 完整阻塞周期数（Cooldown→再次阻塞）
}

type recoveryAttempt struct {
	ctx  enginestate.TraversalContext
	item perception.Candidate
}

type enhancementAttempt struct {
	ctx  enginestate.TraversalContext
	item perception.Candidate
	step int
}

// stepContext 记录单步执行的上下文，用于重复阻塞时构建 LLM 执行历史。
type stepContext struct {
	step        int
	action      string
	pageName    string
	afterPage   string
	escaped     bool
	blocked     bool
	blockReason string
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
	if cfg.LLMBudgetMaxCalls > 0 {
		enhanceBudget = recovery.NewSlidingWindowLLMBudget(
			cfg.LLMBudgetMaxCalls,
			cfg.LLMBudgetWindowStep,
		)
	}
	recoveryState := newRecoveryStateMachineWithCooldown(cfg.RecoveryCooldownSteps)
	maxRecoveryAttempts := cfg.MaxRecoveryAttempts
	if maxRecoveryAttempts <= 0 {
		maxRecoveryAttempts = defaultMaxRecoveryAttempts
	}
	recoveryState.SetMaxRecoveryAttempts(maxRecoveryAttempts)
	return &Runner{
		decider:                decider,
		driver:                 driver,
		cfg:                    cfg,
		rng:                    rand.New(rand.NewSource(time.Now().UnixNano())),
		blockDetector:          newBlockDetector(cfg.BlockNoChangeThreshold),
		fuzzyMatcher:           NewFuzzyPageNameMatcher(cfg.ImageFingerprintHammingThreshold),
		recoveryState:          recoveryState,
		candidateEnhanceBudget: enhanceBudget,
		lastEnhancementStep:    -1,
		recentTrace:            make([]enginestate.ActionTrace, 0, maxRecentTraceEntries),
		pageVisitCount:         make(map[string]int),
		actionCount:            make(map[string]int),
		recoveryFailedAction:   make(map[string]bool),
		recoverySuccessAction:  make(map[string]bool),
		backUselessPages:       make(map[string]bool),
		pageBlockCycles:        make(map[string]int),
	}, nil
}

// Run 启动闭环执行，返回运行报告。
func (r *Runner) Run(ctx context.Context) (*Report, error) {
	report := &Report{
		StartedAt:      time.Now(),
		StepsPlanned:   r.cfg.MaxSteps,
		ActionCount:    make(map[string]int),
		PageVisitCount: make(map[string]int),
		BlockReasonCount: make(map[string]int),
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
	if pageSource == nil && !r.isScreenshotPageSource() {
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

	// 启动后台截图线程（仅 Android 驱动支持）
	if ad, ok := r.driver.(*android.AndroidDriver); ok {
		ad.StartBackgroundScreenshot(ctx, 200*time.Millisecond)
		defer ad.StopBackgroundScreenshot()
	}
	defer r.stopForegroundPackageMonitor()
	r.startHealthSignalMonitor()
	defer r.stopHealthSignalMonitor()
	r.startOrientationMonitor()
	defer r.stopOrientationMonitor()
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
			r.markFailed(report, record, stepStart, nil, nil)
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
			r.appendRecord(report, record, stepStart, nil, nil)
			r.sleepStep(ctx, r.cfg.StepInterval)
			continue
		}

		var (
			xml        string
			screenshot []byte
			err        error
		)
		t0 := time.Now()
		if r.isScreenshotPageSource() {
			screenshot, err = r.driver.Screenshot(ctx)
			if err != nil || len(screenshot) == 0 {
				if err != nil {
					record.Err = err.Error()
					logger.Warnf("monkey step=%d screenshot page source failed: %v", step, err)
				} else {
					record.Err = "截图页面源为空"
					logger.Warnf("monkey step=%d screenshot page source is empty", step)
				}
				r.markFailed(report, record, stepStart, nil, nil)
				if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
					report.StopReason = StopMaxConsecutiveFailures
					return report, nil
				}
				r.tryRecover(report.ConsecutiveFailures)
				r.sleepStep(ctx, r.cfg.StepInterval)
				continue
			}
		} else if r.isMixedPageSource() {
			// mixed 模式：并发获取结构化 XML + 截图，减少 I/O 等待
			var (
				xmlErr     error
				screenshotErr error
				xmlDone    sync.WaitGroup
			)
			xmlDone.Add(1)
			go func() {
				defer xmlDone.Done()
				xml, xmlErr = pageSource.DumpPageSource()
			}()
			screenshot, screenshotErr = r.driver.Screenshot(ctx)
			xmlDone.Wait()
			err = xmlErr

			if xmlErr != nil {
				logger.Warnf("monkey step=%d mixed page source dump failed, fallback to screenshot only: %v", step, xmlErr)
				xml = ""
			}
			if screenshotErr != nil || len(screenshot) == 0 {
				if xml == "" {
					if screenshotErr != nil {
						record.Err = screenshotErr.Error()
					} else {
						record.Err = "mixed 模式: XML 和截图均为空"
					}
					r.markFailed(report, record, stepStart, nil, nil)
					if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
						report.StopReason = StopMaxConsecutiveFailures
						return report, nil
					}
					r.tryRecover(report.ConsecutiveFailures)
					r.sleepStep(ctx, r.cfg.StepInterval)
					continue
				}
				// XML 有值但截图失败，仅用 XML
				if screenshotErr != nil {
					logger.Warnf("monkey step=%d mixed screenshot failed, using XML only: %v", step, screenshotErr)
				}
			}
		} else {
			xml, err = pageSource.DumpPageSource()
			if err != nil {
				if r.shouldUseImagePageControlFallback() {
					screenshot, _ = r.driver.Screenshot(ctx)
					if len(screenshot) > 0 {
						logger.Warnf("monkey step=%d dump page source failed, fallback to screenshot strategy=%s: %v", step, r.cfg.PageControlStrategy, err)
						xml = ""
					} else {
						record.Err = err.Error()
						r.markFailed(report, record, stepStart, nil, nil)
						logger.Warnf("monkey step=%d dump page source failed: %v", step, err)
						if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
							report.StopReason = StopMaxConsecutiveFailures
							return report, nil
						}
						r.tryRecover(report.ConsecutiveFailures)
						r.sleepStep(ctx, r.cfg.StepInterval)
						continue
					}
				} else {
					record.Err = err.Error()
					r.markFailed(report, record, stepStart, nil, nil)
					logger.Warnf("monkey step=%d dump page source failed: %v", step, err)
					if report.ConsecutiveFailures >= r.cfg.MaxConsecutiveFailures {
						report.StopReason = StopMaxConsecutiveFailures
						return report, nil
					}
					r.tryRecover(report.ConsecutiveFailures)
					r.sleepStep(ctx, r.cfg.StepInterval)
					continue
				}
			}
		}

		cachedCrash, cachedANR, cachedReady := r.snapshotHealthSignals()
		if r.cfg.StopOnCrash && cachedReady && cachedCrash {
			report.StopReason = StopCrashDetectedLogcat
			record.Err = "检测到系统 crash 信号"
			r.appendRecord(report, record, stepStart, nil, nil)
			logger.Errorf("monkey stop on crash signal at step=%d", step)
			return report, nil
		}
		if r.cfg.StopOnANR && cachedReady && cachedANR {
			report.StopReason = StopANRDetectedLogcat
			record.Err = "检测到系统 ANR 信号"
			r.appendRecord(report, record, stepStart, nil, nil)
			logger.Errorf("monkey stop on anr signal at step=%d", step)
			return report, nil
		}

		if r.cfg.CaptureScreenshot && len(screenshot) == 0 {
			screenshot, _ = r.driver.Screenshot(ctx)
		}
		screenshot = r.cropScreenshotForEffectiveTouchArea(screenshot)
		logger.Debugf("step=%d timing: screenshot=%v", step, time.Since(t0))
		t1 := time.Now()
		pageName, xml, strategy, cacheHit, scriptTransformed, element := r.resolvePageInfo(ctx, xml, screenshot)
		logger.Debugf("step=%d timing: resolvePage=%v (strategy=%s cache=%v)", step, time.Since(t1), strategy, cacheHit)
		record.PageControlStrategy = strategy
		record.CacheHit = cacheHit
		record.ScriptTransformed = scriptTransformed

		beforePage := coordinator.PageSnapshot{
			PageName:   pageName,
			XML:        xml,
			Screenshot: screenshot,
			Signature:  pageSignature(pageName, xml),
			Element:    element,
		}

		record.PageName = pageName
		report.PageVisitCount[pageName]++

		input := coordinator.ActionInput{
			XMLDescOfGuiTree: xml,
			Screenshot:       screenshot,
		}

		t2 := time.Now()
		cmd, err := r.nextCommandWithRecovery(step, beforePage, pageName, input)
		logger.Debugf("step=%d timing: decision=%v", step, time.Since(t2))
		if err != nil {
			record.Err = err.Error()
			r.markFailed(report, record, stepStart, &beforePage, nil)
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
			r.markFailed(report, record, stepStart, &beforePage, nil)
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
		r.applyEffectiveTouchArea(step, cmd, xml)
		r.toAbsoluteCoordinates(cmd, screenshot)

		record.Action = cmd.Act.String()
		record.ActionTargetBounds = cmd.Pos.String()
		record.ActionWidgetInfo = strings.TrimSpace(cmd.WidgetInfo)
		record.TapPoint = strings.TrimSpace(formatTapPointLog(cmd))
		report.ActionCount[record.Action]++
		report.StepsTotal++
		r.recordActionTrace(beforePage, cmd)

		logger.Infof("monkey step=%d execute cmd={%s}%s%s", step, cmd.DetailLogString(), formatTapPointLog(cmd), formatSwipePointLog(cmd))

		t3 := time.Now()
		if err = r.execute(cmd); err != nil {
			record.Err = err.Error()
			afterPage := r.capturePageSnapshot(ctx, pageSource, pageName)
			if afterPage != nil {
				afterPage.Screenshot = beforePage.Screenshot
			}
			r.invalidatePageControlCache(beforePage.Screenshot)
			r.notifyTraversalOutcome(step, beforePage, afterPage, cmd, false)
			r.recordRecoveryOutcome(false)
			crash, anr := r.currentHealthSignals()
			r.notifyStepResult(step, cmd, false, err.Error(), time.Since(stepStart).Milliseconds(), crash, anr, beforePage, afterPage)
			r.markFailed(report, record, stepStart, &beforePage, afterPage)
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
		logger.Debugf("step=%d timing: execute=%v", step, time.Since(t3))

		// 立即标记动作完成（在 capturePageSnapshot 之前），
		// 让后台截图线程有足够时间产出动作后的新帧
		if ad, ok := r.driver.(*android.AndroidDriver); ok {
			ad.MarkActionDone()
		}

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

		t4 := time.Now()
		afterPage := r.capturePageSnapshot(ctx, pageSource, pageName)
		logger.Debugf("step=%d timing: afterPage=%v", step, time.Since(t4))
		if afterPage != nil {
			afterPage.Screenshot = beforePage.Screenshot
		}
		escaped := afterPage != nil &&
			beforePage.Signature != afterPage.Signature
		if cmd.Act == types.INPUT {
			escaped = true // 输入必然改变页面内容，视为已逃离
		}
		r.notifyTraversalOutcome(step, beforePage, afterPage, cmd, true)
		// 记录 step 上下文到环形缓冲区
		afterPageName := ""
		if afterPage != nil {
			afterPageName = afterPage.PageName
		}
		r.appendStepContext(stepContext{
			step:      step,
			action:    record.Action + " " + record.ActionTargetBounds,
			pageName:  pageName,
			afterPage: afterPageName,
			escaped:   escaped,
		})
		if r.shouldEnableBlockRecovery() && r.blockDetector.Observe(pageName, buildActionKey(cmd), cmd.Act, escaped) {
			blockReason := r.blockDetector.LastReason()
			r.blockDetector.RecordBlockStep(pageName, step)
			// 检查是否触发直接 LLM 规划
			if !r.directLLMUsed && r.blockDetector.CheckRepeatBlock(pageName, step, 2) {
				r.pendingDirectLLMPlan = true
				r.directLLMHistorySteps = 2 * r.blockDetector.threshold
				logger.Infof("repeat block detected on page %s at step %d, will use direct LLM planning", pageName, step)
			}
			r.handleBlockDetectedWithPage(blockReason, &beforePage)
			report.BlockDetectionCount++
			report.BlockReasonCount[blockReason]++
			record.BlockDetected = true
			record.BlockReason = blockReason
			// 更新 step context 中的 block 信息
			r.updateLastStepContextBlock(blockReason)
			logger.Warnf("monkey step=%d detected block loop reason=%s mode=%s pending=%t",
				step, blockReason, r.recoveryState.Mode(), r.pendingBlockRecovery)
			if r.pendingBlockRecovery {
				r.blockDetector.Reset()
			}
		} else if afterPage != nil && r.recoveryState != nil && r.recoveryState.Mode() == TraversalModeRecover {
			r.handleProgress(escaped)
		}
		if escaped {
			r.directLLMUsed = false
			// 页面变化，重置消费标记和 LLM 响应缓存
			r.resetConsumedMarks()
			r.invalidatePlanCache(cachedSignature(beforePage))
		}
		r.recordRecoveryOutcome(escaped)
		crash, anr := r.currentHealthSignals()
		r.notifyStepResult(step, cmd, true, "", time.Since(stepStart).Milliseconds(), crash, anr, beforePage, afterPage)
		r.appendRecord(report, record, stepStart, &beforePage, afterPage)
		logger.Debugf("monkey step=%d execute action=%s success (total=%v)", step, cmd.Act.String(), time.Since(stepStart))
		r.sleepStep(ctx, r.resolveStepDelay(cmd))
	}

	report.StopReason = StopCompleted
	return report, nil
}

func (r *Runner) capturePageSnapshot(ctx context.Context, pageSource common.IPageSource, fallbackPageName string) *coordinator.PageSnapshot {
	if pageSource == nil && !r.isScreenshotPageSource() {
		return nil
	}
	xml := ""
	if pageSource != nil {
		var err error
		xml, err = pageSource.DumpPageSource()
		if err != nil {
			return nil
		}
	}
	if xml == "" {
		return nil
	}
	pageName := r.resolvePageNameByExOrFallback(xml, nil)
	if strings.TrimSpace(pageName) == "" {
		pageName = fallbackPageName
	}
	nextPageName, nextXML, _, _, _, nextElement := r.resolvePageInfo(ctx, xml, nil)
	if strings.TrimSpace(nextPageName) == "" {
		nextPageName = pageName
	}
	return &coordinator.PageSnapshot{
		PageName:  nextPageName,
		XML:       nextXML,
		Signature: pageSignature(nextPageName, nextXML),
		Element:   nextElement,
	}
}

func (r *Runner) invalidatePageControlCache(screenshot []byte) {
	invalidator, ok := r.decider.(pageControlCacheInvalidator)
	if !ok || invalidator == nil || len(screenshot) == 0 {
		return
	}
	invalidator.InvalidatePageControlCache(screenshot)
}

func (r *Runner) markCacheConsumed(screenshot []byte) {
	marker, ok := r.decider.(pageControlCacheConsumptionMarker)
	if !ok || marker == nil || len(screenshot) == 0 {
		return
	}
	marker.MarkCacheConsumed(screenshot)
}

func (r *Runner) resetConsumedMarks() {
	marker, ok := r.decider.(pageControlCacheConsumptionMarker)
	if !ok || marker == nil {
		return
	}
	marker.ResetConsumedMarks()
}

func (r *Runner) invalidatePlanCache(pageSignature string) {
	invalidator, ok := r.decider.(planCacheInvalidator)
	if !ok || invalidator == nil || pageSignature == "" {
		return
	}
	invalidator.InvalidatePlanCache(pageSignature)
}

func (r *Runner) notifyStepResult(step int, cmd *types.ActionCommand, success bool, errText string, durationMs int64, crash bool, anr bool, before coordinator.PageSnapshot, after *coordinator.PageSnapshot) {
	observer, ok := r.decider.(StepResultObserver)
	if !ok || observer == nil {
		return
	}
	_ = observer.OnStepResult(coordinator.StepResultInput{
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

func (r *Runner) notifyTraversalOutcome(step int, before coordinator.PageSnapshot, after *coordinator.PageSnapshot, cmd *types.ActionCommand, success bool) {
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
	before coordinator.PageSnapshot,
	after *coordinator.PageSnapshot,
	success bool,
) traversal.ActionOutcome {
	if cmd == nil || !success || after == nil {
		return traversal.OutcomeNoOp
	}
	if cmd.Act == types.NOP {
		return traversal.OutcomeNoOp
	}
	beforeSig := cachedSignature(before)
	afterSig := cachedSignaturePtr(after)
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

func (r *Runner) nextCommand(pageName string, input coordinator.ActionInput) (*types.ActionCommand, error) {
	cmd, _, err := r.nextCommandWithCandidates(pageName, input)
	return cmd, err
}

func (r *Runner) nextCommandWithCandidates(pageName string, input coordinator.ActionInput) (*types.ActionCommand, []WeightedCandidate, error) {
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

func (r *Runner) detectCrashBySystem() bool {
	crash, err := r.driver.CheckCrash(r.cfg.PackageName)
	return err == nil && crash
}

func (r *Runner) detectANRBySystem() bool {
	anr, err := r.driver.CheckANR(r.cfg.PackageName)
	return err == nil && anr
}

func (r *Runner) resolvePageInfo(ctx context.Context, xml string, screenshot []byte) (string, string, string, bool, bool, types.IElement) {
	pageName := r.resolveBasePageName(ctx, xml, screenshot)
	// Goja resolvePageName 钩子：允许插件自定义页面名生成
	if resolver, ok := r.decider.(GojaPageNameResolver); ok && resolver != nil {
		customName, err := resolver.ResolvePageNameWithInput(pageName, coordinator.ActionInput{
			XMLDescOfGuiTree: xml,
			Screenshot:       screenshot,
		})
		if err == nil && customName != "" {
			pageName = customName
		}
	}
	transformer, ok := r.decider.(PageInfoTransformer)
	if !ok || transformer == nil {
		return pageName, xml, r.cfg.PageControlStrategy, false, false, nil
	}
	pageInfo, err := transformer.TransformPageInfoWithInput(pageName, coordinator.ActionInput{
		XMLDescOfGuiTree: xml,
		Screenshot:       screenshot,
	})
	if err != nil {
		logger.Warnf("transform page info failed, use fallback: %v", err)
		return pageName, xml, r.cfg.PageControlStrategy, false, false, nil
	}
	nextPageName := strings.TrimSpace(pageInfo.PageName)
	if nextPageName == "" {
		nextPageName = pageName
	}
	nextXML := strings.TrimSpace(pageInfo.XML)
	if nextXML == "" {
		nextXML = xml
	}
	return nextPageName, nextXML, r.cfg.PageControlStrategy, pageInfo.CacheHit, pageInfo.ScriptTransformed, pageInfo.Element
}

func (r *Runner) resolveBasePageName(ctx context.Context, xml string, screenshot []byte) string {
	return resolveBasePageNameByStrategy(ctx, r, xml, screenshot)
}

func (r *Runner) isScreenshotPageSource() bool {
	return strings.EqualFold(strings.TrimSpace(r.cfg.PageSourceType), "screenshot")
}

func (r *Runner) isMixedPageSource() bool {
	return r.cfg.MixedMode
}

// resolvePageNameByExOrFallback 优先使用 PageNameResolverEx，否则 fallback 到 PageNameResolver。
func (r *Runner) resolvePageNameByExOrFallback(xml string, screenshot []byte) string {
	if r.cfg.PageNameResolverEx != nil {
		return r.cfg.PageNameResolverEx.ResolvePageName(xml, screenshot)
	}
	if r.cfg.PageNameResolver != nil {
		return r.cfg.PageNameResolver(xml)
	}
	return defaultPageNameResolver(xml)
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

func (r *Runner) shouldUseImagePageControlFallback() bool {
	switch strings.ToLower(strings.TrimSpace(r.cfg.PageControlStrategy)) {
	case "ocr", "llm", "chain":
		return true
	default:
		return false
	}
}

// appendStepContext 将单步上下文推入环形缓冲区。
func (r *Runner) appendStepContext(sc stepContext) {
	const maxStepContextBuffer = 20
	r.stepContextBuffer = append(r.stepContextBuffer, sc)
	if len(r.stepContextBuffer) > maxStepContextBuffer {
		r.stepContextBuffer = r.stepContextBuffer[len(r.stepContextBuffer)-maxStepContextBuffer:]
	}
}

// collectStepContextHistory 从缓冲区取最近 n 步的上下文。
func (r *Runner) collectStepContextHistory(n int) []stepContext {
	if n <= 0 || len(r.stepContextBuffer) == 0 {
		return nil
	}
	start := len(r.stepContextBuffer) - n
	if start < 0 {
		start = 0
	}
	result := make([]stepContext, len(r.stepContextBuffer[start:]))
	copy(result, r.stepContextBuffer[start:])
	return result
}

// buildExecutionHistory 将 stepContext 列表转为 LLM 可用的 ExecutionRecord 列表。
func (r *Runner) buildExecutionHistory(history []stepContext) []enginestate.ExecutionRecord {
	if len(history) == 0 {
		return nil
	}
	records := make([]enginestate.ExecutionRecord, 0, len(history))
	for _, sc := range history {
		records = append(records, enginestate.ExecutionRecord{
			Step:        sc.step,
			Action:      sc.action,
			PageName:    sc.pageName,
			AfterPage:   sc.afterPage,
			Escaped:     sc.escaped,
			Blocked:     sc.blocked,
			BlockReason: sc.blockReason,
		})
	}
	return records
}

// buildTraversalContextWithHistory 构建带有执行历史的 TraversalContext。
func (r *Runner) buildTraversalContextWithHistory(step int, page coordinator.PageSnapshot, history []enginestate.ExecutionRecord) enginestate.TraversalContext {
	mode := TraversalModeExplore
	blockReason := ""
	if r != nil && r.recoveryState != nil {
		mode = r.recoveryState.Mode()
		blockReason = r.recoveryState.BlockReason()
	}
	signature := cachedSignature(page)
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
		PageVisitCount:   r.pageVisitCount,
		ActionCount:      r.actionCount,
		ExecutionHistory: history,
	})
}

// updateLastStepContextBlock 回填最近一步的 block 信息到 stepContext 缓冲区。
func (r *Runner) updateLastStepContextBlock(blockReason string) {
	if len(r.stepContextBuffer) == 0 {
		return
	}
	last := &r.stepContextBuffer[len(r.stepContextBuffer)-1]
	last.blocked = true
	last.blockReason = blockReason
}
