package monkey

import enginestate "trek/internal/engine/state"

// TraversalMode 是 Runner 内部使用的遍历模式别名。
type TraversalMode = enginestate.Mode

const (
	TraversalModeExplore        = enginestate.ModeExplore
	TraversalModeSuspectBlocked = enginestate.ModeSuspectBlocked
	TraversalModeRecover        = enginestate.ModeRecover
	TraversalModeCooldown       = enginestate.ModeCooldown

	defaultCooldownMaxSteps = 10 // 冷却时间上限，避免退避无限增长
)

// recoveryStateMachine 维护第一阶段显式恢复模式切换。
type recoveryStateMachine struct {
	mode                 TraversalMode
	blockReason          string
	cooldownSteps        int
	cooldownRemaining    int
	recoveryAttempts     int // 当前恢复周期内的尝试次数
	maxRecoveryAttempts  int // 恢复周期内最大尝试次数（0=不限制）
	cooldownMultiplier   int // 冷却退避倍数，每次 Cooldown→再次阻塞 +1，成功逃离清零
}

func newRecoveryStateMachine() *recoveryStateMachine {
	return newRecoveryStateMachineWithCooldown(2)
}

func newRecoveryStateMachineWithCooldown(cooldownSteps int) *recoveryStateMachine {
	if cooldownSteps < 0 {
		cooldownSteps = 0
	}
	return &recoveryStateMachine{
		mode:               TraversalModeExplore,
		cooldownSteps:      cooldownSteps,
		cooldownMultiplier: 1,
	}
}

// SetMaxRecoveryAttempts 设置恢复周期内最大尝试次数。
func (m *recoveryStateMachine) SetMaxRecoveryAttempts(maxAttempts int) {
	if m == nil {
		return
	}
	m.maxRecoveryAttempts = maxAttempts
}

// RecoveryAttempts 返回当前恢复周期内的尝试次数。
func (m *recoveryStateMachine) RecoveryAttempts() int {
	if m == nil {
		return 0
	}
	return m.recoveryAttempts
}

// IsRecoveryBudgetExhausted 检查恢复预算是否已耗尽。
func (m *recoveryStateMachine) IsRecoveryBudgetExhausted() bool {
	if m == nil || m.maxRecoveryAttempts <= 0 {
		return false
	}
	return m.recoveryAttempts >= m.maxRecoveryAttempts
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
		m.recoveryAttempts = 0
	case TraversalModeRecover:
		// 恢复失败但预算未耗尽：保持 Recover 状态继续尝试恢复。
		// 这符合设计规格的 Recover→Recover 自循环。
		m.recoveryAttempts++
		if m.IsRecoveryBudgetExhausted() {
			// 预算耗尽：强制进入冷却模式
			m.mode = TraversalModeCooldown
			m.cooldownRemaining = m.cooldownSteps * m.cooldownMultiplier
			if m.cooldownRemaining > defaultCooldownMaxSteps {
				m.cooldownRemaining = defaultCooldownMaxSteps
			}
			m.recoveryAttempts = 0
		}
	case TraversalModeCooldown:
		m.mode = TraversalModeSuspectBlocked
		m.cooldownRemaining = 0
		m.cooldownMultiplier++ // 退避：下次冷却时间更长
	}
}

func (m *recoveryStateMachine) OnProgress(escaped bool) {
	if m == nil || !escaped {
		return
	}
	m.blockReason = ""
	m.recoveryAttempts = 0
	m.cooldownMultiplier = 1 // 成功逃离，重置退避
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
