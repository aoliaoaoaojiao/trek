package types

import (
	"math"
)

// ActionFilterALL 全部动作过滤器
type ActionFilterALL struct{}

// NewActionFilterALL 创建新的全部动作过滤器
func NewActionFilterALL() *ActionFilterALL {
	return &ActionFilterALL{}
}

// Include 包含所有动作
func (f *ActionFilterALL) Include(action IAction) bool {
	return true
}

// GetPriority 获取优先级
func (f *ActionFilterALL) GetPriority(action IAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// ActionFilterTarget 目标动作过滤器
type ActionFilterTarget struct{}

// NewActionFilterTarget 创建新的目标动作过滤器
func NewActionFilterTarget() *ActionFilterTarget {
	return &ActionFilterTarget{}
}

// Include 只包含需要目标的动作
func (f *ActionFilterTarget) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	return action.RequireTarget()
}

// GetPriority 获取优先级
func (f *ActionFilterTarget) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// ActionFilterValid 有效动作过滤器
type ActionFilterValid struct{}

// NewActionFilterValid 创建新的有效动作过滤器
func NewActionFilterValid() *ActionFilterValid {
	return &ActionFilterValid{}
}

// Include 只包含有效动作
func (f *ActionFilterValid) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	return action.IsValid()
}

// GetPriority 获取优先级
func (f *ActionFilterValid) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// ActionFilterEnableValid 启用且有效动作过滤器
type ActionFilterEnableValid struct{}

// NewActionFilterEnableValid 创建新的启用且有效动作过滤器
func NewActionFilterEnableValid() *ActionFilterEnableValid {
	return &ActionFilterEnableValid{}
}

// Include 只包含启用且有效的动作
func (f *ActionFilterEnableValid) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	return action.GetEnabled() && action.IsValid()
}

// GetPriority 获取优先级
func (f *ActionFilterEnableValid) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// ActionFilterUnvisitedValid 未访问且有效动作过滤器
type ActionFilterUnvisitedValid struct{}

// NewActionFilterUnvisitedValid 创建新的未访问且有效动作过滤器
func NewActionFilterUnvisitedValid() *ActionFilterUnvisitedValid {
	return &ActionFilterUnvisitedValid{}
}

// Include 只包含未访问且有效的动作
func (f *ActionFilterUnvisitedValid) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	return action.GetEnabled() && action.IsValid() && !action.IsVisited()
}

// GetPriority 获取优先级
func (f *ActionFilterUnvisitedValid) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// ActionFilterValidUnSaturated 有效且未饱和动作过滤器
type ActionFilterValidUnSaturated struct{}

// NewActionFilterValidUnSaturated 创建新的有效且未饱和动作过滤器
func NewActionFilterValidUnSaturated() *ActionFilterValidUnSaturated {
	return &ActionFilterValidUnSaturated{}
}

// Include 只包含有效且未饱和的动作
func (f *ActionFilterValidUnSaturated) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}

	// 获取状态来检查饱和状态
	state := action.GetState()
	if state == nil {
		return false
	}

	return action.GetEnabled() && action.IsValid() && !state.IsSaturated(action)
}

// GetPriority 获取优先级
func (f *ActionFilterValidUnSaturated) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// ActionFilterValidValuePriority 有效且基于Q值优先级的动作过滤器
type ActionFilterValidValuePriority struct{}

// NewActionFilterValidValuePriority 创建新的有效且基于Q值优先级的动作过滤器
func NewActionFilterValidValuePriority() *ActionFilterValidValuePriority {
	return &ActionFilterValidValuePriority{}
}

// Include 只包含有效的动作
func (f *ActionFilterValidValuePriority) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}
	return action.GetEnabled() && action.IsValid()
}

// GetPriority 获取基于Q值的优先级
func (f *ActionFilterValidValuePriority) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	pri := action.GetPriority()
	if !action.IsBack() {
		pri += int32(math.Ceil(10 * action.GetQValue()))
	}
	return pri
}

// ActionFilterValidDatePriority 有效且基于日期优先级的动作过滤器
type ActionFilterValidDatePriority struct{}

// NewActionFilterValidDatePriority 创建新的有效且基于日期优先级的动作过滤器
func NewActionFilterValidDatePriority() *ActionFilterValidDatePriority {
	return &ActionFilterValidDatePriority{}
}

// Include 根据动作类型包含动作
func (f *ActionFilterValidDatePriority) Include(action *StatefulAction) bool {
	if action == nil {
		return false
	}

	switch action.GetActionType() {
	case START, RESTART, CLEAN_RESTART, NOP, ACTIVATE, BACK:
		return true
	case CLICK, LONG_CLICK, SCROLL_BOTTOM_UP, SCROLL_TOP_DOWN, SCROLL_LEFT_RIGHT, SCROLL_RIGHT_LEFT, SCROLL_BOTTOM_UP_N:
		return action.GetEnabled() && action.IsValid() && !action.IsEmpty()
	default:
		return false
	}
}

// GetPriority 获取优先级
func (f *ActionFilterValidDatePriority) GetPriority(action *StatefulAction) int32 {
	if action == nil {
		return 0
	}
	return action.GetPriority()
}

// 全局过滤器实例
var (
	AllFilter                      = NewActionFilterALL()
	TargetFilter                   = NewActionFilterTarget()
	ValidFilter                    = NewActionFilterValid()
	EnableValidFilter              = NewActionFilterEnableValid()
	EnableValidUnvisitedFilter     = NewActionFilterUnvisitedValid()
	EnableValidUnSaturatedFilter   = NewActionFilterValidUnSaturated()
	EnableValidValuePriorityFilter = NewActionFilterValidValuePriority()
	ValidDatePriorityFilter        = NewActionFilterValidDatePriority()
)

// FilterActions 使用过滤器过滤动作
func FilterActions(actions StatefulActionList, filter IStatefulActionFilter) StatefulActionList {
	if filter == nil {
		return actions
	}

	result := make(StatefulActionList, 0)
	for _, action := range actions {
		if filter.Include(action) {
			result = append(result, action)
		}
	}
	return result
}
