# Types 子域

`types` 负责定义引擎共享类型与工具：
- 核心对象：`Action`、`State`、`Widget`、`Element`、`DeviceOperateWrapper`
- 选择策略接口与列表辅助结构
- 基础哈希/随机/文本工具函数（同包内）

设计约束：
- 只放通用基础能力，避免引入业务编排逻辑
- 优先保持无副作用、可复用
