package types

// CustomActionOperable 是 config.CustomAction 的抽象，
// 使 decision 层无需直接导入 config 包即可处理自定义动作。
type CustomActionOperable interface {
	IAction
	ToActionCommand() *ActionCommand
}

// ConfigProvider 抽象 config.Manager 的决策相关方法，
// 使 decision 层通过接口依赖而非具体类型。
type ConfigProvider interface {
	PatchOperate(operate *ActionCommand)
	ResolvePageAndGetSpecifiedAction(page string, elem IElement) IAction
	SkipAllActionsFromModel() bool
}

// UCTBanditStaticConfig 保存 UCT-Bandit 算法的静态配置覆盖项。
type UCTBanditStaticConfig struct {
	TwoStateLoopPenalty    Optional[float64]
	EdgeRepeatPenalty      Optional[float64]
	EdgeRepeatThreshold    Optional[int]
	ActionCooldownPenalty  Optional[float64]
	RecentActionWindow     Optional[int]
	LoopEscapeExploreBoost Optional[float64]
}

// ReuseStaticConfig 保存 Reuse 算法的静态配置覆盖项。
type ReuseStaticConfig struct {
	Epsilon                Optional[float64]
	Gamma                  Optional[float64]
	NStep                  Optional[int]
	ModelSavePath          string
	EnableModelPersistence Optional[bool]
	ResetModelOnStart      Optional[bool]
}

// StaticConfigProvider 抽象静态配置访问，用于 uctbandit agent 获取配置覆盖。
type StaticConfigProvider interface {
	GetUCTBanditConfig() UCTBanditStaticConfig
	GetReuseConfig() ReuseStaticConfig
}
