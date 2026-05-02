/*
Copyright © 2026 Trek

Trek — Android UI 自动化测试引擎（Smart Monkey）
*/
package cmd

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// rootCmd 是 trek CLI 的根命令，不带子命令时显示帮助。
var rootCmd = &cobra.Command{
	Use:   "trek",
	Short: "Trek — Android UI 自动化测试引擎",
	Long: `Trek 是一个 Android UI 自动化测试引擎（Smart Monkey），
通过感知-决策-执行闭环在真机上自主遍历应用。

支持两种模式：
  trek run  — 执行 monkey 测试
  trek web  — 启动 Web 配置界面`,
}

// logLevel 全局日志级别，由 --log-level 标志设置。
var logLevel string

// Execute 将所有子命令添加到根命令并设置标志，由 main.main() 调用。
func Execute() {
	loadDotEnvFiles()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// loadDotEnvFiles 加载 Trek 允许的 .env 文件。
// 优先级从低到高（后者覆盖前者，但 godotenv.Load 不覆盖已有值，所以先加载高优先级）：
//
//	.env.local              — 本地覆盖，不提交（gitignore）
//	.env.development.local  — 开发环境本地覆盖，不提交（gitignore）
//	.env                    — 全局默认值，提交到仓库
//
// 外部显式注入的环境变量（如 CI/CD、容器）始终优先于任何 .env 文件。
func loadDotEnvFiles() {
	// 按优先级从高到低加载，godotenv.Load 仅补充未设置的变量。
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load(".env.development.local")
	_ = godotenv.Load(".env")
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "控制台日志级别: debug, info, warn, error")
}
