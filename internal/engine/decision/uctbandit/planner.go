package uctbandit

import (
	"math"
	"trek/logger"
)

// PlannerConfig 定义 UCT-Lite 打分的参数。
type PlannerConfig struct {
	ExploreC float64 // UCT 探索系数，默认 1.4
	Weight   float64 // UCT 分数在最终分数中的权重，默认 0.6
}

// DefaultPlannerConfig 返回默认的 Planner 配置。
func DefaultPlannerConfig() PlannerConfig {
	return PlannerConfig{
		ExploreC: 1.4,
		Weight:   0.6,
	}
}

// Planner 负责 UCT-Lite 页面级打分。
type Planner struct {
	stats  *StatsStore
	config PlannerConfig
}

// NewPlanner 创建新的 Planner。
func NewPlanner(stats *StatsStore, config PlannerConfig) *Planner {
	return &Planner{
		stats:  stats,
		config: config,
	}
}

// ComputeUCTScore 计算状态-动作对的 UCT 分数。
// uct(a) = Q(s,a) + c * sqrt(ln(N(s)+1) / (N(s,a)+1))
func (p *Planner) ComputeUCTScore(stateID, actionKey string) float64 {
	stateVisits := p.stats.StateVisits(stateID)
	actionVisits := p.stats.ActionVisits(stateID, actionKey)
	meanReward := p.stats.ActionMeanReward(stateID, actionKey)

	exploration := p.ComputeExplorationTerm(stateID, actionKey)

	score := meanReward + exploration

	// 当状态和动作都未访问时，给予默认探索值
	if stateVisits == 0 && actionVisits == 0 {
		score = p.config.ExploreC * 2.0
	}

	logger.Debugf("UCT: state=%s action=%s Q=%.4f explore=%.4f total=%.4f N(s)=%d N(s,a)=%d",
		stateID, actionKey, meanReward, exploration, score, stateVisits, actionVisits)

	return score
}

// ComputeExplorationTerm 计算 UCT 探索项。
// c * sqrt(ln(N(s)+1) / (N(s,a)+1))
func (p *Planner) ComputeExplorationTerm(stateID, actionKey string) float64 {
	stateVisits := p.stats.StateVisits(stateID)
	actionVisits := p.stats.ActionVisits(stateID, actionKey)

	if actionVisits == 0 && stateVisits == 0 {
		return p.config.ExploreC * 2.0
	}

	if actionVisits == 0 {
		// 未访问的动作，给最大探索激励
		return p.config.ExploreC * math.Sqrt(math.Log(float64(stateVisits)+1))
	}

	return p.config.ExploreC * math.Sqrt(math.Log(float64(stateVisits)+1)/float64(actionVisits+1))
}

// Weight 返回 UCT 权重。
func (p *Planner) Weight() float64 {
	return p.config.Weight
}