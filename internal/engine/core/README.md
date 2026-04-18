# Core 子域说明

`core` 仅保留两类基础能力：

- `model`：状态图与决策模型容器（图结构、Agent 注册、配置接入）。
- `types`：引擎统一领域类型与底层工具函数（Action/State/Widget/Element/Operate）。

约束：
- 新增核心基础类型，应优先放入 `types`。
- 与状态图/模型生命周期强关联的逻辑，放入 `model`。
- 其他业务编排逻辑不再进入 `core`，应放在 `runtime/decision/perception/config`。
