package runtime

import (
	"context"
	"trek/internal/engine/decision"
	"trek/internal/engine/decision/shared/types"
)

// Actuator 璐熻矗鎶婃墽琛岃鍒掔紪璇戞垚璁惧鎿嶄綔銆?
type Actuator interface {
	Compile(ctx context.Context, obs *decision.Observation, plan *decision.ExecutionPlan) (*types.ActionCommand, error)
}
