package runtime

import (
	"testing"

	"trek/internal/engine/decision"
	reusepolicy "trek/internal/engine/decision/reuse"
	xmlperception "trek/internal/engine/perception/xml"
)

func TestMigrationPackagesExposeCoreSymbols(t *testing.T) {
	if decision.NewModel("pkg") == nil {
		t.Fatalf("decision.NewModel should not return nil")
	}

	if decision.NewGraph() == nil {
		t.Fatalf("decision.NewGraph should not return nil")
	}

	if xmlperception.NewAndroidElement() == nil {
		t.Fatalf("xml perception should create element")
	}

	if reusepolicy.SarsaNStep <= 0 {
		t.Fatalf("invalid reuse policy constant")
	}

	stats := reusepolicy.PageVisitCount{"PageA": 1}
	if stats["PageA"] != 1 {
		t.Fatalf("reuse policy exported alias should be usable")
	}
}
