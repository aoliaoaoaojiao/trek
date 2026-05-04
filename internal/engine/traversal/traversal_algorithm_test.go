package traversal_test

import (
	"testing"

	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
)

// mockAlgorithm 用于验证接口编译通过。
type mockAlgorithm struct{}

func (m *mockAlgorithm) Name() string { return "mock" }
func (m *mockAlgorithm) ProposeCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	return nil, nil
}
func (m *mockAlgorithm) SelectAction(ctx enginestate.TraversalContext, candidates []perception.Candidate) (*types.ActionCommand, error) {
	return nil, nil
}
func (m *mockAlgorithm) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
	return nil
}

// TestTraversalAlgorithmInterfaceCompiles 验证接口定义编译通过。
func TestTraversalAlgorithmInterfaceCompiles(t *testing.T) {
	var _ traversal.TraversalAlgorithm = (*mockAlgorithm)(nil)
}

// TestActionOutcomeConstants 验证 ActionOutcome 常量值。
func TestActionOutcomeConstants(t *testing.T) {
	tests := []struct {
		name     string
		outcome  traversal.ActionOutcome
		expected string
	}{
		{"new_state", traversal.OutcomeNewState, "new_state"},
		{"same_state", traversal.OutcomeSameState, "same_state"},
		{"escape_block", traversal.OutcomeEscapeBlock, "escape_block"},
		{"loop", traversal.OutcomeLoop, "loop"},
		{"no_op", traversal.OutcomeNoOp, "no_op"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.outcome) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.outcome)
			}
		})
	}
}
