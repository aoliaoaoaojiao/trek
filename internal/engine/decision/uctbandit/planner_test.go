package uctbandit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPlanner(t *testing.T) {
	stats := NewStatsStore()
	p := NewPlanner(stats, DefaultPlannerConfig())
	assert.NotNil(t, p)
}

func TestPlannerUCTScoreUnvisitedAction(t *testing.T) {
	stats := NewStatsStore()
	p := NewPlanner(stats, DefaultPlannerConfig())

	// 状态访问过一次，动作从未被选择
	stats.RecordStateVisit("state_1")
	score := p.ComputeUCTScore("state_1", "action_1")
	// 未访问的动作应该有探索加成
	assert.True(t, score > 0, "UCT score for unvisited action should be positive")
}

func TestPlannerUCTScoreVisitedAction(t *testing.T) {
	stats := NewStatsStore()
	p := NewPlanner(stats, DefaultPlannerConfig())

	// 模拟已访问的状态和动作
	stats.RecordStateVisit("state_1")
	stats.RecordActionSelection("state_1", "action_1", 2.0)

	score := p.ComputeUCTScore("state_1", "action_1")
	assert.True(t, score > 0, "UCT score for visited action should be positive")
}

func TestPlannerExplorationDecreasesWithVisits(t *testing.T) {
	stats := NewStatsStore()
	cfg := DefaultPlannerConfig()
	p := NewPlanner(stats, cfg)

	// 模拟多个状态访问但动作只被选一次
	for i := 0; i < 10; i++ {
		stats.RecordStateVisit("state_1")
	}
	stats.RecordActionSelection("state_1", "action_1", 1.0)

	exploreScore1 := p.ComputeExplorationTerm("state_1", "action_1")

	// 增加动作选择次数
	for i := 0; i < 9; i++ {
		stats.RecordActionSelection("state_1", "action_1", 1.0)
	}

	exploreScore2 := p.ComputeExplorationTerm("state_1", "action_1")

	// 随着动作被访问更多次，探索项应该减小
	assert.True(t, exploreScore2 < exploreScore1,
		"exploration term should decrease as action visits increase")
}

func TestPlannerHighMeanRewardGivesHigherScore(t *testing.T) {
	stats := NewStatsStore()
	cfg := DefaultPlannerConfig()
	p := NewPlanner(stats, cfg)

	stats.RecordStateVisit("state_1")
	stats.RecordStateVisit("state_1")
	stats.RecordStateVisit("state_1")

	// 两个动作同样访问次数，但不同的 mean reward
	stats.RecordActionSelection("state_1", "high_reward", 5.0)
	stats.RecordActionSelection("state_1", "high_reward", 5.0)
	stats.RecordActionSelection("state_1", "low_reward", 0.5)
	stats.RecordActionSelection("state_1", "low_reward", 0.5)

	scoreHigh := p.ComputeUCTScore("state_1", "high_reward")
	scoreLow := p.ComputeUCTScore("state_1", "low_reward")

	assert.True(t, scoreHigh > scoreLow,
		"action with higher mean reward should have higher UCT score")
}

func TestPlannerUnvisitedStateDefaultScore(t *testing.T) {
	stats := NewStatsStore()
	cfg := DefaultPlannerConfig()
	p := NewPlanner(stats, cfg)

	// 状态从未被访问 → 默认探索值
	score := p.ComputeUCTScore("unknown_state", "unknown_action")
	assert.True(t, score > 0, "UCT score should still be positive due to default exploration value")
	assert.Equal(t, cfg.ExploreC*2.0, score, "unvisited state and action should get default exploration value")
}

func TestDefaultPlannerConfig(t *testing.T) {
	cfg := DefaultPlannerConfig()
	assert.Equal(t, 1.4, cfg.ExploreC)
	assert.Equal(t, 0.6, cfg.Weight)
}

func TestPlannerWeight(t *testing.T) {
	stats := NewStatsStore()
	cfg := DefaultPlannerConfig()
	p := NewPlanner(stats, cfg)
	assert.Equal(t, 0.6, p.Weight())
}