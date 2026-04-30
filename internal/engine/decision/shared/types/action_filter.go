package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type ActionFilterALL = coretypes.ActionFilterALL
type ActionFilterTarget = coretypes.ActionFilterTarget
type ActionFilterValid = coretypes.ActionFilterValid
type ActionFilterEnableValid = coretypes.ActionFilterEnableValid
type ActionFilterUnvisitedValid = coretypes.ActionFilterUnvisitedValid
type ActionFilterValidUnSaturated = coretypes.ActionFilterValidUnSaturated

var (
	AllFilter                    = coretypes.AllFilter
	TargetFilter                 = coretypes.TargetFilter
	ValidFilter                  = coretypes.ValidFilter
	EnableValidFilter            = coretypes.EnableValidFilter
	EnableValidUnvisitedFilter   = coretypes.EnableValidUnvisitedFilter
	EnableValidUnSaturatedFilter = coretypes.EnableValidUnSaturatedFilter
)

func NewActionFilterALL() *ActionFilterALL                       { return coretypes.NewActionFilterALL() }
func NewActionFilterTarget() *ActionFilterTarget                 { return coretypes.NewActionFilterTarget() }
func NewActionFilterValid() *ActionFilterValid                   { return coretypes.NewActionFilterValid() }
func NewActionFilterEnableValid() *ActionFilterEnableValid       { return coretypes.NewActionFilterEnableValid() }
func NewActionFilterUnvisitedValid() *ActionFilterUnvisitedValid { return coretypes.NewActionFilterUnvisitedValid() }
func NewActionFilterValidUnSaturated() *ActionFilterValidUnSaturated {
	return coretypes.NewActionFilterValidUnSaturated()
}
func FilterActions(actions StatefulActionList, filter IStatefulActionFilter) StatefulActionList {
	return coretypes.FilterActions(actions, filter)
}
