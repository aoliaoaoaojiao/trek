# Model 子域

`model` 负责状态图与 Agent 决策容器：
- 维护状态图（`Graph`）
- 维护设备 Agent 映射
- 接入运行时配置管理器
- 提供 `GetOperateOpt` 作为模型级决策入口

设计约束：
- 不承载感知解析与驱动执行细节
- 保持对上层编排层的稳定 API
