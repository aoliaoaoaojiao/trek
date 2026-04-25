package uctbandit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRewarder(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	assert.NotNil(t, r)
}

func TestRewardNewState(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    true,
		FoundNewEdge:     false,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s2",
		RecentStateIDs:   []string{"s0"},
	})
	assert.True(t, result.FoundNewState)
	assert.Equal(t, DefaultRewardConfig().NewStateReward, result.Reward)
}

func TestRewardNewEdge(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     true,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s2",
		RecentStateIDs:   []string{"s0"},
	})
	assert.True(t, result.FoundNewEdge)
	assert.Equal(t, DefaultRewardConfig().NewEdgeReward, result.Reward)
}

func TestRewardNoOp(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     false,
		StructureChanged: false,
		IsNoOp:           true,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s1",
		RecentStateIDs:   []string{},
	})
	assert.True(t, result.IsNoOp)
	assert.Equal(t, DefaultRewardConfig().NoOpPenalty, result.Reward)
}

func TestRewardShortLoop(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     false,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s0",
		RecentStateIDs:   []string{"s0", "s1"},
	})
	assert.True(t, result.IsShortLoop)
	assert.Equal(t, DefaultRewardConfig().ShortLoopPenalty, result.Reward)
}

func TestRewardEmptyResult(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     false,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    true,
		PrevStateID:      "s1",
		NextStateID:      "",
		RecentStateIDs:   []string{},
	})
	assert.Equal(t, DefaultRewardConfig().EmptyResultPenalty, result.Reward)
}

func TestRewardStructureChanged(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     false,
		StructureChanged: true,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s1",
		RecentStateIDs:   []string{},
	})
	assert.True(t, result.StructureChanged)
	assert.Equal(t, DefaultRewardConfig().StructureChangeReward, result.Reward)
}

func TestRewardCombinedSignals(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    true,
		FoundNewEdge:     true,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s2",
		RecentStateIDs:   []string{"s0"},
	})
	assert.True(t, result.FoundNewState)
	assert.True(t, result.FoundNewEdge)
	// 新状态和新边奖励叠加
	expectedReward := DefaultRewardConfig().NewStateReward + DefaultRewardConfig().NewEdgeReward
	assert.Equal(t, expectedReward, result.Reward)
}

func TestNeedEscape(t *testing.T) {
	cfg := DefaultRewardConfig()
	cfg.StagnationThreshold = 2
	r := NewRewarder(cfg)
	// 停滞次数 >= 阈值 → 需要 escape
	assert.True(t, r.NeedEscape(2))
	assert.True(t, r.NeedEscape(3))
	assert.False(t, r.NeedEscape(1))
}

func TestShortLoopNotTriggeredOnFirstVisit(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	// 空的近期状态列表，不应触发短环
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     false,
		StructureChanged: false,
		IsNoOp:           true,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s2",
		RecentStateIDs:   []string{},
	})
	assert.False(t, result.IsShortLoop)
}

func TestRewardReason(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    true,
		FoundNewEdge:     true,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s2",
		RecentStateIDs:   []string{"s0"},
	})
	assert.Equal(t, "new_state,new_edge", result.Reason)
}

func TestRewardNoSignal(t *testing.T) {
	r := NewRewarder(DefaultRewardConfig())
	result := r.ComputeReward(RewardInput{
		FoundNewState:    false,
		FoundNewEdge:     false,
		StructureChanged: false,
		IsNoOp:           false,
		IsShortLoop:      false,
		IsEmptyResult:    false,
		PrevStateID:      "s1",
		NextStateID:      "s2",
		RecentStateIDs:   []string{},
	})
	assert.Equal(t, 0.0, result.Reward)
	assert.Equal(t, "none", result.Reason)
}