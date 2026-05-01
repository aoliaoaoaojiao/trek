package recovery

import (
	"sync"
	enginestate "trek/internal/engine/state"
)

// SlidingWindowLLMBudget 基于步数窗口限制 LLM 调用次数。
type SlidingWindowLLMBudget struct {
	maxCalls int
	window   int

	mu    sync.Mutex
	steps []int
}

// NewSlidingWindowLLMBudget 创建窗口预算。
// maxCalls <= 0 时视为不限额。
// window <= 0 时按全局 maxCalls 限额，不做窗口裁剪。
func NewSlidingWindowLLMBudget(maxCalls int, window int) *SlidingWindowLLMBudget {
	return &SlidingWindowLLMBudget{
		maxCalls: maxCalls,
		window:   window,
		steps:    make([]int, 0, maxCalls),
	}
}

// Allow 判断当前上下文是否允许继续调用 LLM。
func (b *SlidingWindowLLMBudget) Allow(ctx enginestate.TraversalContext) bool {
	if b == nil || b.maxCalls <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(ctx.Step)
	return len(b.steps) < b.maxCalls
}

// Record 记录一次 LLM 调用。
func (b *SlidingWindowLLMBudget) Record(ctx enginestate.TraversalContext) {
	if b == nil || b.maxCalls <= 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(ctx.Step)
	b.steps = append(b.steps, ctx.Step)
}

func (b *SlidingWindowLLMBudget) pruneLocked(step int) {
	if b.window <= 0 || len(b.steps) == 0 {
		return
	}
	cutoff := step - b.window
	keepFrom := 0
	for keepFrom < len(b.steps) && b.steps[keepFrom] <= cutoff {
		keepFrom++
	}
	if keepFrom > 0 {
		n := copy(b.steps, b.steps[keepFrom:])
		b.steps = b.steps[:n]
	}
}
