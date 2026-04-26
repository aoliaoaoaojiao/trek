package candidate

import (
	"testing"
	"trek/internal/engine/decision/shared/types"
)

func TestNewCandidateClonesMetadata(t *testing.T) {
	metadata := map[string]string{
		"memory_key": "page-a#recover",
	}
	cmd := &types.ActionCommand{Act: types.BACK}

	item := NewCandidate(cmd, SourceMemory, "返回上一层", metadata)
	if item.Source != SourceMemory {
		t.Fatalf("source 不符合预期: %s", item.Source)
	}
	if item.Intent != "返回上一层" {
		t.Fatalf("intent 不符合预期: %s", item.Intent)
	}
	if item.Metadata["memory_key"] != "page-a#recover" {
		t.Fatalf("metadata 未写入")
	}

	metadata["memory_key"] = "mutated"
	if item.Metadata["memory_key"] != "page-a#recover" {
		t.Fatalf("metadata 应为快照副本，实际: %s", item.Metadata["memory_key"])
	}
}
