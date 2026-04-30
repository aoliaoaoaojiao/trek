package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type Node = coretypes.Node
type PriorityNodeImpl = coretypes.PriorityNodeImpl

func NewNode() *Node                       { return coretypes.NewNode() }
func NewPriorityNode() *PriorityNodeImpl   { return coretypes.NewPriorityNode() }
