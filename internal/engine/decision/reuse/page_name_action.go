package reuse

import (
	"trek/internal/engine/decision/shared/tool"
	"trek/internal/engine/decision/shared/types"
	"trek/logger"
)

// PageNameAction 甯﹂〉闈㈠悕绉扮殑鍔ㄤ綔
type PageNameAction struct {
	types.StatefulAction
	pageName string
}

// pageName

// NewPageNameAction 鍒涘缓鏂扮殑ActivityNameAction
func NewPageNameAction(pageName string, widget types.IWidget, actionType types.ActionType) *PageNameAction {
	// 鍒涘缓鍩虹鐨凷tatefulAction
	baseAction := types.NewStatefulAction(nil, widget, actionType)

	pageAction := &PageNameAction{
		StatefulAction: *baseAction,
		pageName:       pageName,
	}

	// 璁＄畻鍝堝笇鐮?
	pageNameHashCode := tool.HashString(pageName)
	actionTypeHashCode := tool.HashInt(int(pageAction.GetActionType()))

	var targetHash uintptr
	if widget != nil {
		targetHash = widget.Hash()
	} else {
		targetHash = 0x1
	}

	pageAction.Hashcode = 0x9e3779b9 + (pageNameHashCode << 2) ^
		(((actionTypeHashCode << 6) ^ (targetHash << 1)) << 1)

	logger.Debugf("pageName name action created pageName:%s hashcode:%d activityHash:%d targetHash:%d",
		pageName, pageAction.Hashcode, pageNameHashCode, targetHash)

	return pageAction
}
