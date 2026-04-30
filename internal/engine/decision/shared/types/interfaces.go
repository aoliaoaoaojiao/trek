package types

import (
	coretypes "trek/internal/engine/core/types"
)

// 以下为向后兼容的重新导出。

type EnableNode = coretypes.EnableNode
type IGraphListener = coretypes.IGraphListener
type IAgent = coretypes.IAgent
type StateBlockAwareAgent = coretypes.StateBlockAwareAgent
type HashNode = coretypes.HashNode
type Serializable = coretypes.Serializable
type IAction = coretypes.IAction
type IStatefulActionFilter = coretypes.IStatefulActionFilter
type IState = coretypes.IState
type IElement = coretypes.IElement
type IWidget = coretypes.IWidget

// 新增接口的重新导出
type CustomActionOperable = coretypes.CustomActionOperable
type ConfigProvider = coretypes.ConfigProvider
type StaticConfigProvider = coretypes.StaticConfigProvider
type UCTBanditStaticConfig = coretypes.UCTBanditStaticConfig
