package state

// Mode 表示当前遍历阶段。
type Mode string

const (
	ModeExplore        Mode = "Explore"
	ModeSuspectBlocked Mode = "SuspectBlocked"
	ModeRecover        Mode = "Recover"
	ModeCooldown       Mode = "Cooldown"
)

// ActionTrace 描述最近动作轨迹的最小公共信息。
type ActionTrace struct {
	PageSignature string
	ActionKey     string
}

// VisitStats 表示可跨算法共享的访问统计快照。
type VisitStats struct {
	PageVisitCount map[string]int
	ActionCount    map[string]int
}

// TraversalContext 是第一阶段统一运行上下文骨架。
type TraversalContext struct {
	Step             int
	Mode             string
	PageName         string
	PageSignature    string
	ClusterSignature string
	XML              string
	Screenshot       []byte
	BlockReason      string
	RecentTrace      []ActionTrace
	VisitStats       VisitStats
}

// BuildInput 用于构建 TraversalContext。
type BuildInput struct {
	Step             int
	Mode             string
	PageName         string
	PageSignature    string
	ClusterSignature string
	XML              string
	Screenshot       []byte
	BlockReason      string
	RecentTrace      []ActionTrace
	PageVisitCount   map[string]int
	ActionCount      map[string]int
}

// BuildTraversalContext 基于输入构建独立快照，避免运行时状态泄漏到公共层。
func BuildTraversalContext(input BuildInput) TraversalContext {
	return TraversalContext{
		Step:             input.Step,
		Mode:             input.Mode,
		PageName:         input.PageName,
		PageSignature:    input.PageSignature,
		ClusterSignature: input.ClusterSignature,
		XML:              input.XML,
		Screenshot:       append([]byte(nil), input.Screenshot...),
		BlockReason:      input.BlockReason,
		RecentTrace:      cloneTrace(input.RecentTrace),
		VisitStats: VisitStats{
			PageVisitCount: cloneIntMap(input.PageVisitCount),
			ActionCount:    cloneIntMap(input.ActionCount),
		},
	}
}

func cloneTrace(trace []ActionTrace) []ActionTrace {
	if len(trace) == 0 {
		return nil
	}
	result := make([]ActionTrace, len(trace))
	copy(result, trace)
	return result
}

func cloneIntMap(src map[string]int) map[string]int {
	if len(src) == 0 {
		return map[string]int{}
	}
	result := make(map[string]int, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}
