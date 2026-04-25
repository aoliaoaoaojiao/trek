package runtime

import (
	"testing"
	oldelements "trek/internal/engine/decision/shared/elements"

	"trek/internal/engine/decision"
	xmlperception "trek/internal/engine/perception/xml"
)

func TestDecisionAndPerceptionPathsAreUsable(t *testing.T) {
	if decision.NewModel("pkg-new") == nil {
		t.Fatalf("decision model path should be usable")
	}

	if oldelements.NewAndroidElement() == nil {
		t.Fatalf("legacy elements path should still be usable")
	}
	if xmlperception.NewAndroidElement() == nil {
		t.Fatalf("new perception/xml path should be usable")
	}
}
