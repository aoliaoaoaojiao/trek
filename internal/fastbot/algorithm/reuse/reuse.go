// Package reuse 实现重用相关的功能，包括RichWidget、PageNameNameAction和ReuseState
package reuse

import (
	"Trek/internal/fastbot/core/types"
	"Trek/internal/fastbot/tool"
)

// 导出desc包中的哈希计算工具函数
var (
	HashString         = tool.HashString
	HashInt            = tool.HashInt
	CombineHashWidgets = types.CombineHashWidgets
)
