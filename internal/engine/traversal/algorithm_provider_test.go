package traversal_test

import (
	"testing"

	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
	"trek/internal/engine/traversal"
)

// mockProviderAlgorithm 用于测试 AlgorithmProvider 的候选生成。
type mockProviderAlgorithm struct {
	candidates []candidate.Candidate
	name       string
}

func (m *mockProviderAlgorithm) Name() string { return m.name }

func (m *mockProviderAlgorithm) ProposeCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	return m.candidates, nil
}

func (m *mockProviderAlgorithm) SelectAction(ctx enginestate.TraversalContext, candidates []candidate.Candidate) (*types.ActionCommand, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates[0].Command, nil
}

func (m *mockProviderAlgorithm) ObserveOutcome(ctx enginestate.TraversalContext, action *types.ActionCommand, outcome traversal.ActionOutcome) error {
	return nil
}

func TestAlgorithmProviderBuildCandidates(t *testing.T) {
	algo := &mockProviderAlgorithm{
		name: "mock_algo",
		candidates: []candidate.Candidate{
			candidate.NewCandidate(nil, candidate.SourceAlgorithm, "click_search", nil),
		},
	}
	provider := traversal.NewAlgorithmProvider(algo)

	ctx := enginestate.TraversalContext{Step: 1, Mode: "Explore"}
	result, err := provider.BuildCandidates(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result))
	}
	if result[0].Source != candidate.SourceAlgorithm {
		t.Errorf("expected source %s, got %s", candidate.SourceAlgorithm, result[0].Source)
	}
	if result[0].Intent != "click_search" {
		t.Errorf("expected intent click_search, got %s", result[0].Intent)
	}
}

func TestAlgorithmProviderWithNilAlgorithm(t *testing.T) {
	provider := traversal.NewAlgorithmProvider(nil)
	ctx := enginestate.TraversalContext{Step: 1}
	result, err := provider.BuildCandidates(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 candidates for nil algorithm, got %d", len(result))
	}
}

func TestAlgorithmProviderEmptyCandidates(t *testing.T) {
	algo := &mockProviderAlgorithm{name: "empty_algo", candidates: nil}
	provider := traversal.NewAlgorithmProvider(algo)

	ctx := enginestate.TraversalContext{Step: 1}
	result, err := provider.BuildCandidates(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 candidates, got %d", len(result))
	}
}