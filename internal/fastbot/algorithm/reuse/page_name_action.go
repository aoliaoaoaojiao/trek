package reuse

import (
	types2 "Trek/internal/fastbot/core/types"
	"Trek/log"
)

// PageNameAction 带页面名称的动作
type PageNameAction struct {
	types2.StatefulAction
	pageName string
}

// pageName

// NewPageNameAction 创建新的ActivityNameAction
func NewPageNameAction(pageName string, widget *types2.Widget, actionType types2.ActionType) *PageNameAction {
	// 创建基础的StatefulAction
	baseAction := types2.NewStatefulAction(nil, widget, actionType)

	pageAction := &PageNameAction{
		StatefulAction: *baseAction,
		pageName:       pageName,
	}

	// 计算哈希码
	pageNameHashCode := HashString(pageName)
	actionTypeHashCode := HashInt(int(pageAction.GetActionType()))

	var targetHash uintptr
	if widget != nil {
		targetHash = widget.Hash()
	} else {
		targetHash = 0x1
	}

	pageAction.Hashcode = 0x9e3779b9 + (pageNameHashCode << 2) ^
		(((actionTypeHashCode << 6) ^ (targetHash << 1)) << 1)

	log.Debugf("pageName name action created pageName:%s hashcode:%d activityHash:%d targetHash:%d",
		pageName, pageAction.Hashcode, pageNameHashCode, targetHash)

	return pageAction
}
