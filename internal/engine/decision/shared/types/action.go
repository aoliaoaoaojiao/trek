package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type Action = coretypes.Action
type StatefulAction = coretypes.StatefulAction
type NetActionParam = coretypes.NetActionParam
type StatefulActionList = coretypes.StatefulActionList
type StatefulActionSet = coretypes.StatefulActionSet

var (
	NOPAction      = coretypes.NOPAction
	ACTIVATEAction = coretypes.ACTIVATEAction
	RESTARTAction  = coretypes.RESTARTAction
)

func NewAction(actionType ActionType) *Action { return coretypes.NewAction(actionType) }
func NewStatefulAction(state *State, targetWidget IWidget, actionType ActionType) *StatefulAction {
	return coretypes.NewStatefulAction(state, targetWidget, actionType)
}
