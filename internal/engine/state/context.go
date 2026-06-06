package state

import "strings"

// BuildTraceSignature 将最近动作轨迹拼接为签名字符串，用于记忆匹配。
func BuildTraceSignature(traces []ActionTrace) string {
	if len(traces) == 0 {
		return ""
	}
	parts := make([]string, 0, len(traces))
	for _, item := range traces {
		key := strings.TrimSpace(item.ActionKey)
		if key == "" {
			continue
		}
		parts = append(parts, key)
	}
	return strings.Join(parts, ">")
}

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

// ExecutionRecord 描述单步执行历史，供重复阻塞恢复时传递给 LLM。
type ExecutionRecord struct {
	Step        int    `json:"step"`
	Action      string `json:"action"`
	PageName    string `json:"page_name"`
	AfterPage   string `json:"after_page"`
	Escaped     bool   `json:"escaped"`
	Blocked     bool   `json:"blocked"`
	BlockReason string `json:"block_reason,omitempty"`
}

// CandidateSummary 是供恢复/增强提示词使用的轻量候选摘要。
type CandidateSummary struct {
	ActionKey   string
	ActionType  string
	Source      string
	Intent      string
	Confidence  float64
	EscapeScore float64
	RiskScore   float64
}

// TraversalContext 是第一阶段统一运行上下文骨架。
type TraversalContext struct {
	Step                int
	Mode                Mode
	PageName            string
	PageSignature       string
	ClusterSignature    string
	XML                 string
	Screenshot          []byte
	BlockReason         string
	RecentTrace         []ActionTrace
	VisitStats          VisitStats
	LocalCandidates     []CandidateSummary
	KnownFailedActions  []string
	KnownSuccessActions []string
	ExecutionHistory    []ExecutionRecord
	// VLM 截图编号标注配置（由 coordinator 设置）
	AnnotationEnabled   bool
	AnnotationFontScale int
}

// BuildInput 用于构建 TraversalContext。
type BuildInput struct {
	Step                int
	Mode                Mode
	PageName            string
	PageSignature       string
	ClusterSignature    string
	XML                 string
	Screenshot          []byte
	BlockReason         string
	RecentTrace         []ActionTrace
	PageVisitCount      map[string]int
	ActionCount         map[string]int
	LocalCandidates     []CandidateSummary
	KnownFailedActions  []string
	KnownSuccessActions []string
	ExecutionHistory    []ExecutionRecord
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
		Screenshot:       input.Screenshot,
		BlockReason:      input.BlockReason,
		RecentTrace:      cloneTrace(input.RecentTrace),
		VisitStats: VisitStats{
			PageVisitCount: cloneIntMap(input.PageVisitCount),
			ActionCount:    cloneIntMap(input.ActionCount),
		},
		LocalCandidates:     cloneCandidateSummaries(input.LocalCandidates),
		KnownFailedActions:  cloneStringSlice(input.KnownFailedActions),
		KnownSuccessActions: cloneStringSlice(input.KnownSuccessActions),
		ExecutionHistory:    cloneExecutionHistory(input.ExecutionHistory),
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

func cloneCandidateSummaries(src []CandidateSummary) []CandidateSummary {
	if len(src) == 0 {
		return nil
	}
	result := make([]CandidateSummary, len(src))
	copy(result, src)
	return result
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	result := make([]string, len(src))
	copy(result, src)
	return result
}

func cloneExecutionHistory(src []ExecutionRecord) []ExecutionRecord {
	if len(src) == 0 {
		return nil
	}
	result := make([]ExecutionRecord, len(src))
	copy(result, src)
	return result
}
