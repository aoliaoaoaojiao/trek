/*
Copyright © 2026 Trek
*/
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"trek/internal/engine/decision"
	"trek/internal/scripting"
	"trek/logger"
	"trek/pkg/coordinator"
	"trek/pkg/driver/android"
	"trek/pkg/monkey"

	"github.com/spf13/cobra"
)

// runOptions 存储 run 子命令的标志值。
var runOptions = struct {
	packageName       string
	deviceSerial      string
	configPath        string
	algorithm         string
	maxSteps          int
	maxDuration       time.Duration
	stepInterval      time.Duration
	captureScreenshot bool
	keepStepRecords   bool
	probePageName     bool
	autoCurrentApp    bool
}{}

var (
	captureScreenshotCLISet bool
	keepStepRecordsCLISet   bool
)

// runCmd 定义 run 子命令。
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "执行 monkey 测试",
	Long:  `在连接的 Android 设备上执行 Smart Monkey UI 自动化遍历测试。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		captureScreenshotCLISet = cmd.Flags().Lookup("capture-screenshot").Changed
		keepStepRecordsCLISet = cmd.Flags().Lookup("keep-step-records").Changed
		return runMonkey(logLevel, runOptions)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&runOptions.packageName, "package", "", "被测应用包名（必填，或使用 --auto-current-app 自动获取）")
	runCmd.Flags().StringVar(&runOptions.deviceSerial, "serial", "", "设备序列号（可选，默认自动选择）")
	runCmd.Flags().StringVar(&runOptions.configPath, "config", "", "配置文件路径（可选，仅支持 .js，支持绝对/相对路径）")
	runCmd.Flags().StringVar(&runOptions.algorithm, "algorithm", "reuse", "决策算法（可选: reuse, uctbandit）")
	runCmd.Flags().IntVar(&runOptions.maxSteps, "max-steps", 300, "最大执行步数")
	runCmd.Flags().DurationVar(&runOptions.maxDuration, "max-duration", 10*time.Minute, "最大运行时长")
	runCmd.Flags().DurationVar(&runOptions.stepInterval, "step-interval", 300*time.Millisecond, "基础步进间隔")
	runCmd.Flags().BoolVar(&runOptions.captureScreenshot, "capture-screenshot", false, "是否采集截图给决策层")
	runCmd.Flags().BoolVar(&runOptions.keepStepRecords, "keep-step-records", true, "是否保留每步记录")
	runCmd.Flags().BoolVar(&runOptions.probePageName, "probe-page-name", false, "仅探测当前页面名后退出")
	runCmd.Flags().BoolVar(&runOptions.autoCurrentApp, "auto-current-app", false, "自动使用当前前台应用进行测试")
}

// runMonkey 执行 monkey 测试的核心逻辑。
func runMonkey(logLevelStr string, opts struct {
	packageName       string
	deviceSerial      string
	configPath        string
	algorithm         string
	maxSteps          int
	maxDuration       time.Duration
	stepInterval      time.Duration
	captureScreenshot bool
	keepStepRecords   bool
	probePageName     bool
	autoCurrentApp    bool
}) error {
	if err := logger.SetLevel(logLevelStr); err != nil {
		return fmt.Errorf("设置日志级别失败: %w", err)
	}

	staticCfg := scripting.StaticConfig{}
	if opts.configPath != "" {
		cfg, err := scripting.LoadStaticConfigFile(opts.configPath)
		if err != nil {
			return fmt.Errorf("读取配置文件失败(%s): %w", opts.configPath, err)
		}
		staticCfg = cfg
	}

	// JS 配置文件中的 algorithm 可覆盖 CLI 标志默认值
	if staticCfg.Algorithm != "" && opts.algorithm == "reuse" {
		// 仅在 CLI 未显式指定（仍为默认值）时，使用配置文件中的值
		opts.algorithm = staticCfg.Algorithm
	}
	if staticCfg.CaptureScreenshot.IsSet() && !captureScreenshotCLISet {
		opts.captureScreenshot = staticCfg.CaptureScreenshot.Get()
	}
	if staticCfg.KeepStepRecords.IsSet() && !keepStepRecordsCLISet {
		opts.keepStepRecords = staticCfg.KeepStepRecords.Get()
	}
	exploreOCRTimeout := time.Duration(staticCfg.ExploreOCRTimeoutMs.OrDefault(10000)) * time.Millisecond
	recoveryCooldownSteps := staticCfg.RecoveryCooldownSteps.OrDefault(2)
	llmTimeout := time.Duration(staticCfg.LLMTimeoutMs.OrDefault(15000)) * time.Millisecond
	llmMaxCalls := staticCfg.LLMMaxCalls.OrDefault(0)
	llmWindowSteps := staticCfg.LLMWindowSteps.OrDefault(0)
	recoveryTwoStateLoopThreshold := staticCfg.RecoveryTwoStateLoopThreshold.OrDefault(2)
	recoveryHighVisitThreshold := staticCfg.RecoveryHighVisitThreshold.OrDefault(8)
	recoveryLowRewardWindow := staticCfg.RecoveryLowRewardWindow.OrDefault(6)
	candidateAmbiguityTopGapThreshold := staticCfg.CandidateAmbiguityTopGapThreshold.OrDefault(0.15)
	highValuePageVisitLimit := staticCfg.HighValuePageVisitLimit.OrDefault(2)
	candidateRiskDropThreshold := staticCfg.CandidateRiskDropThreshold.OrDefault(2.1)
	candidateMinFusionScore := staticCfg.CandidateMinFusionScore.OrDefault(-0.3)
	pageControlStrategy := strings.TrimSpace(staticCfg.PageControlStrategy)

	if normalizePageControlStrategy(pageControlStrategy) != pageControlStrategyRaw {
		opts.captureScreenshot = true
	}

	pageSourceType, err := android.ResolvePageSourceType(staticCfg.PageSource)
	if err != nil {
		return err
	}
	touchMode, touchType, err := android.ResolveTouchMode(staticCfg.TouchMode)
	if err != nil {
		return err
	}

	driverOptions, err := android.BuildDriverOptions(android.DriverBootstrapConfig{
		PageSource:    staticCfg.PageSource,
		TouchMode:     staticCfg.TouchMode,
		UIAServerPort: staticCfg.UIA.ServerPort,
		PocoEngine:    staticCfg.Poco.Engine,
		PocoPort:      staticCfg.Poco.Port,
	}, pageSourceType, touchType)
	if err != nil {
		return err
	}
	deviceSerial := strings.TrimSpace(opts.deviceSerial)
	if deviceSerial == "" && staticCfg.EffectiveTouchArea != nil {
		if scopedSerial := strings.TrimSpace(staticCfg.EffectiveTouchArea.Serial); scopedSerial != "" {
			deviceSerial = scopedSerial
			fmt.Printf("未指定 -serial，使用 effective_touch_area.serial: %s\n", deviceSerial)
		}
	}

	driver, err := android.NewAndroidDriverWith(deviceSerial, driverOptions...)
	if err != nil {
		return fmt.Errorf("创建设备驱动失败: %w", err)
	}
	defer func() { _ = driver.Close() }()
	fmt.Printf("运行参数: page_source=%s touch_mode=%s\n", pageSourceType, touchMode)

	packageName := opts.packageName
	if opts.autoCurrentApp {
		pkg, err := driver.GetCurrentPackage(context.Background())
		if err != nil {
			return fmt.Errorf("获取当前前台应用失败: %w", err)
		}
		packageName = pkg
		fmt.Printf("自动检测到前台应用: %s\n", packageName)
	}

	if packageName == "" {
		return fmt.Errorf("参数 --package 不能为空，或使用 --auto-current-app 自动获取")
	}

	// 解析决策算法类型
	algorithmType, err := resolveAlgorithmType(opts.algorithm)
	if err != nil {
		return err
	}

	coord, err := coordinator.New(coordinator.Config{
		PackageName:         packageName,
		Algorithm:           algorithmType,
		ExploreOCRTimeout:   exploreOCRTimeout,
		RecoveryLLMTimeout:  llmTimeout,
		PageControlStrategy: pageControlStrategy,
	})
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	if opts.configPath != "" {
		if err := coord.LoadConfigFile(opts.configPath); err != nil {
			return fmt.Errorf("加载配置文件失败(%s): %w", opts.configPath, err)
		}
		fmt.Printf("配置加载成功: %s\n", opts.configPath)
	}

	cfg := monkey.Config{
		PackageName:                       packageName,
		DeviceSerial:                      deviceSerial,
		MaxSteps:                          opts.maxSteps,
		MaxDuration:                       opts.maxDuration,
		StepInterval:                      opts.stepInterval,
		PageSourceType:                    pageSourceType,
		PageNameStrategy:                  strings.TrimSpace(staticCfg.PageNameStrategy),
		PageControlStrategy:               pageControlStrategy,
		CaptureScreenshot:                 opts.captureScreenshot,
		KeepStepRecords:                   opts.keepStepRecords,
		StopOnCrash:                       true,
		StopOnANR:                         true,
		EffectiveTouchAreas:               buildEffectiveTouchAreasConfig(staticCfg, packageName, deviceSerial),
		RecoveryCooldownSteps:             recoveryCooldownSteps,
		LLMBudgetMaxCalls:                 llmMaxCalls,
		LLMBudgetWindowStep:               llmWindowSteps,
		TwoStateLoopThreshold:             recoveryTwoStateLoopThreshold,
		HighVisitThreshold:                recoveryHighVisitThreshold,
		LowRewardWindow:                   recoveryLowRewardWindow,
		CandidateAmbiguityTopGapThreshold: candidateAmbiguityTopGapThreshold,
		HighValuePageVisitLimit:           highValuePageVisitLimit,
		CandidateRiskDropThreshold:        candidateRiskDropThreshold,
		CandidateMinFusionScore:           candidateMinFusionScore,
		ImageFingerprintRegions:           buildImageFingerprintRegionsConfig(staticCfg),
	}

	if opts.probePageName {
		return probePageName(driver, cfg)
	}

	runner, err := monkey.NewRunner(coord, driver, cfg)
	if err != nil {
		return fmt.Errorf("创建 runner 失败: %w", err)
	}

	report, err := runner.Run(context.Background())
	if err != nil {
		return fmt.Errorf("执行 monkey 失败: %w", err)
	}

	fmt.Printf("运行完成: stop_reason=%s total=%d success=%d failed=%d duration_ms=%d\n",
		report.StopReason, report.StepsTotal, report.StepsSucceeded, report.StepsFailed, report.DurationMs)
	fmt.Printf("页面统计: %+v\n", report.PageVisitCount)
	fmt.Printf("恢复冷却统计: cooldown_enter=%d cooldown_step_hits=%d\n",
		report.RecoveryCooldownEnterCount, report.RecoveryCooldownStepCount)
	fmt.Printf("LLM 决策统计（兼容字段，当前固定为 0）: recovery_calls=%d recovery_budget_denied=%d enhancement_calls=%d enhancement_hits=%d enhancement_budget_denied=%d\n",
		report.RecoveryLLMCalls, report.RecoveryLLMBudgetDenied, report.CandidateEnhancementCalls, report.CandidateEnhancementSelects, report.EnhancementLLMBudgetDenied)
	return nil
}

const (
	pageControlStrategyRaw = "raw"
	pageControlStrategyOCR = "ocr"
	pageControlStrategyLLM = "llm"
)

func normalizePageControlStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "", pageControlStrategyRaw:
		return pageControlStrategyRaw
	case pageControlStrategyOCR:
		return pageControlStrategyOCR
	case pageControlStrategyLLM:
		return pageControlStrategyLLM
	default:
		return pageControlStrategyRaw
	}
}

// probePageName 输出当前程序判定的页面名，便于调试页面识别逻辑。
func probePageName(driver *android.AndroidDriver, cfg monkey.Config) error {
	pageSource := driver.GetPageSource(cfg.PageSourceType)
	if pageSource == nil {
		return fmt.Errorf("页面源不可用: %s", cfg.PageSourceType)
	}
	xml, err := pageSource.DumpPageSource()
	if err != nil {
		return fmt.Errorf("抓取页面源失败: %w", err)
	}
	pageName := monkey.ResolvePageName(xml, cfg.PageNameResolver)
	fmt.Printf("当前页面名: %s\n", pageName)
	return nil
}

// resolveAlgorithmType 将字符串解析为决策算法类型。
func resolveAlgorithmType(text string) (decision.AlgorithmType, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "reuse", "4":
		return decision.AlgorithmReuse, nil
	case "uctbandit", "uct", "7":
		return decision.AlgorithmUctBandit, nil
	case "random", "0":
		return decision.AlgorithmRandom, nil
	default:
		return decision.AlgorithmReuse, nil
	}
}

// buildEffectiveTouchAreaConfig 从静态配置构建有效触控区域配置。
func buildEffectiveTouchAreasConfig(staticCfg scripting.StaticConfig, packageName string, deviceSerial string) []monkey.EffectiveTouchArea {
	if staticCfg.EffectiveTouchArea == nil {
		return nil
	}
	rangeCfg := staticCfg.EffectiveTouchArea.Range
	area := monkey.EffectiveTouchArea{
		Serial:      strings.TrimSpace(staticCfg.EffectiveTouchArea.Serial),
		PackageName: strings.TrimSpace(staticCfg.EffectiveTouchArea.PackageName),
		Range: monkey.EffectiveTouchRange{
			Left:   rangeCfg.Left,
			Top:    rangeCfg.Top,
			Right:  rangeCfg.Right,
			Bottom: rangeCfg.Bottom,
		},
	}
	if area.Serial == "" {
		area.Serial = strings.TrimSpace(deviceSerial)
	}
	if area.PackageName == "" {
		area.PackageName = strings.TrimSpace(packageName)
	}
	return []monkey.EffectiveTouchArea{area}
}

func buildImageFingerprintRegionsConfig(staticCfg scripting.StaticConfig) []monkey.ImageFingerprintRegion {
	if len(staticCfg.ImageFingerprintRegions) == 0 {
		return nil
	}
	regions := make([]monkey.ImageFingerprintRegion, 0, len(staticCfg.ImageFingerprintRegions))
	for _, region := range staticCfg.ImageFingerprintRegions {
		regions = append(regions, monkey.ImageFingerprintRegion{
			Left:   region.Left,
			Top:    region.Top,
			Right:  region.Right,
			Bottom: region.Bottom,
		})
	}
	return regions
}
