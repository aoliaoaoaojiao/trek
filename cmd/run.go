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
	"trek/pkg/driver/android"
	"trek/pkg/driver/common/page/poco"
	"trek/pkg/monkey"
	"trek/pkg/session"

	"github.com/spf13/cobra"
)

// runOptions 存储 run 子命令的标志值。
var runOptions = struct {
	packageName            string
	deviceSerial           string
	configPath             string
	recoveryMemoryFile     string
	recoveryCooldownSteps  int
	recoveryLLMEndpoint    string
	recoveryLLMAPIKey      string
	recoveryLLMModel       string
	recoveryOpenAIModel    string
	recoveryOpenAIAPIKey   string
	recoveryOpenAIBaseURL  string
	recoveryLLMTimeout     time.Duration
	recoveryLLMMaxCalls    int
	recoveryLLMWindowSteps int
	algorithm              string
	maxSteps               int
	maxDuration            time.Duration
	stepInterval           time.Duration
	captureScreenshot      bool
	keepStepRecords        bool
	probePageName          bool
	autoCurrentApp         bool
}{}

// runCmd 定义 run 子命令。
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "执行 monkey 测试",
	Long:  `在连接的 Android 设备上执行 Smart Monkey UI 自动化遍历测试。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMonkey(logLevel, runOptions)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&runOptions.packageName, "package", "", "被测应用包名（必填，或使用 --auto-current-app 自动获取）")
	runCmd.Flags().StringVar(&runOptions.deviceSerial, "serial", "", "设备序列号（可选，默认自动选择）")
	runCmd.Flags().StringVar(&runOptions.configPath, "config", "", "配置文件路径（可选，仅支持 .js，支持绝对/相对路径）")
	runCmd.Flags().StringVar(&runOptions.recoveryMemoryFile, "recovery-memory-file", "", "恢复经验库 jsonl 文件路径（可选）")
	runCmd.Flags().IntVar(&runOptions.recoveryCooldownSteps, "recovery-cooldown-steps", 2, "恢复成功后的冷却步数，冷却期间抑制再次进入 recover")
	runCmd.Flags().StringVar(&runOptions.recoveryLLMEndpoint, "recovery-llm-endpoint", "", "恢复模式 LLM HTTP 接口地址（可选）")
	runCmd.Flags().StringVar(&runOptions.recoveryLLMAPIKey, "recovery-llm-api-key", "", "恢复模式 LLM 接口鉴权 key（可选，未传则读取 TREK_RECOVERY_LLM_API_KEY）")
	runCmd.Flags().StringVar(&runOptions.recoveryLLMModel, "recovery-llm-model", "", "恢复模式 LLM 模型名（可选，透传给接口）")
	runCmd.Flags().StringVar(&runOptions.recoveryOpenAIModel, "recovery-openai-model", "", "恢复模式 OpenAI 模型名（可选，配置后走 OpenAI Responses API）")
	runCmd.Flags().StringVar(&runOptions.recoveryOpenAIAPIKey, "recovery-openai-api-key", "", "OpenAI API Key（可选，未传则读取 OPENAI_API_KEY）")
	runCmd.Flags().StringVar(&runOptions.recoveryOpenAIBaseURL, "recovery-openai-base-url", "", "OpenAI Responses API 地址（可选，默认 https://api.openai.com/v1/responses）")
	runCmd.Flags().DurationVar(&runOptions.recoveryLLMTimeout, "recovery-llm-timeout", 15*time.Second, "恢复模式 LLM 接口超时时间")
	runCmd.Flags().IntVar(&runOptions.recoveryLLMMaxCalls, "recovery-llm-max-calls", 0, "恢复模式下 LLM 候选最大调用次数（0 表示不限制）")
	runCmd.Flags().IntVar(&runOptions.recoveryLLMWindowSteps, "recovery-llm-window-steps", 0, "恢复模式下 LLM 调用统计窗口步数（0 表示全局统计）")
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
	packageName            string
	deviceSerial           string
	configPath             string
	recoveryMemoryFile     string
	recoveryCooldownSteps  int
	recoveryLLMEndpoint    string
	recoveryLLMAPIKey      string
	recoveryLLMModel       string
	recoveryOpenAIModel    string
	recoveryOpenAIAPIKey   string
	recoveryOpenAIBaseURL  string
	recoveryLLMTimeout     time.Duration
	recoveryLLMMaxCalls    int
	recoveryLLMWindowSteps int
	algorithm              string
	maxSteps               int
	maxDuration            time.Duration
	stepInterval           time.Duration
	captureScreenshot      bool
	keepStepRecords        bool
	probePageName          bool
	autoCurrentApp         bool
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

	pageSourceType, err := resolvePageSourceType(staticCfg)
	if err != nil {
		return err
	}
	touchMode, touchType, err := resolveTouchMode(staticCfg)
	if err != nil {
		return err
	}

	driverOptions, err := resolveDriverOptions(staticCfg, pageSourceType, touchType)
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
		pkg, err := driver.GetCurrentPackage()
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

	sess := session.NewSession(session.Config{
		PackageName:              packageName,
		Algorithm:                algorithmType,
		RecoveryMemoryFile:       strings.TrimSpace(opts.recoveryMemoryFile),
		RecoveryLLMEndpoint:      strings.TrimSpace(opts.recoveryLLMEndpoint),
		RecoveryLLMAPIKey:        strings.TrimSpace(opts.recoveryLLMAPIKey),
		RecoveryLLMModel:         strings.TrimSpace(opts.recoveryLLMModel),
		RecoveryLLMOpenAIModel:   strings.TrimSpace(opts.recoveryOpenAIModel),
		RecoveryLLMOpenAIAPIKey:  strings.TrimSpace(opts.recoveryOpenAIAPIKey),
		RecoveryLLMOpenAIBaseURL: strings.TrimSpace(opts.recoveryOpenAIBaseURL),
		RecoveryLLMTimeout:       opts.recoveryLLMTimeout,
	})
	if opts.configPath != "" {
		if err := sess.LoadConfigFile(opts.configPath); err != nil {
			return fmt.Errorf("加载配置文件失败(%s): %w", opts.configPath, err)
		}
		fmt.Printf("配置加载成功: %s\n", opts.configPath)
	}

	cfg := monkey.Config{
		PackageName:                 packageName,
		DeviceSerial:                deviceSerial,
		MaxSteps:                    opts.maxSteps,
		MaxDuration:                 opts.maxDuration,
		StepInterval:                opts.stepInterval,
		PageSourceType:              pageSourceType,
		PageNameStrategy:            strings.TrimSpace(staticCfg.PageNameStrategy),
		CaptureScreenshot:           opts.captureScreenshot,
		KeepStepRecords:             opts.keepStepRecords,
		StopOnCrash:                 true,
		StopOnANR:                   true,
		EffectiveTouchArea:          buildEffectiveTouchAreaConfig(staticCfg, packageName, deviceSerial),
		RecoveryCooldownSteps:       opts.recoveryCooldownSteps,
		RecoveryLLMBudgetMaxCalls:   opts.recoveryLLMMaxCalls,
		RecoveryLLMBudgetWindowStep: opts.recoveryLLMWindowSteps,
	}

	if opts.probePageName {
		return probePageName(driver, cfg)
	}

	runner, err := monkey.NewRunner(sess, driver, cfg)
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
	return nil
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
	case "server", "6":
		return decision.AlgorithmServer, nil
	default:
		return decision.AlgorithmReuse, nil
	}
}

// resolvePageSourceType 从静态配置解析页面源类型。
func resolvePageSourceType(staticCfg scripting.StaticConfig) (string, error) {
	pageSource := strings.TrimSpace(staticCfg.PageSource)
	if pageSource == "" {
		pageSource = "uia"
	}
	switch strings.ToLower(pageSource) {
	case "uia":
		return "uia", nil
	case "poco":
		return "poco", nil
	default:
		return "", fmt.Errorf("不支持的页面源类型: %s（可选: uia, poco）", pageSource)
	}
}

// resolveTouchMode 从静态配置解析触控模式。
func resolveTouchMode(staticCfg scripting.StaticConfig) (string, android.TouchType, error) {
	touchMode := strings.TrimSpace(staticCfg.TouchMode)
	if touchMode == "" {
		touchMode = "motion"
	}
	switch strings.ToLower(touchMode) {
	case "motion":
		return "motion", android.TouchTypeMotion, nil
	case "uia":
		return "uia", android.TouchTypeUIA, nil
	case "adb":
		return "adb", android.TouchTypeADB, nil
	default:
		return "", "", fmt.Errorf("不支持的触控模式: %s（可选: motion, uia, adb）", touchMode)
	}
}

// resolveDriverOptions 从静态配置构建 Android 驱动选项。
func resolveDriverOptions(staticCfg scripting.StaticConfig, pageSourceType string, touchType android.TouchType) ([]android.AndroidDriverOption, error) {
	options := []android.AndroidDriverOption{
		android.WithTouch(touchType),
	}

	uiaServerPort := staticCfg.UIA.ServerPort
	if uiaServerPort > 0 {
		options = append(options, android.WithUIAServerPort(uiaServerPort))
	}

	if strings.EqualFold(pageSourceType, "poco") {
		engineText := strings.TrimSpace(staticCfg.Poco.Engine)
		if engineText == "" {
			return nil, fmt.Errorf("使用 poco 页面源时必须指定 Poco 引擎（请配置 config.poco.engine）")
		}
		engine, err := parsePocoEngine(engineText)
		if err != nil {
			return nil, err
		}

		pocoPort := staticCfg.Poco.Port
		if pocoPort <= 0 {
			pocoPort = engine.GetDefaultPort()
		}
		if pocoPort <= 0 {
			return nil, fmt.Errorf("Poco 端口无效，请通过 config.poco.port 指定")
		}
		options = append(options, android.WithPoco(engine, pocoPort))
	}

	return options, nil
}

// parsePocoEngine 将字符串解析为 Poco 引擎类型。
func parsePocoEngine(text string) (poco.Engine, error) {
	raw := strings.TrimSpace(text)
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(raw, "-", "_"), " ", "_"))
	switch normalized {
	case string(poco.Unity3d), "UNITY", "UNITY3D":
		return poco.Unity3d, nil
	case string(poco.UE4):
		return poco.UE4, nil
	case string(poco.Cocos2dxJs), "COCOS2DX_JS", "COCOS_JS":
		return poco.Cocos2dxJs, nil
	case string(poco.CocosCreator), "COCOS_CREATOR3D":
		return poco.CocosCreator, nil
	case string(poco.Egret):
		return poco.Egret, nil
	case string(poco.Cocos2dxLua), "COCOS2DX_LUA":
		return poco.Cocos2dxLua, nil
	case string(poco.Cocos2dxCPlus), "COCOS2DX_C++", "COCOS2DX_CPLUS", "COCOS2DX_CPP":
		return poco.Cocos2dxCPlus, nil
	default:
		return "", fmt.Errorf("不支持的 Poco 引擎: %s", raw)
	}
}

// buildEffectiveTouchAreaConfig 从静态配置构建有效触控区域配置。
func buildEffectiveTouchAreaConfig(staticCfg scripting.StaticConfig, packageName string, deviceSerial string) *monkey.EffectiveTouchArea {
	if staticCfg.EffectiveTouchArea == nil {
		return nil
	}
	rangeCfg := staticCfg.EffectiveTouchArea.Range
	area := &monkey.EffectiveTouchArea{
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
	return area
}
