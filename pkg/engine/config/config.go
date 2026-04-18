package config

// RuntimeConfig 是对外暴露的稳定配置契约，供 TOML/脚本层映射后驱动引擎行为。
// 该结构应保持向后兼容：新增字段优先、谨慎删除/重命名字段。
type RuntimeConfig struct {
	// DumpRules 作用于 UI dump 的动态规则，如节点屏蔽、属性修补等。
	DumpRules []DumpRule `toml:"dump_rules"`
	// Script 声明运行时脚本相关设置（例如 goja）。
	Script ScriptConfig `toml:"script"`
}

// DumpRule 描述一条 dump 处理规则。
type DumpRule struct {
	Name     string            `toml:"name"`
	Enabled  bool              `toml:"enabled"`
	Selector Selector          `toml:"selector"`
	Action   RuleAction        `toml:"action"`
	Patch    map[string]string `toml:"patch"`
}

// Selector 用于匹配节点。
type Selector struct {
	PageName string `toml:"page_name"`
	XPath    string `toml:"xpath"`
	Text     string `toml:"text"`
	Class    string `toml:"class"`
}

// RuleAction 定义匹配后的处理动作。
type RuleAction string

const (
	RuleActionMask  RuleAction = "mask"
	RuleActionPatch RuleAction = "patch"
	RuleActionDrop  RuleAction = "drop"
)

// ScriptConfig 描述脚本引擎相关配置。
type ScriptConfig struct {
	Enabled bool     `toml:"enabled"`
	Timeout int      `toml:"timeout_ms"`
	Entry   string   `toml:"entry"`
	APIs    []string `toml:"apis"`
}
