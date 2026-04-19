package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
	"trek/pkg/driver/android"
	"trek/pkg/monkey"
	"trek/pkg/session"
)

// cliOptions 定义 monkey 主入口参数。
type cliOptions struct {
	packageName       string
	deviceSerial      string
	configPath        string
	pageSourceType    string
	maxSteps          int
	maxDuration       time.Duration
	stepInterval      time.Duration
	captureScreenshot bool
	keepStepRecords   bool
	probePageName     bool
	autoCurrentApp    bool
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
	flag.StringVar(&opts.pageSourceType, "page-source", "uia", "页面源类型，如 uia或者poco")
	flag.IntVar(&opts.maxSteps, "max-steps", 300, "最大执行步数")
	flag.DurationVar(&opts.maxDuration, "max-duration", 10*time.Minute, "最大运行时长")
	flag.DurationVar(&opts.stepInterval, "step-interval", 300*time.Millisecond, "基础步进间隔")
	flag.BoolVar(&opts.captureScreenshot, "capture-screenshot", false, "是否采集截图给决策层")
	flag.BoolVar(&opts.keepStepRecords, "keep-step-records", true, "是否保留每步记录")
	flag.BoolVar(&opts.probePageName, "probe-page-name", false, "仅探测当前页面名后退出")
	flag.BoolVar(&opts.autoCurrentApp, "auto-current-app", false, "自动使用当前前台应用进行测试")
	flag.Parse()
	return opts
}

func run(opts cliOptions) error {
	driver, err := android.NewAndroidDriverWith(opts.deviceSerial)
	if err != nil {
		return fmt.Errorf("创建设备驱动失败: %w", err)
	}
	defer func() { _ = driver.Close() }()

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
		PageSourceType:    opts.pageSourceType,
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
