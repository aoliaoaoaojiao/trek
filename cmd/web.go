/*
Copyright © 2026 Trek

*/
package cmd

import (
	"embed"
	"io/fs"

	"trek/internal/webserver"
	"trek/logger"

	"github.com/spf13/cobra"
)

// webOptions 存储 web 子命令的标志值。
var webOptions = struct {
	addr string
}{}

//go:embed web/ui/dist/*
var webUIDistFS embed.FS

// webCmd 定义 web 子命令。
var webCmd = &cobra.Command{
	Use:   "web",
	Short: "启动 web 配置界面",
	Long:  `启动 Trek Web 配置界面，提供可视化配置和预览功能。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := logger.SetLevel(logLevel); err != nil {
			return err
		}
		distFS, err := fs.Sub(webUIDistFS, "web/ui/dist")
		if err != nil {
			return err
		}
		return webserver.Serve(webOptions.addr, distFS)
	},
}

func init() {
	rootCmd.AddCommand(webCmd)

	webCmd.Flags().StringVar(&webOptions.addr, "addr", ":17888", "web 模式监听地址")
}