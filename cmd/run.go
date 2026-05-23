/*
Copyright © 2026 Trek
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"trek/internal/config"
	"trek/internal/engine/decision"
	"trek/internal/reporting"
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
	reportFile        string
	reportFormat      string
	artifactDir       string
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
	runCmd.Flags().StringVar(&runOptions.reportFile, "report-file", "", "运行报告输出路径（可选，支持 .json/.md）")
	runCmd.Flags().StringVar(&runOptions.reportFormat, "report-format", "", "运行报告格式（可选：json、md；默认按 report-file 扩展名推断）")
	runCmd.Flags().StringVar(&runOptions.artifactDir, "artifact-dir", "", "页面产物输出目录（可选；默认跟随 report-file 自动生成同名目录）")
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
	reportFile        string
	reportFormat      string
	artifactDir       string
}) error {
	if err := logger.SetLevel(logLevelStr); err != nil {
		return fmt.Errorf("设置日志级别失败: %w", err)
	}

	// 可取消 context + 信号处理：Ctrl+C 触发优雅退出
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		logger.Warnf("收到中断信号，正在退出...")
		cancel()
		<-sigCh
		logger.Warnf("二次中断，强制退出")
		os.Exit(1)
	}()

	// 提前解析产物目录，供 Config 实时写盘使用
	reportFile := strings.TrimSpace(opts.reportFile)
	artifactDir := resolveArtifactDir(reportFile, opts.artifactDir)

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
	if strings.TrimSpace(opts.reportFile) != "" || strings.TrimSpace(opts.artifactDir) != "" {
		if !opts.keepStepRecords {
			fmt.Println("检测到报告/产物输出需求，已自动启用 keep-step-records")
		}
		opts.keepStepRecords = true
	}
	exploreOCRTimeout := time.Duration(staticCfg.ExploreOCRTimeoutMs.OrDefault(10000)) * time.Millisecond
	recoveryCooldownSteps := staticCfg.RecoveryCooldownSteps.OrDefault(2)
	llmTimeout := time.Duration(staticCfg.LLMTimeoutMs.OrDefault(15000)) * time.Millisecond

	// 统一全局超时：goja trek.llm.chat() / trek.ocr.recognize() 默认使用配置值
	scripting.SetDefaultLLMTimeout(llmTimeout)
	scripting.SetDefaultOCRTimeout(exploreOCRTimeout)
	recoveryTwoStateLoopThreshold := staticCfg.RecoveryTwoStateLoopThreshold.OrDefault(2)
	recoveryHighVisitThreshold := staticCfg.RecoveryHighVisitThreshold.OrDefault(8)
	recoveryLowRewardWindow := staticCfg.RecoveryLowRewardWindow.OrDefault(6)
	candidateAmbiguityTopGapThreshold := staticCfg.CandidateAmbiguityTopGapThreshold.OrDefault(0.15)
	highValuePageVisitLimit := staticCfg.HighValuePageVisitLimit.OrDefault(2)
	candidateRiskDropThreshold := staticCfg.CandidateRiskDropThreshold.OrDefault(2.1)
	candidateMinFusionScore := staticCfg.CandidateMinFusionScore.OrDefault(-0.3)
	pageControlStrategy := strings.TrimSpace(staticCfg.PageControlStrategy)
	pageControlCacheTTL := time.Duration(staticCfg.PageControlCacheTTLSeconds.OrDefault(0)) * time.Second

	if normalizePageControlStrategy(pageControlStrategy) != pageControlStrategyRaw {
		opts.captureScreenshot = true
	}

	pageSourceType, err := android.ResolvePageSourceType(staticCfg.PageSource)
	if err != nil {
		return err
	}
	if pageSourceType == "screenshot" {
		opts.captureScreenshot = true
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
		PackageName:          packageName,
		Algorithm:            algorithmType,
		ExploreOCRTimeout:    exploreOCRTimeout,
		RecoveryLLMTimeout:   llmTimeout,
		PageControlStrategy:  pageControlStrategy,
		PageControlCacheFile: strings.TrimSpace(staticCfg.PageControlCacheFile),
		PageControlCacheTTL:  pageControlCacheTTL,
	})
	if err != nil {
		return fmt.Errorf("创建会话失败: %w", err)
	}
	defer func() { _ = coord.Close() }()
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
		TwoStateLoopThreshold:             recoveryTwoStateLoopThreshold,
		HighVisitThreshold:                recoveryHighVisitThreshold,
		LowRewardWindow:                   recoveryLowRewardWindow,
		CandidateAmbiguityTopGapThreshold: candidateAmbiguityTopGapThreshold,
		HighValuePageVisitLimit:           highValuePageVisitLimit,
		CandidateRiskDropThreshold:        candidateRiskDropThreshold,
		CandidateMinFusionScore:           candidateMinFusionScore,
		ImageFingerprintRegions:           buildImageFingerprintRegionsConfig(staticCfg),
		ImageSimilaritySSIMThreshold:      staticCfg.ImageSimilaritySSIMThreshold.OrDefault(0),
		ImageFingerprintHammingThreshold:  staticCfg.ImageFingerprintHammingThreshold.OrDefault(config.DefaultImageFingerprintHammingThreshold),
		ArtifactDir:                       artifactDir,
	}

	if opts.probePageName {
		return probePageName(driver, cfg)
	}

	runner, err := monkey.NewRunner(coord, driver, cfg)
	if err != nil {
		return fmt.Errorf("创建 runner 失败: %w", err)
	}

	// defer 保证无论正常结束还是 Ctrl+C 中断，都会写出报告和产物
	metadata := reporting.RunMetadata{
		PackageName:         packageName,
		DeviceSerial:        deviceSerial,
		Algorithm:           opts.algorithm,
		MaxSteps:            opts.maxSteps,
		MaxDuration:         opts.maxDuration,
		StepInterval:        opts.stepInterval,
		PageSourceType:      pageSourceType,
		PageControlStrategy: pageControlStrategy,
		CaptureScreenshot:   opts.captureScreenshot,
		KeepStepRecords:     opts.keepStepRecords,
		ConfigPath:          opts.configPath,
	}
	var report *monkey.Report
	defer func() {
		if report == nil {
			return
		}
		fmt.Printf("运行完成: stop_reason=%s total=%d success=%d failed=%d duration_ms=%d\n",
			report.StopReason, report.StepsTotal, report.StepsSucceeded, report.StepsFailed, report.DurationMs)
		fmt.Printf("页面统计: %+v\n", report.PageVisitCount)
		fmt.Printf("恢复冷却统计: cooldown_enter=%d cooldown_step_hits=%d\n",
			report.RecoveryCooldownEnterCount, report.RecoveryCooldownStepCount)
		if reportFile != "" {
			if wErr := reporting.WriteRunReportWithArtifacts(reportFile, opts.reportFormat, artifactDir, metadata, report); wErr != nil {
				fmt.Fprintf(os.Stderr, "写入运行报告失败: %v\n", wErr)
			} else {
				fmt.Printf("报告已输出: %s\n", reportFile)
				if artifactDir != "" {
					fmt.Printf("页面产物已输出: %s\n", artifactDir)
				}
			}
		} else if artifactDir != "" {
			if _, wErr := reporting.BuildRunReportEnvelope(metadata, report, artifactDir); wErr != nil {
				fmt.Fprintf(os.Stderr, "写入页面产物失败: %v\n", wErr)
			} else {
				fmt.Printf("页面产物已输出: %s\n", artifactDir)
			}
		}
	}()

	report, err = runner.Run(ctx)
	if err != nil {
		return fmt.Errorf("执行 monkey 失败: %w", err)
	}
	return nil
}

func resolveArtifactDir(reportFile string, artifactDir string) string {
	explicitDir := strings.TrimSpace(artifactDir)
	if explicitDir != "" {
		return explicitDir
	}
	reportPath := strings.TrimSpace(reportFile)
	if reportPath == "" {
		return ""
	}
	ext := filepath.Ext(reportPath)
	base := strings.TrimSuffix(reportPath, ext)
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return base + "_artifacts"
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
	if strings.EqualFold(strings.TrimSpace(cfg.PageSourceType), "screenshot") {
		screenshot, err := driver.Screenshot(context.Background())
		if err != nil {
			return fmt.Errorf("获取截图失败: %w", err)
		}
		pageName := monkey.ResolveImageFingerprintPageName(screenshot, cfg.ImageFingerprintRegions)
		fmt.Printf("当前页面名: %s\n", pageName)
		return nil
	}
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
