package state

import "testing"

func TestBuildTraversalContextClonesVisitStatsAndTrace(t *testing.T) {
	pageVisits := map[string]int{"MainActivity": 3}
	actionCount := map[string]int{"CLICK": 2}
	trace := []ActionTrace{
		{PageSignature: "page-a", ActionKey: "CLICK#1"},
		{PageSignature: "page-b", ActionKey: "BACK#1"},
	}
	localCandidates := []CandidateSummary{
		{ActionKey: `{"act":"BACK"}`, ActionType: "BACK", Source: "memory", Confidence: 0.8},
	}
	failed := []string{`{"act":"CLICK"}`}
	success := []string{`{"act":"BACK"}`}

	ctx := BuildTraversalContext(BuildInput{
		Step:                12,
		Mode:                ModeRecover,
		PageName:            "MainActivity",
		PageSignature:       "page-signature",
		ClusterSignature:    "cluster-signature",
		XML:                 "<hierarchy/>",
		Screenshot:          []byte{1, 2, 3},
		BlockReason:         "scroll_no_change",
		RecentTrace:         trace,
		PageVisitCount:      pageVisits,
		ActionCount:         actionCount,
		LocalCandidates:     localCandidates,
		KnownFailedActions:  failed,
		KnownSuccessActions: success,
	})

	if ctx.Step != 12 {
		t.Fatalf("step 不符合预期: %d", ctx.Step)
	}
	if ctx.Mode != ModeRecover {
		t.Fatalf("mode 不符合预期: %s", ctx.Mode)
	}
	if ctx.BlockReason != "scroll_no_change" {
		t.Fatalf("block reason 不符合预期: %s", ctx.BlockReason)
	}
	if len(ctx.RecentTrace) != 2 {
		t.Fatalf("recent trace 长度错误: %d", len(ctx.RecentTrace))
	}
	if ctx.VisitStats.PageVisitCount["MainActivity"] != 3 {
		t.Fatalf("page visit count 未写入")
	}
	if ctx.VisitStats.ActionCount["CLICK"] != 2 {
		t.Fatalf("action count 未写入")
	}
	if len(ctx.LocalCandidates) != 1 || ctx.LocalCandidates[0].ActionType != "BACK" {
		t.Fatalf("local candidates 未写入")
	}
	if len(ctx.KnownFailedActions) != 1 || len(ctx.KnownSuccessActions) != 1 {
		t.Fatalf("known actions 未写入")
	}

	pageVisits["MainActivity"] = 99
	actionCount["CLICK"] = 88
	trace[0].ActionKey = "MUTATED"
	localCandidates[0].ActionType = "MUTATED"
	failed[0] = "MUTATED"
	success[0] = "MUTATED"

	if ctx.VisitStats.PageVisitCount["MainActivity"] != 3 {
		t.Fatalf("page visit count 应为快照副本，实际: %d", ctx.VisitStats.PageVisitCount["MainActivity"])
	}
	if ctx.VisitStats.ActionCount["CLICK"] != 2 {
		t.Fatalf("action count 应为快照副本，实际: %d", ctx.VisitStats.ActionCount["CLICK"])
	}
	if ctx.RecentTrace[0].ActionKey != "CLICK#1" {
		t.Fatalf("recent trace 应为快照副本，实际: %s", ctx.RecentTrace[0].ActionKey)
	}
	if ctx.LocalCandidates[0].ActionType != "BACK" {
		t.Fatalf("local candidates 应为快照副本，实际: %s", ctx.LocalCandidates[0].ActionType)
	}
	if ctx.KnownFailedActions[0] != `{"act":"CLICK"}` || ctx.KnownSuccessActions[0] != `{"act":"BACK"}` {
		t.Fatalf("known actions 应为快照副本，实际: failed=%v success=%v", ctx.KnownFailedActions, ctx.KnownSuccessActions)
	}
}
