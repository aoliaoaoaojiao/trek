// Package reuse 提供 reuse 策略子域对外复用的公共符号与兼容导出。
package reuse

import (
	"trek/internal/engine/core/tool"
	"trek/internal/engine/core/types"
)

// 兼容导出：复用 core 中常用哈希与控件组合工具，降低迁移改动范围。
var (
	HashString         = tool.HashString
	HashInt            = tool.HashInt
	CombineHashWidgets = types.CombineHashWidgets
)
