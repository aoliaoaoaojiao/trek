package uctbandit

import "trek/logger"

// RewardConfig 定义 reward 计算的参数配置。
type RewardConfig struct {
	NewStateReward        float64 // 首次到达新状态
	NewEdgeReward         float64 // 首次发现新边
	StructureChangeReward float64 // 页面名相同但结构变化
	NoOpPenalty           float64 // 页面无变化
	ShortLoopPenalty      float64 // 短环惩罚
	TwoStateLoopPenalty   float64 // 双状态往返惩罚（A↔B）
	EdgeRepeatPenalty     float64 // 重复边惩罚
	EdgeRepeatThreshold   int     // 重复边阈值（超过后开始惩罚）
	EmptyResultPenalty    float64 // 空结果/连续无效
	ShortLoopWindow       int     // 短环检测窗口大小
	StagnationThreshold   int     // 停滞阈值
}

// DefaultRewardConfig 返回第一阶段的默认 reward 参数。
func DefaultRewardConfig() RewardConfig {
	return RewardConfig{
		NewStateReward:        5.0,
		NewEdgeReward:         3.0,
		StructureChangeReward: 2.0,
		NoOpPenalty:           -1.0,
		ShortLoopPenalty:      -2.0,
		TwoStateLoopPenalty:   -3.0,
		EdgeRepeatPenalty:     -1.0,
		EdgeRepeatThreshold:   2,
		EmptyResultPenalty:    -3.0,
		ShortLoopWindow:       3,
		StagnationThreshold:   2,
	}
}

// RewardInput 是 reward 计算的输入。
type RewardInput struct {
	FoundNewState    bool
	FoundNewEdge     bool
	StructureChanged bool
	IsNoOp           bool
	IsShortLoop      bool
	IsEmptyResult    bool
	PrevStateID      string
	NextStateID      string
	EdgeRepeatCount  int
	RecentStateIDs   []string // 最近的 state ID 列表，用于短环检测
}

// RewardResult 是 reward 计算的结果。
type RewardResult struct {
	Reward           float64
	FoundNewState    bool
	FoundNewEdge     bool
	StructureChanged bool
	IsNoOp           bool
	IsShortLoop      bool
	IsTwoStateLoop   bool
	NeedEscape       bool
	Reason           string
}

// Rewarder 负责将动作结果映射为 reward 值。
type Rewarder struct {
	config RewardConfig
}

// NewRewarder 创建新的 Rewarder。
func NewRewarder(config RewardConfig) *Rewarder {
	return &Rewarder{config: config}
}

// UpdateConfig 更新配置，避免每步重建 Rewarder。
func (r *Rewarder) UpdateConfig(config RewardConfig) {
	r.config = config
}

// ComputeReward 根据输入计算 reward。
func (r *Rewarder) ComputeReward(input RewardInput) RewardResult {
	result := RewardResult{
		FoundNewState:    input.FoundNewState,
		FoundNewEdge:     input.FoundNewEdge,
		StructureChanged: input.StructureChanged,
		IsNoOp:           input.IsNoOp,
	}

	var reward float64
	var reasons []string

	// 短环检测：下一状态在最近窗口内
	isShortLoop := r.detectShortLoop(input)
	result.IsShortLoop = isShortLoop
	isTwoStateLoop := r.detectTwoStateLoop(input)
	result.IsTwoStateLoop = isTwoStateLoop

	// 按优先级叠加 reward
	if input.FoundNewState {
		reward += r.config.NewStateReward
		reasons = append(reasons, "new_state")
	}
	if input.FoundNewEdge {
		reward += r.config.NewEdgeReward
		reasons = append(reasons, "new_edge")
	}
	if input.StructureChanged {
		reward += r.config.StructureChangeReward
		reasons = append(reasons, "structure_changed")
	}
	if input.IsEmptyResult {
		reward += r.config.EmptyResultPenalty
		reasons = append(reasons, "empty_result")
	} else if input.IsNoOp {
		reward += r.config.NoOpPenalty
		reasons = append(reasons, "noop")
	}
	if isShortLoop {
		reward += r.config.ShortLoopPenalty
		reasons = append(reasons, "short_loop")
	}
	if isTwoStateLoop {
		reward += r.config.TwoStateLoopPenalty
		reasons = append(reasons, "two_state_loop")
	}
	if input.EdgeRepeatCount > r.config.EdgeRepeatThreshold {
		overCount := input.EdgeRepeatCount - r.config.EdgeRepeatThreshold
		reward += r.config.EdgeRepeatPenalty * float64(overCount)
		reasons = append(reasons, "edge_repeat")
	}

	result.Reward = reward
	result.Reason = joinReasons(reasons)

	logger.Debugf("reward computed: reward=%.2f reason=%s newState=%v newEdge=%v noop=%v shortLoop=%v twoStateLoop=%v edgeRepeat=%d",
		reward, result.Reason, input.FoundNewState, input.FoundNewEdge, input.IsNoOp, isShortLoop, isTwoStateLoop, input.EdgeRepeatCount)

	return result
}

// NeedEscape 判断是否需要逃逸（停滞次数达到阈值）。
func (r *Rewarder) NeedEscape(stagnationCount int) bool {
	return stagnationCount >= r.config.StagnationThreshold
}

// detectShortLoop 检测是否为短环。
func (r *Rewarder) detectShortLoop(input RewardInput) bool {
	if input.NextStateID == "" {
		return false
	}
	window := r.config.ShortLoopWindow
	if len(input.RecentStateIDs) < 1 {
		return false
	}
	start := len(input.RecentStateIDs) - window
	if start < 0 {
		start = 0
	}
	for i := start; i < len(input.RecentStateIDs); i++ {
		if input.RecentStateIDs[i] == input.NextStateID && input.RecentStateIDs[i] != input.PrevStateID {
			return true
		}
	}
	return false
}

// detectTwoStateLoop 检测是否触发双状态往返（A↔B）模式。
func (r *Rewarder) detectTwoStateLoop(input RewardInput) bool {
	if input.PrevStateID == "" || input.NextStateID == "" || len(input.RecentStateIDs) < 2 {
		return false
	}
	n := len(input.RecentStateIDs)
	return input.RecentStateIDs[n-2] == input.NextStateID && input.RecentStateIDs[n-1] == input.PrevStateID
}

// joinReasons 拼接多个标签。
func joinReasons(reasons []string) string {
	if len(reasons) == 0 {
		return "none"
	}
	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += "," + reasons[i]
	}
	return result
}
