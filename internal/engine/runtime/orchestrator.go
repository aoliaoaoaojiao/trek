package runtime

import (
	"context"
	"fmt"

	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	"trek/internal/engine/perception"
	perceptionfusion "trek/internal/engine/perception/fusion"
	perceptionvision "trek/internal/engine/perception/vision"
	xmlperception "trek/internal/engine/perception/xml"
)

// Orchestrator 负责编排感知、策略、规划和执行流程。
type Orchestrator struct {
	perceptor perception.Perceptor
	policy    decision.Policy
	planner   decision.Planner
	actuator  Actuator
}

func (o *Orchestrator) NextAction(ctx context.Context, pageName string, xmlDescOfGuiTree string) *types.DeviceOperateWrapper {
	return o.NextActionWithInput(ctx, decision.PerceptionInput{
		PageName: pageName,
		XMLDesc:  xmlDescOfGuiTree,
	})
}

func (o *Orchestrator) NextActionWithInput(ctx context.Context, input decision.PerceptionInput) *types.DeviceOperateWrapper {
	if o == nil || o.perceptor == nil || o.policy == nil || o.planner == nil || o.actuator == nil {
		return nil
	}

	obs, err := o.perceptor.Observe(ctx, input)
	if err != nil || obs == nil {
		return nil
	}

	candidates, err := o.policy.Propose(ctx, obs)
	if err != nil || len(candidates) == 0 {
		return nil
	}

	plan, err := o.planner.Select(ctx, obs, candidates)
	if err != nil || plan == nil {
		return nil
	}

	operate, err := o.actuator.Compile(ctx, obs, plan)
	if err != nil || operate == nil {
		return nil
	}

	return operate
}

type xmlObservationPerceptor struct{}

func (p *xmlObservationPerceptor) Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error) {
	_ = ctx
	elem, err := xmlperception.CreateAndroidElementFromXml(input.XMLDesc)
	if err != nil {
		return nil, err
	}
	return &decision.Observation{
		PageName:   input.PageName,
		XMLDesc:    input.XMLDesc,
		Screenshot: input.Screenshot,
		Element:    elem,
	}, nil
}

type legacyModelPolicy struct {
	modelProvider func(pageName string) *decision.Model
}

func (p *legacyModelPolicy) Name() string { return "legacy-model-policy" }

func (p *legacyModelPolicy) Propose(ctx context.Context, obs *decision.Observation) ([]decision.CandidateAction, error) {
	_ = ctx
	if p == nil || p.modelProvider == nil || obs == nil || obs.Element == nil {
		return nil, nil
	}
	m := p.modelProvider(obs.PageName)
	if m == nil {
		return nil, nil
	}

	op := m.GetOperateOpt(obs.Element, obs.PageName, "")
	if op == nil {
		return nil, nil
	}

	return []decision.CandidateAction{{
		Operate: op,
		Source:  p.Name(),
	}}, nil
}

type firstCandidatePlanner struct{}

func (p *firstCandidatePlanner) Select(ctx context.Context, obs *decision.Observation, candidates []decision.CandidateAction) (*decision.ExecutionPlan, error) {
	_ = ctx
	_ = obs
	if len(candidates) == 0 {
		return nil, nil
	}
	return &decision.ExecutionPlan{
		Operate:  candidates[0].Operate,
		Strategy: "first-candidate",
	}, nil
}

type passthroughActuator struct{}

func (a *passthroughActuator) Compile(ctx context.Context, obs *decision.Observation, plan *decision.ExecutionPlan) (*types.DeviceOperateWrapper, error) {
	_ = ctx
	_ = obs
	if plan == nil || plan.Operate == nil {
		return nil, nil
	}
	return plan.Operate, nil
}

func newDefaultOrchestrator() *Orchestrator {
	return newOrchestratorWithMode(observationMode)
}

func newOrchestratorWithMode(mode perceptionfusion.Mode) *Orchestrator {
	fusionPerceptor, err := perceptionfusion.NewPerceptor(mode, &xmlObservationPerceptor{}, perceptionvision.NewPerceptor())
	if err != nil {
		// 容错回退：异常模式默认降级到 XML-only，避免中断现有流程。
		fusionPerceptor, err = perceptionfusion.NewPerceptor(perceptionfusion.ModeXMLOnly, &xmlObservationPerceptor{}, perceptionvision.NewPerceptor())
		if err != nil {
			panic(fmt.Errorf("failed to init default perceptor: %w", err))
		}
	}

	return &Orchestrator{
		perceptor: fusionPerceptor,
		policy: &legacyModelPolicy{
			modelProvider: ensureModel,
		},
		planner:  &firstCandidatePlanner{},
		actuator: &passthroughActuator{},
	}
}
