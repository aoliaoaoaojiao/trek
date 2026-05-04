/*
Copyright © 2026 Trek

Trek — Android UI 自动化测试引擎（Smart Monkey）
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"trek/internal/util"
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
	util.LoadDotEnvFiles()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "控制台日志级别: debug, info, warn, error")
}
