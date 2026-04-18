package vision

import (
	"context"
	"fmt"

	"trek/internal/engine/decision"
)

// Perceptor 是图像感知占位实现，当前仅做输入通道校验。
type Perceptor struct{}

func NewPerceptor() *Perceptor {
	return &Perceptor{}
}

func (p *Perceptor) Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error) {
	_ = ctx
	if len(input.Screenshot) == 0 {
		return nil, fmt.Errorf("vision perception requires screenshot")
	}
	return &decision.Observation{
		PageName:   input.PageName,
		XMLDesc:    input.XMLDesc,
		Screenshot: input.Screenshot,
		Element:    nil,
	}, nil
}
