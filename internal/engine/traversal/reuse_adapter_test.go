package traversal_test

import (
	"testing"

	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/traversal"
)

func TestReuseAdapterName(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	if adapter.Name() != "reuse_adapter" {
		t.Errorf("expected name reuse_adapter, got %s", adapter.Name())
	}
}

func TestReuseAdapterProposeCandidatesWithNilAgent(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()
	candidates, err := adapter.ProposeCandidates(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates with nil agent, got %d", len(candidates))
	}
}

func TestReuseAdapterSelectActionWithNoCandidates(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()
	cmd, err := adapter.SelectAction(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != nil {
		t.Fatalf("expected nil action for empty candidates")
	}
}

func TestReuseAdapterSelectActionEmptySlice(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()
	cmd, err := adapter.SelectAction(ctx, []candidate.Candidate{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd != nil {
		t.Fatalf("expected nil action for empty candidate slice")
	}
}

func TestReuseAdapterObserveOutcomeNoOp(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()
	err := adapter.ObserveOutcome(ctx, nil, traversal.OutcomeSameState)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReuseAdapterObserveOutcomeAffectsSelection(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()

	backCmd := types.NewActionCommand()
	backCmd.Act = types.BACK
	clickCmd := types.NewActionCommand()
	clickCmd.Act = types.CLICK
	clickCmd.Pos = *types.NewRect(0.1, 0.2, 0.3, 0.4)

	if err := adapter.ObserveOutcome(ctx, backCmd, traversal.OutcomeLoop); err != nil {
		t.Fatalf("observe outcome 失败: %v", err)
	}

	backCandidate := candidate.NewCandidate(backCmd, candidate.SourceMemory, "back", nil)
	backCandidate.Confidence = 0.5
	clickCandidate := candidate.NewCandidate(clickCmd, candidate.SourceMemory, "click", nil)
	clickCandidate.Confidence = 0.5

	cmd, err := adapter.SelectAction(ctx, []candidate.Candidate{backCandidate, clickCandidate})
	if err != nil {
		t.Fatalf("select action 失败: %v", err)
	}
	if cmd == nil || cmd.Act != types.CLICK {
		t.Fatalf("预期负反馈后优先选择 CLICK，实际: %+v", cmd)
	}
}

func TestReuseAdapterSelectActionPrefersAlgorithmCandidates(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()

	clickCmd := types.NewActionCommand()
	clickCmd.Act = types.CLICK
	clickCmd.Pos = *types.NewRect(0.1, 0.2, 0.3, 0.4)

	backCmd := types.NewActionCommand()
	backCmd.Act = types.BACK

	candidates := []candidate.Candidate{
		candidate.NewCandidate(backCmd, candidate.SourceMemory, "go_back", nil),
		candidate.NewCandidate(clickCmd, candidate.SourceAlgorithm, "click_search", nil),
		candidate.NewCandidate(backCmd, candidate.SourceHeuristic, "escape_back", nil),
	}

	cmd, err := adapter.SelectAction(ctx, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil action")
	}
	if cmd.Act != types.CLICK {
		t.Errorf("expected CLICK action from algorithm source, got %s", cmd.Act)
	}
}

func TestReuseAdapterSelectActionFallbackToAnyValid(t *testing.T) {
	adapter := traversal.NewReuseAdapter(nil, nil)
	ctx := testTraversalContext()

	backCmd := types.NewActionCommand()
	backCmd.Act = types.BACK

	candidates := []candidate.Candidate{
		candidate.NewCandidate(backCmd, candidate.SourceMemory, "go_back", nil),
	}

	cmd, err := adapter.SelectAction(ctx, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil action from fallback")
	}
	if cmd.Act != types.BACK {
		t.Errorf("expected BACK action from fallback, got %s", cmd.Act)
	}
}
