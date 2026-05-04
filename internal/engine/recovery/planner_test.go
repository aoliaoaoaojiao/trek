package recovery

import (
	"testing"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

type stubProvider struct {
	candidates []perception.Candidate
	calls      int
	err        error
	lastCtx    enginestate.TraversalContext
}

func (s *stubProvider) BuildCandidates(ctx enginestate.TraversalContext) ([]perception.Candidate, error) {
	s.calls++
	s.lastCtx = ctx
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
		Mode:        enginestate.ModeRecover,
		PageName:    "MainActivity",
		BlockReason: "scroll_no_change",
	})
	memory := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceMemory, "返回上一层", nil),
		},
	}
	heuristic := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.CLICK}, perception.SourceHeuristic, "点击主按钮", nil),
		},
	}
	llm := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.LONG_CLICK}, perception.SourceLLM, "长按试探", nil),
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
	if items[0].Source != perception.SourceMemory || items[1].Source != perception.SourceHeuristic || items[2].Source != perception.SourceLLM {
		t.Fatalf("恢复候选顺序错误: %#v", []string{items[0].Source, items[1].Source, items[2].Source})
	}
	if memory.calls != 1 || heuristic.calls != 1 || llm.calls != 1 {
		t.Fatalf("provider 调用次数错误: memory=%d heuristic=%d llm=%d", memory.calls, heuristic.calls, llm.calls)
	}
}

func TestPlannerSkipsLLMWhenMemoryHasHighConfidenceCandidate(t *testing.T) {
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode:        enginestate.ModeRecover,
		PageName:    "MainActivity",
		BlockReason: "two_state_ping_pong",
	})
	memoryCandidate := perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceMemory, "返回上一层", nil)
	memoryCandidate.Confidence = 0.95

	memory := &stubProvider{candidates: []perception.Candidate{memoryCandidate}}
	heuristic := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.CLICK}, perception.SourceHeuristic, "点击主按钮", nil),
		},
	}
	llm := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.LONG_CLICK}, perception.SourceLLM, "长按试探", nil),
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
		Mode:        enginestate.ModeRecover,
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	memory := &stubProvider{}
	heuristic := &stubProvider{}
	llm := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceLLM, "llm back", nil),
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

func TestPlannerPassesLocalCandidateSummaryToLLMContext(t *testing.T) {
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:        3,
		Mode:        enginestate.ModeRecover,
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	memory := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceMemory, "返回", nil),
		},
	}
	heuristic := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}, perception.SourceHeuristic, "点击", nil),
		},
	}
	llm := &stubProvider{}

	planner := NewPlanner(PlannerConfig{
		Memory:    memory,
		Heuristic: heuristic,
		LLM:       llm,
	})
	_, err := planner.BuildRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("构建恢复候选失败: %v", err)
	}
	if llm.calls != 1 {
		t.Fatalf("预期调用 llm provider，实际: %d", llm.calls)
	}
	if len(llm.lastCtx.LocalCandidates) < 2 {
		t.Fatalf("预期 llm 上下文包含 memory+heuristic 本地候选摘要，实际: %d", len(llm.lastCtx.LocalCandidates))
	}
}

func TestPlannerRecordsLLMCallbacks(t *testing.T) {
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Step:        5,
		Mode:        enginestate.ModeRecover,
		PageName:    "MainActivity",
		BlockReason: "same_page_no_change",
	})
	llm := &stubProvider{
		candidates: []perception.Candidate{
			perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceLLM, "llm", nil),
		},
	}
	llmCalls := 0
	denied := 0
	planner := NewPlanner(PlannerConfig{
		LLM:               llm,
		OnLLMCall:         func(ctx enginestate.TraversalContext) { llmCalls++ },
		OnLLMBudgetDenied: func(ctx enginestate.TraversalContext) { denied++ },
	})
	_, err := planner.BuildRecoveryCandidates(ctx)
	if err != nil {
		t.Fatalf("构建恢复候选失败: %v", err)
	}
	if llmCalls != 1 || denied != 0 {
		t.Fatalf("预期调用回调统计正确，实际 call=%d denied=%d", llmCalls, denied)
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
