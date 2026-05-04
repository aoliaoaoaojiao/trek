package memory

import (
	"fmt"
	"time"
	"trek/internal/engine/perception"
)

const (
	OutcomeEscaped                  = "escaped"
	OutcomeFailed                   = "failed"
	BlockReasonCandidateEnhancement = "candidate_enhancement"
)

// RecoveryMemoryRecord 是恢复经验的最小持久化记录。
type RecoveryMemoryRecord struct {
	MemoryKey        string               `json:"memory_key"`
	PageSignature    string               `json:"page_signature"`
	ClusterSignature string               `json:"cluster_signature"`
	BlockReason      string               `json:"block_reason"`
	TraceSignature   string               `json:"trace_signature"`
	Mode             string               `json:"mode"`
	Item             perception.Candidate `json:"item"`
	Outcome          string               `json:"outcome"`
	EscapeScore      float64              `json:"escape_score"`
	SuccessCount     int                  `json:"success_count"`
	FailCount        int                  `json:"fail_count"`
	LastUsedAt       time.Time            `json:"last_used_at"`
	CreatedAt        time.Time            `json:"created_at"`
}

// BuildMemoryKey 构建恢复经验主键。
func BuildMemoryKey(pageSignature string, blockReason string, traceSignature string, mode string) string {
	return fmt.Sprintf("%s|%s|%s|%s", pageSignature, blockReason, traceSignature, mode)
}
