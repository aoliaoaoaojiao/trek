package monkey

import enginestate "trek/internal/engine/state"

// TraversalMode 是 Runner 内部使用的遍历模式别名。
type TraversalMode = enginestate.Mode

const (
	TraversalModeExplore        = enginestate.ModeExplore
	TraversalModeSuspectBlocked = enginestate.ModeSuspectBlocked
	TraversalModeRecover        = enginestate.ModeRecover
	TraversalModeCooldown       = enginestate.ModeCooldown
)

// recoveryStateMachine 维护第一阶段显式恢复模式切换。
type recoveryStateMachine struct {
	mode              TraversalMode
	blockReason       string
	cooldownSteps     int
	cooldownRemaining int
}

func newRecoveryStateMachine() *recoveryStateMachine {
	return newRecoveryStateMachineWithCooldown(2)
}

func newRecoveryStateMachineWithCooldown(cooldownSteps int) *recoveryStateMachine {
	if cooldownSteps < 0 {
		cooldownSteps = 0
	}
	return &recoveryStateMachine{
		mode:          TraversalModeExplore,
		cooldownSteps: cooldownSteps,
	}
}

func (m *recoveryStateMachine) Mode() TraversalMode {
	if m == nil {
		return TraversalModeExplore
	}
	return m.mode
}

func (m *recoveryStateMachine) BlockReason() string {
	if m == nil {
		return ""
	}
	return m.blockReason
}

func (m *recoveryStateMachine) OnBlockDetected(reason string) {
	if m == nil {
		return
	}
	m.blockReason = reason
	switch m.mode {
	case TraversalModeExplore:
		m.mode = TraversalModeSuspectBlocked
	case TraversalModeSuspectBlocked:
		m.mode = TraversalModeRecover
	case TraversalModeCooldown:
		m.mode = TraversalModeSuspectBlocked
		m.cooldownRemaining = 0
	}
}

func (m *recoveryStateMachine) OnProgress(escaped bool) {
	if m == nil || !escaped {
		return
	}
	m.blockReason = ""
	if m.cooldownSteps > 0 {
		m.mode = TraversalModeCooldown
		m.cooldownRemaining = m.cooldownSteps
		return
	}
	m.mode = TraversalModeExplore
}

func (m *recoveryStateMachine) OnStepAdvance() {
	if m == nil || m.mode != TraversalModeCooldown {
		return
	}
	if m.cooldownRemaining > 0 {
		m.cooldownRemaining--
	}
	if m.cooldownRemaining <= 0 {
		m.mode = TraversalModeExplore
	}
}
