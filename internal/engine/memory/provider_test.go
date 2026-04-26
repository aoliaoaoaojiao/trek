package memory

import (
	"path/filepath"
	"testing"
	"time"
	"trek/internal/engine/candidate"
	types2 "trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
)

func TestProviderBuildCandidatesFromStore(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "memory.jsonl"))
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}

	now := time.Now().UTC()
	record := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "scroll_no_change", "CLICK>BACK", string(enginestate.ModeRecover)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		TraceSignature:   "CLICK>BACK",
		Mode:             string(enginestate.ModeRecover),
		Candidate: candidate.NewCandidate(
			&types2.ActionCommand{Act: types2.BACK},
			candidate.SourceMemory,
			"返回上一层",
			nil,
		),
		Outcome:      OutcomeEscaped,
		EscapeScore:  0.7,
		SuccessCount: 4,
		FailCount:    1,
		LastUsedAt:   now,
		CreatedAt:    now,
	}
	if err := store.Append(record); err != nil {
		t.Fatalf("追加记录失败: %v", err)
	}

	provider := NewProvider(store)
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode:             string(enginestate.ModeRecover),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "CLICK"},
			{ActionKey: "BACK"},
		},
	})

	items, err := provider.BuildCandidates(ctx)
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("候选数量错误: %d", len(items))
	}
	if items[0].Source != candidate.SourceMemory {
		t.Fatalf("候选来源错误: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types2.BACK {
		t.Fatalf("候选命令错误: %+v", items[0].Command)
	}
	if items[0].Confidence <= 0 {
		t.Fatalf("候选 confidence 应大于 0，实际: %.3f", items[0].Confidence)
	}
}
