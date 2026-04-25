package reuse

import (
	"trek/internal/engine/decision/shared/tool"
	"trek/internal/engine/decision/shared/types"
	"trek/logger"
)

// RandomPickUnvisitedAction 为 reuse 提供“未访问优先 + back 兜底”的动作选择。
func RandomPickUnvisitedAction(state types.IState) types.IAction {
	if state == nil {
		return nil
	}
	action := RandomPickAction(state, types.EnableValidUnvisitedFilter, false)
	if action == nil {
		back := findBackAction(state)
		if types.EnableValidUnvisitedFilter.Include(back) {
			return back
		}
	}
	return action
}

// GreedyPickAction 为 reuse 提供按过滤器优先级挑选最大值动作。
func GreedyPickAction(state types.IState, filter types.IStatefulActionFilter) types.IAction {
	if state == nil {
		return nil
	}
	if filter == nil {
		filter = types.EnableValidFilter
	}

	filtered := types.FilterActions(state.GetActions(), filter)
	if len(filtered) == 0 {
		return nil
	}

	maxAction := filtered[0]
	maxPriority := filter.GetPriority(maxAction)
	for _, action := range filtered[1:] {
		priority := filter.GetPriority(action)
		if priority > maxPriority {
			maxAction = action
			maxPriority = priority
		}
	}
	return maxAction
}

// RandomPickAction 为 reuse 提供按优先级权重随机选择动作。
func RandomPickAction(state types.IState, filter types.IStatefulActionFilter, includeBack bool) types.IAction {
	if state == nil {
		return nil
	}
	total := countActionPriority(state, filter, includeBack)
	if total == 0 {
		return nil
	}
	index := tool.RandomInt(0, total)
	return pickAction(state, filter, includeBack, index)
}

func countActionPriority(state types.IState, filter types.IStatefulActionFilter, includeBack bool) int {
	if filter == nil {
		filter = types.EnableValidFilter
	}
	total := 0
	for _, action := range state.GetActions() {
		if !includeBack && action.IsBack() {
			continue
		}
		if !filter.Include(action) {
			continue
		}
		priority := filter.GetPriority(action)
		if priority <= 0 {
			logger.Debugf("Error: Action should has a positive priority, but we get %d", priority)
			continue
		}
		total += int(priority)
	}
	return total
}

func pickAction(state types.IState, filter types.IStatefulActionFilter, includeBack bool, index int) types.IAction {
	if filter == nil {
		filter = types.EnableValidFilter
	}
	left := index
	for _, action := range state.GetActions() {
		if !includeBack && action.IsBack() {
			continue
		}
		if !filter.Include(action) {
			continue
		}
		priority := int(filter.GetPriority(action))
		if priority > left {
			return action
		}
		left -= priority
	}
	logger.Debugf("ERROR: action filter is unstable")
	return nil
}

func findBackAction(state types.IState) *types.StatefulAction {
	if state == nil {
		return nil
	}
	for _, action := range state.GetActions() {
		if action != nil && action.IsBack() {
			return action
		}
	}
	return nil
}
