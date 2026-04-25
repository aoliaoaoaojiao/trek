package uctbandit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStatsStore(t *testing.T) {
	store := NewStatsStore()
	assert.NotNil(t, store)
}

func TestStateStats(t *testing.T) {
	store := NewStatsStore()

	// 初始状态不存在
	stats, ok := store.GetStateStats("state_1")
	assert.False(t, ok)

	// 记录状态访问
	store.RecordStateVisit("state_1")
	stats, ok = store.GetStateStats("state_1")
	assert.True(t, ok)
	assert.Equal(t, 1, stats.Visits)

	// 再次访问
	store.RecordStateVisit("state_1")
	stats, _ = store.GetStateStats("state_1")
	assert.Equal(t, 2, stats.Visits)
}

func TestActionStats(t *testing.T) {
	store := NewStatsStore()

	// 初始动作不存在
	stats, ok := store.GetActionStats("state_1", "action_1")
	assert.False(t, ok)

	// 记录动作选择
	store.RecordActionSelection("state_1", "action_1", 3.0)
	stats, ok = store.GetActionStats("state_1", "action_1")
	assert.True(t, ok)
	assert.Equal(t, 1, stats.Visits)
	assert.Equal(t, 3.0, stats.TotalReward)
	assert.Equal(t, 3.0, stats.MeanReward)

	// 再次选择，reward 不同
	store.RecordActionSelection("state_1", "action_1", 1.0)
	stats, _ = store.GetActionStats("state_1", "action_1")
	assert.Equal(t, 2, stats.Visits)
	assert.Equal(t, 4.0, stats.TotalReward)
	assert.Equal(t, 2.0, stats.MeanReward)
}

func TestBanditArmStats(t *testing.T) {
	store := NewStatsStore()

	// 初始 arm 不存在
	stats, ok := store.GetArmStats("arm_1")
	assert.False(t, ok)

	// 记录 arm 拉取
	store.RecordArmPull("arm_1", 2.5)
	stats, ok = store.GetArmStats("arm_1")
	assert.True(t, ok)
	assert.Equal(t, 1, stats.Pulls)
	assert.Equal(t, 2.5, stats.TotalReward)
	assert.Equal(t, 2.5, stats.MeanReward)

	// 再次拉取
	store.RecordArmPull("arm_1", 1.5)
	stats, _ = store.GetArmStats("arm_1")
	assert.Equal(t, 2, stats.Pulls)
	assert.Equal(t, 4.0, stats.TotalReward)
	assert.Equal(t, 2.0, stats.MeanReward)
}

func TestTotalStateVisits(t *testing.T) {
	store := NewStatsStore()
	assert.Equal(t, 0, store.TotalStateVisits())

	store.RecordStateVisit("state_1")
	assert.Equal(t, 1, store.TotalStateVisits())

	store.RecordStateVisit("state_2")
	assert.Equal(t, 2, store.TotalStateVisits())
}

func TestTotalArmPulls(t *testing.T) {
	store := NewStatsStore()
	assert.Equal(t, 0, store.TotalArmPulls())

	store.RecordArmPull("arm_1", 1.0)
	assert.Equal(t, 1, store.TotalArmPulls())

	store.RecordArmPull("arm_2", 2.0)
	assert.Equal(t, 2, store.TotalArmPulls())
}

func TestStateStagnation(t *testing.T) {
	store := NewStatsStore()

	store.RecordStateVisit("state_1")
	stats, _ := store.GetStateStats("state_1")
	assert.Equal(t, 0, stats.StagnationCount)

	store.IncrementStagnation("state_1")
	stats, _ = store.GetStateStats("state_1")
	assert.Equal(t, 1, stats.StagnationCount)

	store.ResetStagnation("state_1")
	stats, _ = store.GetStateStats("state_1")
	assert.Equal(t, 0, stats.StagnationCount)
}

func TestConcurrentAccess(t *testing.T) {
	store := NewStatsStore()

	// 并发访问不应该 panic
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			stateID := "state_concurrent"
			store.RecordStateVisit(stateID)
			store.RecordActionSelection(stateID, "action_concurrent", float64(id))
			store.RecordArmPull("arm_concurrent", float64(id))
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证总访问数正确
	stats, ok := store.GetStateStats("state_concurrent")
	assert.True(t, ok)
	assert.Equal(t, 10, stats.Visits)
}