package reuse

import (
	"math"
	"trek/internal/engine/core/types"
)

// ActionFilterValidValuePriority 是 reuse 算法专用的 Q 值优先级过滤器。
type ActionFilterValidValuePriority struct {
	qValueGetter func(action *types.StatefulAction) float64
}

func NewActionFilterValidValuePriority(qValueGetter func(action *types.StatefulAction) float64) *ActionFilterValidValuePriority {
	return &ActionFilterValidValuePriority{qValueGetter: qValueGetter}
}

func (f *ActionFilterValidValuePriority) Include(action *types.StatefulAction) bool {
	if action == nil {
		return false
	}
	return action.GetEnabled() && action.IsValid()
}

func (f *ActionFilterValidValuePriority) GetPriority(action *types.StatefulAction) int32 {
	if action == nil {
		return 0
	}
	pri := action.GetPriority()
	if !action.IsBack() {
		qv := 0.0
		if f != nil && f.qValueGetter != nil {
			qv = f.qValueGetter(action)
		}
		pri += int32(math.Ceil(10 * qv))
	}
	return pri
}
