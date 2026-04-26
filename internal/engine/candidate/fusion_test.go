package candidate

import (
	"testing"
	"trek/internal/engine/decision/shared/types"
)

func TestFuseCandidatesDeduplicatesAndMergesSources(t *testing.T) {
	cmd := &types.ActionCommand{Act: types.BACK}
	items := []Candidate{
		{
			Command:    cmd,
			Source:     SourceMemory,
			Intent:     "返回",
			Confidence: 0.8,
			Metadata:   map[string]string{"memory_key": "m1"},
		},
		{
			Command:     cmd.Clone(),
			Source:      SourceLLM,
			Confidence:  0.9,
			EscapeScore: 0.7,
			Metadata:    map[string]string{"llm_reason": "close popup"},
		},
	}

	result := FuseCandidates(items, FusionOptions{})
	if len(result) != 1 {
		t.Fatalf("预期融合后仅 1 条候选，实际: %d", len(result))
	}
	item := result[0]
	if item.Source != SourceMemory+"|"+SourceLLM {
		t.Fatalf("候选来源合并错误: %s", item.Source)
	}
	if item.Confidence != 0.9 {
		t.Fatalf("候选置信度应取最大值，实际: %f", item.Confidence)
	}
	if item.EscapeScore != 0.7 {
		t.Fatalf("候选逃逸分应取最大值，实际: %f", item.EscapeScore)
	}
	if item.Metadata["memory_key"] != "m1" || item.Metadata["llm_reason"] != "close popup" {
		t.Fatalf("候选元数据未合并: %+v", item.Metadata)
	}
}

func TestFuseCandidatesAppliesKnownFailedRiskPenalty(t *testing.T) {
	back := Candidate{
		Command:    &types.ActionCommand{Act: types.BACK},
		Source:     SourceMemory,
		Confidence: 0.8,
	}
	click := Candidate{
		Command:     &types.ActionCommand{Act: types.CLICK, Pos: *types.NewRect(0.2, 0.2, 0.3, 0.3)},
		Source:      SourceHeuristic,
		Confidence:  0.7,
		EscapeScore: 0.6,
	}

	result := FuseCandidates([]Candidate{back, click}, FusionOptions{
		KnownFailedActions: map[string]bool{
			back.Command.ToJSON(): true,
		},
	})
	if len(result) != 2 {
		t.Fatalf("预期融合后保留 2 条候选，实际: %d", len(result))
	}
	if result[0].Command == nil || result[0].Command.Act != types.CLICK {
		t.Fatalf("已知失败动作应被风险惩罚后降序，实际首选: %+v", result[0].Command)
	}
}
