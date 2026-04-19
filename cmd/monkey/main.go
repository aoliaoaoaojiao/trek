package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"trek/internal/scripting"
	"trek/logger"
	"trek/pkg/driver/android"
	"trek/pkg/driver/common/page/poco"
	"trek/pkg/monkey"
	"trek/pkg/session"
)

// cliOptions 定义 monkey 主入口参数。
type cliOptions struct {
	packageName       string
	deviceSerial      string
	configPath        string
	maxSteps          int
	maxDuration       time.Duration
	stepInterval      time.Duration
	captureScreenshot bool
	keepStepRecords   bool
	probePageName     bool
	autoCurrentApp    bool
	logLevel          string
}

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() cliOptions {
	var opts cliOptions
	flag.StringVar(&opts.packageName, "package", "", "被测应用包名（必填）")
	flag.StringVar(&opts.deviceSerial, "serial", "", "设备序列号（可选，默认自动选择）")
	flag.StringVar(&opts.configPath, "config", "", "配置文件路径（可选，仅支持 .js，支持绝对/相对路径）")
	flag.IntVar(&opts.maxSteps, "max-steps", 300, "最大执行步数")
	flag.DurationVar(&opts.maxDuration, "max-duration", 10*time.Minute, "最大运行时长")
	flag.DurationVar(&opts.stepInterval, "step-interval", 300*time.Millisecond, "基础步进间隔")
	flag.BoolVar(&opts.captureScreenshot, "capture-screenshot", false, "是否采集截图给决策层")
	flag.BoolVar(&opts.keepStepRecords, "keep-step-records", true, "是否保留每步记录")
	flag.BoolVar(&opts.probePageName, "probe-page-name", false, "仅探测当前页面名后退出")
	flag.BoolVar(&opts.autoCurrentApp, "auto-current-app", false, "自动使用当前前台应用进行测试")
	flag.StringVar(&opts.logLevel, "log-level", "info", "控制台日志级别: debug, info, warn, error")
	flag.Parse()
	return opts
}

func run(opts cliOptions) error {
	if err := logger.SetLevel(opts.logLevel); err != nil {
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
	driver, err := android.NewAndroidDriverWith(opts.deviceSerial, driverOptions...)
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
		return fmt.Errorf("参数 -package 不能为空，或使用 -auto-current-app 自动获取")
	}

	sess := session.NewSession(session.Config{
		PackageName: packageName,
	})
	if opts.configPath != "" {
		if err := sess.LoadConfigFile(opts.configPath); err != nil {
			return fmt.Errorf("加载配置文件失败(%s): %w", opts.configPath, err)
		}
		fmt.Printf("配置加载成功: %s\n", opts.configPath)
	}

	cfg := monkey.Config{
		PackageName:       packageName,
		MaxSteps:          opts.maxSteps,
		MaxDuration:       opts.maxDuration,
		StepInterval:      opts.stepInterval,
		PageSourceType:    pageSourceType,
		CaptureScreenshot: opts.captureScreenshot,
		KeepStepRecords:   opts.keepStepRecords,
		StopOnCrash:       true,
		StopOnANR:         true,
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
