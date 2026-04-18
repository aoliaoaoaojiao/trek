package runtime

import (
	"context"
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
)

// Actuator 负责把执行计划编译成设备操作。
type Actuator interface {
	Compile(ctx context.Context, obs *decision.Observation, plan *decision.ExecutionPlan) (*types.DeviceOperateWrapper, error)
}
