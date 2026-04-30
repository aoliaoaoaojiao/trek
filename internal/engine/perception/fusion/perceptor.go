package fusion

import (
	"context"
	"fmt"

	"trek/internal/engine/decision"
)

// Mode 表示感知融合模式。
type Mode string

const (
	ModeXMLOnly   Mode = "xml-only"
	ModeImageOnly Mode = "image-only"
	ModeHybrid    Mode = "hybrid"
)

type Perceptor struct {
	mode   Mode
	xml    decision.Perceptor
	vision decision.Perceptor
}

func NewPerceptor(mode Mode, xmlPerceptor decision.Perceptor, visionPerceptor decision.Perceptor) (*Perceptor, error) {
	if xmlPerceptor == nil {
		return nil, fmt.Errorf("xml perceptor is required")
	}
	if visionPerceptor == nil {
		return nil, fmt.Errorf("vision perceptor is required")
	}
	if mode == "" {
		mode = ModeXMLOnly
	}
	if mode != ModeXMLOnly && mode != ModeImageOnly && mode != ModeHybrid {
		return nil, fmt.Errorf("unsupported perception mode: %s", mode)
	}
	return &Perceptor{mode: mode, xml: xmlPerceptor, vision: visionPerceptor}, nil
}

func ParseMode(mode string) (Mode, error) {
	m := Mode(mode)
	if m == "" {
		return ModeXMLOnly, nil
	}
	if m != ModeXMLOnly && m != ModeImageOnly && m != ModeHybrid {
		return "", fmt.Errorf("unsupported perception mode: %s", mode)
	}
	return m, nil
}

func (p *Perceptor) Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error) {
	switch p.mode {
	case ModeXMLOnly:
		return p.xml.Observe(ctx, input)
	case ModeImageOnly:
		return p.vision.Observe(ctx, input)
	case ModeHybrid:
		obs, err := p.xml.Observe(ctx, input)
		if err == nil && obs != nil {
			return obs, nil
		}
		return p.vision.Observe(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported perception mode: %s", p.mode)
	}
}
