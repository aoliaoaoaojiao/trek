package memory

import (
	"path/filepath"
	"testing"
	"time"
	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"
)

func TestProviderBuildCandidatesFromStore(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "memory.jsonl"))
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	now := time.Now().UTC()
	record := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "scroll_no_change", "CLICK>BACK", string(enginestate.ModeRecover)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "scroll_no_change",
		TraceSignature:   "CLICK>BACK",
		Mode:             string(enginestate.ModeRecover),
		Item: perception.NewCandidate(
			&types.ActionCommand{Act: types.BACK},
			perception.SourceMemory,
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
		Mode:             enginestate.ModeRecover,
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
	if items[0].Source != perception.SourceMemory {
		t.Fatalf("候选来源错误: %s", items[0].Source)
	}
	if items[0].Command == nil || items[0].Command.Act != types.BACK {
		t.Fatalf("候选命令错误: %+v", items[0].Command)
	}
	if items[0].Confidence <= 0 {
		t.Fatalf("候选 confidence 应大于 0，实际: %.3f", items[0].Confidence)
	}
}

func TestProviderBoostsCandidateEnhancementInExplore(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "memory.jsonl"))
	if err != nil {
		t.Fatalf("创建 store 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	now := time.Now().UTC()

	enhanced := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", BlockReasonCandidateEnhancement, "CLICK", string(enginestate.ModeExplore)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      BlockReasonCandidateEnhancement,
		TraceSignature:   "CLICK",
		Mode:             string(enginestate.ModeExplore),
		Item:             perception.NewCandidate(&types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.1, 0.1, 0.2, 0.2)}, perception.SourceMemory, "增强点击", nil),
		Outcome:          OutcomeEscaped,
		SuccessCount:     1,
		FailCount:        1,
		LastUsedAt:       now,
		CreatedAt:        now,
	}
	normal := RecoveryMemoryRecord{
		MemoryKey:        BuildMemoryKey("page-a", "same_page_no_change", "BACK", string(enginestate.ModeExplore)),
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
		BlockReason:      "same_page_no_change",
		TraceSignature:   "BACK",
		Mode:             string(enginestate.ModeExplore),
		Item:             perception.NewCandidate(&types.ActionCommand{Act: types.BACK}, perception.SourceMemory, "普通返回", nil),
		Outcome:          OutcomeEscaped,
		SuccessCount:     1,
		FailCount:        1,
		LastUsedAt:       now,
		CreatedAt:        now,
	}
	if err := store.Append(enhanced); err != nil {
		t.Fatalf("写入增强记录失败: %v", err)
	}
	if err := store.Append(normal); err != nil {
		t.Fatalf("写入普通记录失败: %v", err)
	}

	provider := NewProvider(store)
	items, err := provider.BuildCandidates(enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode:             enginestate.ModeExplore,
		PageSignature:    "page-a",
		ClusterSignature: "cluster-a",
	}))
	if err != nil {
		t.Fatalf("构建候选失败: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("候选数量错误: %d", len(items))
	}

	confByAction := map[types.ActionType]float64{}
	for _, item := range items {
		if item.Command == nil {
			continue
		}
		confByAction[item.Command.Act] = item.Confidence
	}
	if confByAction[types.CLICK] <= confByAction[types.BACK] {
		t.Fatalf("Explore 下增强样本应获得更高置信度: click=%.3f back=%.3f", confByAction[types.CLICK], confByAction[types.BACK])
	}
}

func TestProviderDoesNotBoostCandidateEnhancementInRecover(t *testing.T) {
	record := RecoveryMemoryRecord{
		BlockReason:  BlockReasonCandidateEnhancement,
		SuccessCount: 1,
		FailCount:    1,
	}
	ctx := enginestate.BuildTraversalContext(enginestate.BuildInput{
		Mode: enginestate.ModeRecover,
	})
	base := successRate(record)
	got := providerConfidence(record, ctx)
	if got != base {
		t.Fatalf("Recover 下不应加权: got=%.3f want=%.3f", got, base)
	}
}
