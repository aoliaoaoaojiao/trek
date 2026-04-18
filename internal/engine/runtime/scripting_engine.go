package runtime

// ScriptRuntimeContext 表示脚本执行时可见的最小运行态信息。
type ScriptRuntimeContext struct {
	PageName string
}

// ScriptConfig 描述脚本引擎运行配置。
type ScriptConfig struct {
	Enabled bool
	Timeout int
	Entry   string
	APIs    []string
}

// ScriptEngine 定义脚本引擎最小能力边界（可由 goja 等实现）。
type ScriptEngine interface {
	Apply(cfg ScriptConfig) error
	Execute(ctx ScriptRuntimeContext) error
}
