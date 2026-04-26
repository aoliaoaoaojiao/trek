package recovery

import (
	"testing"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
)

type stubProvider struct {
	candidates []candidate.Candidate
	calls      int
	err        error
}

func (s *stubProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]candidate.Candidate, error) {
	s.calls++
	return s.candidates, s.err
}

type stubBudget struct {
	allow       bool
	allowCalls  int
	recordCalls int
}

func (b *stubBudget) Allow(ctx enginestate.TraversalContext) bool {
	b.allowCalls++
	return b.allow
}

func (b *stubBudget) Record(ctx enginestate.TraversalContext) {
	b.recordCalls++
}

func TestPlannerBuildRecoveryCandidatesAggregatesProvidersInOrder(t *testing.T) {
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode:        string(enginestate.ModeRecover),
		PageName:    "MainActivity",
		BlockReason: "scroll_no_change",
	})
	memory := &stubProvider{
		candidates: []candidate.Candidate{
			candidate.NewCandidate(&types.ActionCommand{Act: types.BACK}, candidate.SourceMemory, "返回上一层", nil),
		},
	}
	heuristic := &stubProvider{
		candidates: []candidate.Candidate{
			candidate.NewCandidate(&types.ActionCommand{Act: types.CLICK}, candidate.SourceHeuristic, "点击主按钮", nil),
		},
	}
	llm := &stubProvider{
		candidates: []candidate.Candidate{
			candidate.NewCandidate(&types.ActionCommand{Act: types.LONG_CLICK}, candidate.SourceLLM, "长按试探", nil),
		},
	}

	planner := NewPlanner(PlannerConfig{
		Memory:    memory,
		Heuristic: heuristic,
		LLM:       llm,
	})

	items, err := planner.BuildRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("构建恢复候选失败: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("恢复候选数量错误: %d", len(items))
	}
	if items[0].Source != candidate.SourceMemory || items[1].Source != candidate.SourceHeuristic || items[2].Source != candidate.SourceLLM {
		t.Fatalf("恢复候选顺序错误: %#v", []string{items[0].Source, items[1].Source, items[2].Source})
	}
	if memory.calls != 1 || heuristic.calls != 1 || llm.calls != 1 {
		t.Fatalf("provider 调用次数错误: memory=%d heuristic=%d llm=%d", memory.calls, heuristic.calls, llm.calls)
	}
}

func TestPlannerSkipsLLMWhenMemoryHasHighConfidenceCandidate(t *testing.T) {
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode:        string(enginestate.ModeRecover),
		PageName:    "MainActivity",
		BlockReason: "two_state_ping_pong",
	})
	memoryCandidate := candidate.NewCandidate(&types.ActionCommand{Act: types.BACK}, candidate.SourceMemory, "返回上一层", nil)
	memoryCandidate.Confidence = 0.95

	memory := &stubProvider{candidates: []candidate.Candidate{memoryCandidate}}
	heuristic := &stubProvider{
		candidates: []candidate.Candidate{
			candidate.NewCandidate(&types.ActionCommand{Act: types.CLICK}, candidate.SourceHeuristic, "点击主按钮", nil),
		},
	}
	llm := &stubProvider{
		candidates: []candidate.Candidate{
			candidate.NewCandidate(&types.ActionCommand{Act: types.LONG_CLICK}, candidate.SourceLLM, "长按试探", nil),
		},
	}

	planner := NewPlanner(PlannerConfig{
		Memory:                  memory,
		Heuristic:               heuristic,
		LLM:                     llm,
		HighConfidenceThreshold: 0.9,
	})

	items, err := planner.BuildRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("构建恢复候选失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("高置信 memory 命中后应仅保留 memory + heuristic，实际数量: %d", len(items))
	}
	if llm.calls != 0 {
		t.Fatalf("高置信 memory 命中后不应调用 LLM，实际调用: %d", llm.calls)
	}
}

func TestPlannerSkipsLLMWhenBudgetDenied(t *testing.T) {
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:        10,
		Mode:        string(enginestate.ModeRecover),
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	memory := &stubProvider{}
	heuristic := &stubProvider{}
	llm := &stubProvider{
		candidates: []candidate.Candidate{
			candidate.NewCandidate(&types.ActionCommand{Act: types.BACK}, candidate.SourceLLM, "llm back", nil),
		},
	}
	budget := &stubBudget{allow: false}

	planner := NewPlanner(PlannerConfig{
		Memory:    memory,
		Heuristic: heuristic,
		LLM:       llm,
		LLMBudget: budget,
	})
	items, err := planner.BuildRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("构建恢复候选失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("预算拒绝时不应返回 llm 候选，实际: %d", len(items))
	}
	if budget.allowCalls != 1 {
		t.Fatalf("预算 Allow 调用次数错误: %d", budget.allowCalls)
	}
	if llm.calls != 0 {
		t.Fatalf("预算拒绝时不应调用 llm provider，实际: %d", llm.calls)
	}
	if budget.recordCalls != 0 {
		t.Fatalf("预算拒绝时不应记录调用，实际: %d", budget.recordCalls)
	}
}

func TestSlidingWindowLLMBudget(t *testing.T) {
	budget := NewSlidingWindowLLMBudget(2, 5)
	if !budget.Allow(enginestate.TraversalContext{Step: 1}) {
		t.Fatalf("初始预算应允许调用")
	}

	budget.Record(enginestate.TraversalContext{Step: 1})
	budget.Record(enginestate.TraversalContext{Step: 2})
	if budget.Allow(enginestate.TraversalContext{Step: 3}) {
		t.Fatalf("窗口内达到上限后应拒绝调用")
	}

	// 步数窗口前移后，旧记录应淘汰，预算恢复可用。
	if !budget.Allow(enginestate.TraversalContext{Step: 8}) {
		t.Fatalf("窗口过期后应恢复预算")
	}
}
