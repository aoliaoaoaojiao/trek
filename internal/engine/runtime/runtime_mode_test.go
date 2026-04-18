package runtime

import "testing"

func TestSetObservationModeRoundTrip(t *testing.T) {
	ResetModel()
	if err := SetObservationMode("hybrid"); err != nil {
		t.Fatalf("expected set mode success, got err=%v", err)
	}
	if got := GetObservationMode(); got != "hybrid" {
		t.Fatalf("unexpected mode: %s", got)
	}

	if err := SetObservationMode("xml-only"); err != nil {
		t.Fatalf("expected xml-only success, got err=%v", err)
	}
	if got := GetObservationMode(); got != "xml-only" {
		t.Fatalf("unexpected mode: %s", got)
	}
}

func TestSetObservationModeRejectsInvalid(t *testing.T) {
	ResetModel()
	if err := SetObservationMode("invalid-mode"); err == nil {
		t.Fatalf("expected invalid mode error")
	}
}
