package traversal

import (
	"math"
	"strings"
	"sync"

	"trek/internal/engine/decision/shared/types"
	"trek/internal/engine/perception"
)

const (
	outcomeFeedbackScale = 0.3
	outcomeRiskScale     = 0.6
)

// outcomeFeedbackStore 维护动作级在线反馈分，用于将执行结果回写到候选排序。
type outcomeFeedbackStore struct {
	mu     sync.RWMutex
	scores map[string]float64
}

func newOutcomeFeedbackStore() *outcomeFeedbackStore {
	return &outcomeFeedbackStore{
		scores: make(map[string]float64),
	}
}

func (s *outcomeFeedbackStore) observe(action *types.ActionCommand, outcome ActionOutcome) {
	if s == nil || action == nil {
		return
	}
	key := strings.TrimSpace(action.ToJSON())
	if key == "" {
		return
	}
	delta := outcomeDelta(outcome)
	if delta == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	score := s.scores[key] + delta
	if score > 1 {
		score = 1
	}
	if score < -1 {
		score = -1
	}
	s.scores[key] = score
}

func (s *outcomeFeedbackStore) scoreFor(action *types.ActionCommand) float64 {
	if s == nil || action == nil {
		return 0
	}
	key := strings.TrimSpace(action.ToJSON())
	if key == "" {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scores[key]
}

func outcomeDelta(outcome ActionOutcome) float64 {
	switch outcome {
	case OutcomeNewState, OutcomeEscapeBlock:
		return 0.35
	case OutcomeSameState:
		return -0.15
	case OutcomeLoop:
		return -0.4
	case OutcomeNoOp:
		return -0.2
	default:
		return 0
	}
}

func applyOutcomeFeedback(item perception.Candidate, feedback *outcomeFeedbackStore) perception.Candidate {
	if item.Command == nil || feedback == nil {
		return item
	}
	score := feedback.scoreFor(item.Command)
	if score == 0 {
		return item
	}
	item.Confidence = clamp01(item.Confidence + score*outcomeFeedbackScale)
	if score < 0 {
		item.RiskScore += math.Abs(score) * outcomeRiskScale
	}
	if item.Metadata == nil {
		item.Metadata = map[string]string{}
	}
	item.Metadata["outcome_feedback"] = item.Command.Act.String()
	return item
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
