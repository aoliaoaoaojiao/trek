package memory

import (
	"path/filepath"
	"testing"
	"time"
	"trek/internal/engine/candidate"
	"trek/internal/engine/decision/shared/types"
	enginestate "trek/internal/engine/state"
)

func TestStoreAppendAndReloadFromJSONL(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "recovery_memory.jsonl")

	store, err := NewStore(filePath)
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	record := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "scroll_no_change", "CLICK>BACK", string(enginestate.ModeRecover)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		TraceSignature:   "CLICK>BACK",
		Mode:             string(enginestate.ModeRecover),
		Candidate: candidate.NewCandidate(
			&types.ActionCommand{Act: types.BACK},
			candidate.SourceMemory,
			"返回上一层",
			map[string]string{"memory_key": "page-a"},
		),
		Outcome:      OutcomeEscaped,
		EscapeScore:  0.8,
		SuccessCount: 3,
		FailCount:    1,
		LastUsedAt:   time.Now().UTC(),
		CreatedAt:    time.Now().UTC(),
	}
	if err := store.Append(record); err != nil {
		t.Fatalf("追加 record 失败: %v", err)
	}

	reloaded, err := NewStore(filePath)
	if err != nil {
		t.Fatalf("重载 store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = reloaded.Close()
	})

	items := reloaded.All()
	if len(items) != 1 {
		t.Fatalf("重载后记录数量错误: %d", len(items))
	}
	if items[0].PageSignature != "page-a" {
		t.Fatalf("重载记录 page signature 错误: %s", items[0].PageSignature)
	}
	if items[0].Candidate.Command == nil || items[0].Candidate.Command.Act != types.BACK {
		t.Fatalf("重载记录 candidate 命令错误: %+v", items[0].Candidate.Command)
	}
}

func TestStoreFindPrefersExactMatch(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "memory.jsonl"))
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	now := time.Now().UTC()
	exact := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "scroll_no_change", "CLICK>BACK", string(enginestate.ModeRecover)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		TraceSignature:   "CLICK>BACK",
		Mode:             string(enginestate.ModeRecover),
		Candidate: candidate.NewCandidate(
			&types.ActionCommand{Act: types.BACK},
			candidate.SourceMemory,
			"exact",
			nil,
		),
		Outcome:      OutcomeEscaped,
		SuccessCount: 2,
		CreatedAt:    now,
		LastUsedAt:   now,
	}
	fallback := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "scroll_no_change", "", string(enginestate.ModeRecover)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		TraceSignature:   "",
		Mode:             string(enginestate.ModeRecover),
		Candidate: candidate.NewCandidate(
			&types.ActionCommand{Act: types.CLICK},
			candidate.SourceMemory,
			"fallback",
			nil,
		),
		Outcome:      OutcomeEscaped,
		SuccessCount: 1,
		CreatedAt:    now,
		LastUsedAt:   now,
	}
	if err := store.Append(exact); err != nil {
		t.Fatalf("追加 exact 失败: %v", err)
	}
	if err := store.Append(fallback); err != nil {
		t.Fatalf("追加 fallback 失败: %v", err)
	}

	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode:             enginestate.ModeRecover,
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		RecentTrace: []enginestate.ActionTrace{
			{ActionKey: "CLICK"},
			{ActionKey: "BACK"},
		},
	})

	items := store.Find(ctx)
	if len(items) == 0 {
		t.Fatalf("预期命中至少一个记录")
	}
	if items[0].Candidate.Intent != "exact" {
		t.Fatalf("预期优先命中 exact，实际: %s", items[0].Candidate.Intent)
	}
}

func TestStoreAppendOutcomeAggregatesByMemoryKeyAndAction(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "recovery_memory.jsonl")
	store, err := NewStore(filePath)
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	now := time.Now().UTC()
	base := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "same_page_no_change", "BACK", string(enginestate.ModeRecover)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "same_page_no_change",
		TraceSignature:   "BACK",
		Mode:             string(enginestate.ModeRecover),
		Candidate: candidate.NewCandidate(
			&types.ActionCommand{Act: types.BACK},
			candidate.SourceMemory,
			"返回上一层",
			nil,
		),
		LastUsedAt: now,
		CreatedAt:  now,
	}
	success := base
	success.Outcome = OutcomeEscaped
	success.SuccessCount = 1
	success.EscapeScore = 1

	fail := base
	fail.Outcome = OutcomeFailed
	fail.FailCount = 1
	fail.LastUsedAt = now.Add(time.Second)

	if err := store.AppendOutcome(success); err != nil {
		t.Fatalf("写入 success 失败: %v", err)
	}
	if err := store.AppendOutcome(fail); err != nil {
		t.Fatalf("写入 fail 失败: %v", err)
	}

	all := store.All()
	if len(all) != 1 {
		t.Fatalf("聚合后应为 1 条记录，实际: %d", len(all))
	}
	if all[0].SuccessCount != 1 || all[0].FailCount != 1 {
		t.Fatalf("聚合计数错误: success=%d fail=%d", all[0].SuccessCount, all[0].FailCount)
	}
	if all[0].Outcome != OutcomeFailed {
		t.Fatalf("最新 outcome 应覆盖为 failed，实际: %s", all[0].Outcome)
	}
}
