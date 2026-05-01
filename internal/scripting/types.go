package scripting

// ActionType 是脚本层稳定动作名称。
type ActionType string

const (
	ActionNOP             ActionType = "NOP"
	ActionBack            ActionType = "BACK"
	ActionClick           ActionType = "CLICK"
	ActionLongClick       ActionType = "LONG_CLICK"
	ActionScrollTopDown   ActionType = "SCROLL_TOP_DOWN"
	ActionScrollBottomUp  ActionType = "SCROLL_BOTTOM_UP"
	ActionScrollLeftRight ActionType = "SCROLL_LEFT_RIGHT"
	ActionScrollRightLeft ActionType = "SCROLL_RIGHT_LEFT"
	ActionScrollBottomUpN ActionType = "SCROLL_BOTTOM_UP_N"
	ActionStart           ActionType = "START"
	ActionRestart         ActionType = "RESTART"
	ActionCleanRestart    ActionType = "CLEAN_RESTART"
	ActionActivate        ActionType = "ACTIVATE"
)

type Action struct {
	Type         ActionType
	Bounds       [4]float64
	Text         string
	Clear        bool
	ADBInput     bool
	AllowFuzzing bool
	Throttle     int
	WaitTime     int
}

type Screenshot struct {
	Bytes  []byte
	MIME   string
	Width  int
	Height int
}

type PageNode struct {
	Text        string
	ResourceID  string
	ContentDesc string
	ClassName   string
	Bounds      [4]float64
	Clickable   bool
	Enabled     bool
	Editable    bool
	XPath       string
}

type PageSnapshot struct {
	Name       string
	XML        string
	Screenshot *Screenshot
	Nodes      []PageNode
}

type RuntimeContext struct {
	Step                int
	PackageName         string
	PageSourceType      string
	LastAction          *Action
	LastError           string
	ConsecutiveFailures int
	PageVisitCount      map[string]int
	ActionCount         map[string]int
	BlockRecovery       *BlockRecoveryContext
}

// BlockRecoveryContext 描述当前是否处于阻塞恢复决策阶段。
type BlockRecoveryContext struct {
	Requested bool
	Reason    string
}

type PluginContext struct {
	Page    PageSnapshot
	Runtime RuntimeContext
}

type StepResult struct {
	Step       int
	Action     Action
	Success    bool
	Error      string
	DurationMs int64
	Crash      bool
	ANR        bool
	Before     PageSnapshot
	After      *PageSnapshot
}

type StepResultContext struct {
	PluginContext
	Result StepResult
}

// LifecycleContext 生命周期钩子上下文（onInit / onDestroy），
// 不含页面快照，仅提供基本配置信息。
type LifecycleContext struct {
	PackageName    string
	PageSourceType string
}
