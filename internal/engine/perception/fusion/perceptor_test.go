package fusion

import (
	"context"
	"errors"
	"testing"

	"trek/internal/engine/decision"
)

type stubPerceptor struct {
	called bool
	obs    *decision.Observation
	err    error
}

func (s *stubPerceptor) Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error) {
	_ = ctx
	_ = input
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	return s.obs, nil
}

func TestFusionXMLOnlyUsesXMLPerceptor(t *testing.T) {
	xmlP := &stubPerceptor{obs: &decision.Observation{PageName: "Main"}}
	visionP := &stubPerceptor{obs: &decision.Observation{PageName: "Vision"}}
	p, err := NewPerceptor(ModeXMLOnly, xmlP, visionP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obs, err := p.Observe(context.Background(), decision.PerceptionInput{PageName: "Main", XMLDesc: "<x/>"})
	if err != nil {
		t.Fatalf("unexpected observe error: %v", err)
	}
	if obs == nil || obs.PageName != "Main" {
		t.Fatalf("unexpected observation: %+v", obs)
	}
	if !xmlP.called || visionP.called {
		t.Fatalf("xml should be called, vision should not")
	}
}

func TestFusionImageOnlyUsesVisionPerceptor(t *testing.T) {
	xmlP := &stubPerceptor{obs: &decision.Observation{PageName: "Main"}}
	visionP := &stubPerceptor{obs: &decision.Observation{PageName: "Vision"}}
	p, err := NewPerceptor(ModeImageOnly, xmlP, visionP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obs, err := p.Observe(context.Background(), decision.PerceptionInput{PageName: "Main", Screenshot: []byte{1, 2}})
	if err != nil {
		t.Fatalf("unexpected observe error: %v", err)
	}
	if obs == nil || obs.PageName != "Vision" {
		t.Fatalf("unexpected observation: %+v", obs)
	}
	if xmlP.called || !visionP.called {
		t.Fatalf("vision should be called, xml should not")
	}
}

func TestFusionHybridFallsBackToVision(t *testing.T) {
	xmlP := &stubPerceptor{err: errors.New("xml failed")}
	visionP := &stubPerceptor{obs: &decision.Observation{PageName: "Vision"}}
	p, err := NewPerceptor(ModeHybrid, xmlP, visionP)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obs, err := p.Observe(context.Background(), decision.PerceptionInput{PageName: "Main", XMLDesc: "bad", Screenshot: []byte{1}})
	if err != nil {
		t.Fatalf("unexpected observe error: %v", err)
	}
	if obs == nil || obs.PageName != "Vision" {
		t.Fatalf("unexpected observation: %+v", obs)
	}
	if !xmlP.called || !visionP.called {
		t.Fatalf("hybrid should try xml and then vision")
	}
}
