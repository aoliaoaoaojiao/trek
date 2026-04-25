// Package reuse exports compatibility symbols for reuse strategy.
package reuse

import (
	"trek/internal/engine/decision/shared/tool"
	"trek/internal/engine/decision/shared/types"
)

var (
	HashString         = tool.HashString
	HashInt            = tool.HashInt
	CombineHashWidgets = types.CombineHashWidgets
)
