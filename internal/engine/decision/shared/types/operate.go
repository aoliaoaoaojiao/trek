package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type ActionCommand = coretypes.ActionCommand
type OperateList = coretypes.OperateList

var ActionCommandNop = coretypes.ActionCommandNop

func NewActionCommand() *ActionCommand                         { return coretypes.NewActionCommand() }
func NewActionCommandFromJSON(optJSONStr string) *ActionCommand { return coretypes.NewActionCommandFromJSON(optJSONStr) }
func NewActionCommandCopy(opt *ActionCommand) *ActionCommand    { return coretypes.NewActionCommandCopy(opt) }
