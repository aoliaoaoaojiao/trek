package perception

import (
	"context"
	"trek/internal/engine/decision"
)

// Perceptor 负责把原始输入转换为统一 Observation。
type Perceptor interface {
	Observe(ctx context.Context, input decision.PerceptionInput) (*decision.Observation, error)
}
