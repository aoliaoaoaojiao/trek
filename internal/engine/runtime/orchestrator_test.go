package runtime

import (
	"context"
	"errors"
	"testing"
	types2 "trek/internal/engine/decision/shared/types"

	"trek/internal/engine/decision"
)

type fakePerceptor struct {
	called bool
	obs    *decision.Observation
	err    error
}

func (f *fakePerceptor) Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error) {
	f.called = true
	_ = input
	if f.err != nil {
		return nil, f.err
	}
	return f.obs, nil
}

type fakePolicy struct {
	called     bool
	candidates []decision.CandidateAction
	err        error
}

func (f *fakePolicy) Name() string { return "fake" }

func (f *fakePolicy) Propose(ctx context.Context, obs *decision.Observation) ([]decision.CandidateAction, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.candidates, nil
}

type fakePlanner struct {
	called bool
	plan   *decision.ExecutionPlan
	err    error
}

func (f *fakePlanner) Select(ctx context.Context, obs *decision.Observation, candidates []decision.CandidateAction) (*decision.ExecutionPlan, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.plan, nil
}

type fakeActuator struct {
	called bool
	op     *types2.ActionCommand
	err    error
}

func (f *fakeActuator) Compile(ctx context.Context, obs *decision.Observation, plan *decision.ExecutionPlan) (*types2.ActionCommand, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return f.op, nil
}

func TestOrchestratorPipelineSuccess(t *testing.T) {
	obs := &decision.Observation{PageName: "Main"}
	op := types2.NewActionCommand()
	op.Act = types2.BACK

	orch := &Orchestrator{
		perceptor: &fakePerceptor{obs: obs},
		policy:    &fakePolicy{candidates: []decision.CandidateAction{{Operate: op, Source: "fake"}}},
		planner:   &fakePlanner{plan: &decision.ExecutionPlan{Operate: op, Strategy: "first"}},
		actuator:  &fakeActuator{op: op},
	}

	result := orch.NextAction(context.Background(), "Main", "<hierarchy/>")
	if result == nil {
		t.Fatalf("expected non nil result")
	}
	if result.Act != types2.BACK {
		t.Fatalf("unexpected action: %v", result.Act)
	}
}

func TestOrchestratorStopsOnPerceptionError(t *testing.T) {
	perceptor := &fakePerceptor{err: errors.New("perception failed")}
	policy := &fakePolicy{}
	planner := &fakePlanner{}
	actuator := &fakeActuator{}

	orch := &Orchestrator{
		perceptor: perceptor,
		policy:    policy,
		planner:   planner,
		actuator:  actuator,
	}

	result := orch.NextAction(context.Background(), "Main", "bad xml")
	if result != nil {
		t.Fatalf("expected nil result when perception fails")
	}
	if policy.called || planner.called || actuator.called {
		t.Fatalf("downstream components should not be called")
	}
}
