// Package reuse 实现重用相关的功能，包括RichWidget、ActivityNameAction和ReuseState
package reuse

import (
	"trek/internal/core/types"
	"trek/internal/tool"
)

// 导出desc包中的哈希计算工具函数
var (
	HashString         = tool.HashString
	HashInt            = tool.HashInt
	CombineHashWidgets = types.CombineHashWidgets
)
