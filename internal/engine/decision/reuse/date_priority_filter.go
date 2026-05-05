package reuse

import "trek/internal/engine/core/types"

// ActionFilterValidDatePriority 是 reuse 策略专用的动作过滤器。
type ActionFilterValidDatePriority struct{}

func NewActionFilterValidDatePriority() *ActionFilterValidDatePriority {
	return &ActionFilterValidDatePriority{}
}

func (f *ActionFilterValidDatePriority) Include(action *types.StatefulAction) bool {
	if action == nil {
		return false
	}

	switch action.GetActionType() {
	case types.START, types.RESTART, types.CLEAN_RESTART, types.NOP, types.ACTIVATE, types.BACK:
		return true
	case types.CLICK, types.LONG_CLICK, types.INPUT, types.SCROLL_BOTTOM_UP, types.SCROLL_TOP_DOWN, types.SCROLL_LEFT_RIGHT, types.SCROLL_RIGHT_LEFT, types.SCROLL_BOTTOM_UP_N:
		return action.GetEnabled() && action.IsValid() && !action.IsEmpty()
	default:
		return false
	}
}

func (f *ActionFilterValidDatePriority) GetPriority(action *types.StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}
