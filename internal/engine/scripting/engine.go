package scripting

import "trek/pkg/engine/config"

// RuntimeContext 表示脚本执行时可见的最小运行态信息。
type RuntimeContext struct {
	PageName string
}

// Engine 定义脚本引擎最小能力边界（可由 goja 实现）。
type Engine interface {
	Apply(cfg config.ScriptConfig) error
	Execute(ctx RuntimeContext) error
}
