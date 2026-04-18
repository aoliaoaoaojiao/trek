package runtime

import (
	"testing"

	oldmodel "trek/internal/engine/core/model"
	oldelements "trek/internal/engine/core/types/elements"
	"trek/internal/engine/decision"
	xmlperception "trek/internal/engine/perception/xml"
)

func TestLegacyAndNewPathsAreBothUsable(t *testing.T) {
	if oldmodel.NewModel("pkg-old") == nil {
		t.Fatalf("legacy core/model path should still be usable")
	}
	if decision.NewModel("pkg-new") == nil {
		t.Fatalf("new decision path should be usable")
	}

	if oldelements.NewAndroidElement() == nil {
		t.Fatalf("legacy elements path should still be usable")
	}
	if xmlperception.NewAndroidElement() == nil {
		t.Fatalf("new perception/xml path should be usable")
	}
}
