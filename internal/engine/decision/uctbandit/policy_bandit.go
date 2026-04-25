package uctbandit

import (
	"math"
	"trek/logger"
)

// BanditConfig 定义 Bandit 打分的参数。
type BanditConfig struct {
	ExploreC float64 // Bandit 探索系数，默认 1.0
	Weight   float64 // Bandit 分数在最终分数中的权重，默认 0.3
}

// DefaultBanditConfig 返回默认的 Bandit 配置。
func DefaultBanditConfig() BanditConfig {
	return BanditConfig{
		ExploreC: 1.0,
		Weight:   0.3,
	}
}

// BanditPolicy 负责动作级 UCB bandit 打分。
type BanditPolicy struct {
	stats  *StatsStore
	config BanditConfig
}

// NewBanditPolicy 创建新的 BanditPolicy。
func NewBanditPolicy(stats *StatsStore, config BanditConfig) *BanditPolicy {
	return &BanditPolicy{
		stats:  stats,
		config: config,
	}
}

// ComputeBanditScore 计算某个 arm 的 bandit 分数。
// bandit_score = arm_mean_reward + c * sqrt(ln(total_pulls+1) / (arm_pulls+1))
func (p *BanditPolicy) ComputeBanditScore(armKey string) float64 {
	armStats, exists := p.stats.GetArmStats(armKey)
	totalPulls := p.stats.TotalArmPulls()

	if !exists || armStats.Pulls == 0 {
		// 未拉取的 arm，给予默认探索值
		defaultScore := p.config.ExploreC * 2.0
		logger.Debugf("Bandit: arm=%s score=%.4f (unvisited, default)", armKey, defaultScore)
		return defaultScore
	}

	exploration := p.ComputeExplorationTerm(armKey)
	score := armStats.MeanReward + exploration

	logger.Debugf("Bandit: arm=%s mean=%.4f explore=%.4f total=%.4f pulls=%d totalPulls=%d",
		armKey, armStats.MeanReward, exploration, score, armStats.Pulls, totalPulls)

	return score
}

// ComputeExplorationTerm 计算 Bandit 探索项。
// c * sqrt(ln(total_pulls+1) / (arm_pulls+1))
func (p *BanditPolicy) ComputeExplorationTerm(armKey string) float64 {
	armStats, exists := p.stats.GetArmStats(armKey)
	totalPulls := p.stats.TotalArmPulls()

	if !exists || armStats.Pulls == 0 {
		return p.config.ExploreC * 2.0
	}

	return p.config.ExploreC * math.Sqrt(
		math.Log(float64(totalPulls)+1)/float64(armStats.Pulls+1),
	)
}

// Weight 返回 Bandit 权重。
func (p *BanditPolicy) Weight() float64 {
	return p.config.Weight
}