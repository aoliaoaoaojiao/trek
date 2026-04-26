package memory

import (
	"strings"
	"sync"
	"trek/internal/engine/candidate"
	enginestate "trek/internal/engine/state"
)

// Store 管理 recovery memory 的内存索引与 jsonl 持久化。
type Store struct {
	path    string
	mu      sync.RWMutex
	records []RecoveryMemoryRecord
}

// NewStore 创建 Store，并从 jsonl 文件加载已有记录。
func NewStore(path string) (*Store, error) {
	records, err := loadRecordsFromJSONL(path)
	if err != nil {
		return nil, err
	}
	return &Store{
		path:    path,
		records: aggregateRecords(records),
	}, nil
}

// Append 追加一条记录并落盘 jsonl。
func (s *Store) Append(record RecoveryMemoryRecord) error {
	if s == nil {
		return nil
	}
	item := cloneRecord(record)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := appendRecordToJSONL(s.path, item); err != nil {
		return err
	}
	s.upsertRecordLocked(item)
	return nil
}

// AppendOutcome 追加一次恢复结果增量，并在内存中按 key 聚合统计。
func (s *Store) AppendOutcome(record RecoveryMemoryRecord) error {
	return s.Append(record)
}

// All 返回记录快照。
func (s *Store) All() []RecoveryMemoryRecord {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneRecords(s.records)
}

// Find 基于上下文执行分层匹配。
func (s *Store) Find(ctx enginestate.TraversalContext) []RecoveryMemoryRecord {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return findMatches(s.records, ctx)
}

func cloneRecords(src []RecoveryMemoryRecord) []RecoveryMemoryRecord {
	if len(src) == 0 {
		return nil
	}
	result := make([]RecoveryMemoryRecord, len(src))
	for i := range src {
		result[i] = cloneRecord(src[i])
	}
	return result
}

func cloneRecord(src RecoveryMemoryRecord) RecoveryMemoryRecord {
	item := src
	item.Candidate = cloneCandidate(src.Candidate)
	return item
}

func cloneCandidate(src candidate.Candidate) candidate.Candidate {
	dst := src
	if src.Command != nil {
		cmd := *src.Command
		dst.Command = &cmd
	}
	if len(src.Metadata) == 0 {
		dst.Metadata = map[string]string{}
		return dst
	}
	metadata := make(map[string]string, len(src.Metadata))
	for key, value := range src.Metadata {
		metadata[key] = value
	}
	dst.Metadata = metadata
	return dst
}

func aggregateRecords(records []RecoveryMemoryRecord) []RecoveryMemoryRecord {
	if len(records) == 0 {
		return nil
	}
	result := make([]RecoveryMemoryRecord, 0, len(records))
	index := make(map[string]int, len(records))
	for _, item := range records {
		key := recordAggregateKey(item)
		if pos, ok := index[key]; ok {
			result[pos] = mergeRecord(result[pos], item)
			continue
		}
		index[key] = len(result)
		result = append(result, cloneRecord(item))
	}
	return result
}

func (s *Store) upsertRecordLocked(item RecoveryMemoryRecord) {
	key := recordAggregateKey(item)
	for i := range s.records {
		if recordAggregateKey(s.records[i]) == key {
			s.records[i] = mergeRecord(s.records[i], item)
			return
		}
	}
	s.records = append(s.records, cloneRecord(item))
}

func mergeRecord(base RecoveryMemoryRecord, delta RecoveryMemoryRecord) RecoveryMemoryRecord {
	result := cloneRecord(base)
	if result.SuccessCount < 0 {
		result.SuccessCount = 0
	}
	if result.FailCount < 0 {
		result.FailCount = 0
	}
	if delta.SuccessCount > 0 {
		result.SuccessCount += delta.SuccessCount
	}
	if delta.FailCount > 0 {
		result.FailCount += delta.FailCount
	}
	if strings.TrimSpace(delta.Outcome) != "" {
		result.Outcome = delta.Outcome
	}
	if delta.EscapeScore > result.EscapeScore {
		result.EscapeScore = delta.EscapeScore
	}
	if !delta.LastUsedAt.IsZero() {
		result.LastUsedAt = delta.LastUsedAt
	}
	if result.CreatedAt.IsZero() || (!delta.CreatedAt.IsZero() && delta.CreatedAt.Before(result.CreatedAt)) {
		result.CreatedAt = delta.CreatedAt
	}
	if delta.Candidate.Command != nil {
		result.Candidate = cloneCandidate(delta.Candidate)
	}
	return result
}

func recordAggregateKey(item RecoveryMemoryRecord) string {
	act := ""
	if item.Candidate.Command != nil {
		act = item.Candidate.Command.ToJSON()
	}
	return strings.TrimSpace(item.MemoryKey) + "|" + act
}
