package uctbandit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBanditPolicy(t *testing.T) {
	stats := NewStatsStore()
	p := NewBanditPolicy(stats, DefaultBanditConfig())
	assert.NotNil(t, p)
}

func TestBanditScoreUnvisitedArm(t *testing.T) {
	stats := NewStatsStore()
	p := NewBanditPolicy(stats, DefaultBanditConfig())

	score := p.ComputeBanditScore("arm_1")
	// 未拉取的 arm 应该有默认探索加成
	assert.True(t, score > 0, "bandit score for unvisited arm should be positive")
}

func TestBanditScoreVisitedArm(t *testing.T) {
	stats := NewStatsStore()
	p := NewBanditPolicy(stats, DefaultBanditConfig())

	// 拉取 arm 一次
	stats.RecordArmPull("arm_1", 3.0)
	// 再全局拉取几次其他 arm
	stats.RecordArmPull("arm_2", 1.0)
	stats.RecordArmPull("arm_2", 1.0)

	score := p.ComputeBanditScore("arm_1")
	assert.True(t, score > 0, "bandit score for visited arm should be positive")
}

func TestBanditScoreHigherRewardHigherScore(t *testing.T) {
	stats := NewStatsStore()
	p := NewBanditPolicy(stats, DefaultBanditConfig())

	// 两个 arm 相同拉取次数，不同 reward
	stats.RecordArmPull("high_arm", 5.0)
	stats.RecordArmPull("low_arm", 0.5)
	stats.RecordArmPull("high_arm", 5.0)
	stats.RecordArmPull("low_arm", 0.5)

	scoreHigh := p.ComputeBanditScore("high_arm")
	scoreLow := p.ComputeBanditScore("low_arm")

	assert.True(t, scoreHigh > scoreLow,
		"arm with higher mean reward should have higher bandit score")
}

func TestBanditScoreExplorationDecreases(t *testing.T) {
	stats := NewStatsStore()
	p := NewBanditPolicy(stats, DefaultBanditConfig())

	// arm_1 拉取1次
	stats.RecordArmPull("arm_1", 2.0)
	explore1 := p.ComputeExplorationTerm("arm_1")

	// 增加总拉取和 arm_1 拉取次数
	for i := 0; i < 9; i++ {
		stats.RecordArmPull("arm_other", 1.0)
	}
	for i := 0; i < 9; i++ {
		stats.RecordArmPull("arm_1", 2.0)
	}
	explore10 := p.ComputeExplorationTerm("arm_1")

	assert.True(t, explore10 < explore1,
		"exploration term should decrease as arm pulls increase")
}

func TestDefaultBanditConfig(t *testing.T) {
	cfg := DefaultBanditConfig()
	assert.Equal(t, 1.0, cfg.ExploreC)
	assert.Equal(t, 0.3, cfg.Weight)
}

func TestBanditPolicyWeight(t *testing.T) {
	stats := NewStatsStore()
	cfg := DefaultBanditConfig()
	p := NewBanditPolicy(stats, cfg)
	assert.Equal(t, 0.3, p.Weight())
}