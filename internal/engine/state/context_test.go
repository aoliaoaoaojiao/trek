package state

import "testing"

func TestBuildTraversalContextClonesVisitStatsAndTrace(t *testing.T) {
	pageVisits := map[string]int{"MainActivity": 3}
	actionCount := map[string]int{"CLICK": 2}
	trace := []ActionTrace{
		{PageSignature: "page-a", ActionKey: "CLICK#1"},
		{PageSignature: "page-b", ActionKey: "BACK#1"},
	}

	ctx := BuildTraversalContext(BuildInput{
		Step:             12,
		Mode:             string(ModeRecover),
		PageName:         "MainActivity",
		PageSignature:    "page-signature",
		ClusterSignature: "cluster-signature",
		XML:              "<hierarchy/>",
		Screenshot:       []byte{1, 2, 3},
		BlockReason:      "scroll_no_change",
		RecentTrace:      trace,
		PageVisitCount:   pageVisits,
		ActionCount:      actionCount,
	})

	if ctx.Step != 12 {
		t.Fatalf("step 不符合预期: %d", ctx.Step)
	}
	if ctx.Mode != string(ModeRecover) {
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

	pageVisits["MainActivity"] = 99
	actionCount["CLICK"] = 88
	trace[0].ActionKey = "MUTATED"

	if ctx.VisitStats.PageVisitCount["MainActivity"] != 3 {
		t.Fatalf("page visit count 应为快照副本，实际: %d", ctx.VisitStats.PageVisitCount["MainActivity"])
	}
	if ctx.VisitStats.ActionCount["CLICK"] != 2 {
		t.Fatalf("action count 应为快照副本，实际: %d", ctx.VisitStats.ActionCount["CLICK"])
	}
	if ctx.RecentTrace[0].ActionKey != "CLICK#1" {
		t.Fatalf("recent trace 应为快照副本，实际: %s", ctx.RecentTrace[0].ActionKey)
	}
}
