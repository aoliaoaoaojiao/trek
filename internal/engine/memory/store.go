package memory

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"trek/internal/engine/perception"
	enginestate "trek/internal/engine/state"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Store 管理 recovery memory 的 SQLite 持久化与查询。
type Store struct {
	path string
	db   *gorm.DB
	mu   sync.RWMutex
}

type recoveryMemoryRow struct {
	ID uint `gorm:"primaryKey"`

	MemoryKey        string `gorm:"size:1024;not null;index:idx_memory_action,unique"`
	ActionKey        string `gorm:"size:4096;not null;index:idx_memory_action,unique"`
	PageSignature    string `gorm:"size:1024;index"`
	ClusterSignature string `gorm:"size:1024;index"`
	BlockReason      string `gorm:"size:256;index"`
	TraceSignature   string `gorm:"size:2048"`
	Mode             string `gorm:"size:64;index"`

	ItemJSON     string  `gorm:"type:text"`
	Outcome      string  `gorm:"size:64"`
	EscapeScore  float64 `gorm:"not null;default:0"`
	SuccessCount int     `gorm:"not null;default:0"`
	FailCount    int     `gorm:"not null;default:0"`

	LastUsedAt       time.Time `gorm:"index"`
	RecordCreatedAt  time.Time `gorm:"column:record_created_at;index"`
	RecordModifiedAt time.Time `gorm:"column:record_modified_at;index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (recoveryMemoryRow) TableName() string {
	return "recovery_memory_records"
}

// NewStore 创建 Store，并初始化 SQLite 表结构。
func NewStore(path string) (*Store, error) {
	dbPath := resolveSQLitePath(path)
	if err := ensureParentDir(dbPath); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	db = db.Session(&gorm.Session{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err := db.AutoMigrate(&recoveryMemoryRow{}); err != nil {
		return nil, err
	}

	store := &Store{
		path: dbPath,
		db:   db,
	}
	return store, nil
}

// Close 关闭底层数据库连接。
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Append 追加一条记录并在 SQLite 内按 key 聚合。
func (s *Store) Append(record RecoveryMemoryRecord) error {
	if s == nil || s.db == nil {
		return nil
	}
	item := cloneRecord(record)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upsertRecordLocked(item)
}

// AppendOutcome 追加一次恢复结果增量，并在内存中按 key 聚合统计。
func (s *Store) AppendOutcome(record RecoveryMemoryRecord) error {
	return s.Append(record)
}

// All 返回记录快照。
func (s *Store) All() []RecoveryMemoryRecord {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allLocked()
}

// Find 基于上下文执行分层匹配。
func (s *Store) Find(ctx enginestate.TraversalContext) []RecoveryMemoryRecord {
	if s == nil || s.db == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return findMatches(s.findByPageSignatureLocked(ctx.Mode, ctx.PageSignature, ctx.ClusterSignature), ctx)
}

// findByPageSignatureLocked 用 SQL WHERE 按 mode + page/cluster 签名预过滤，
// 避免全表扫描后再在 Go 中逐条匹配。
func (s *Store) findByPageSignatureLocked(mode enginestate.Mode, pageSignature, clusterSignature string) []RecoveryMemoryRecord {
	ctxMode := strings.TrimSpace(string(mode))
	pageSig := strings.TrimSpace(pageSignature)
	clusterSig := strings.TrimSpace(clusterSignature)

	if ctxMode == "" && pageSig == "" && clusterSig == "" {
		return s.allLocked()
	}

	query := s.db.Order("id ASC")
	if ctxMode != "" {
		query = query.Where("mode = ?", ctxMode)
	}
	if pageSig != "" || clusterSig != "" {
		query = query.Where("page_signature = ? OR cluster_signature = ?", pageSig, clusterSig)
	}

	rows := make([]recoveryMemoryRow, 0, 16)
	if err := query.Find(&rows).Error; err != nil {
		return nil
	}
	result := make([]RecoveryMemoryRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toRecord())
	}
	return result
}

func (s *Store) allLocked() []RecoveryMemoryRecord {
	rows := make([]recoveryMemoryRow, 0, 64)
	if err := s.db.Order("id ASC").Find(&rows).Error; err != nil {
		return nil
	}
	result := make([]RecoveryMemoryRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toRecord())
	}
	return result
}

func (s *Store) upsertRecordLocked(item RecoveryMemoryRecord) error {
	actionKey := actionKeyFromRecord(item)
	var row recoveryMemoryRow
	err := s.db.Where("memory_key = ? AND action_key = ?", strings.TrimSpace(item.MemoryKey), actionKey).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		newRow := rowFromRecord(item)
		return s.db.Create(&newRow).Error
	}

	merged := mergeRecord(row.toRecord(), item)
	next := rowFromRecord(merged)
	next.ID = row.ID
	next.CreatedAt = row.CreatedAt
	return s.db.Save(&next).Error
}

func resolveSQLitePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "recovery_memory.sqlite"
	}
	return path
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func rowFromRecord(item RecoveryMemoryRecord) recoveryMemoryRow {
	data, _ := json.Marshal(cloneCandidate(item.Item))
	return recoveryMemoryRow{
		MemoryKey:        strings.TrimSpace(item.MemoryKey),
		ActionKey:        actionKeyFromRecord(item),
		PageSignature:    strings.TrimSpace(item.PageSignature),
		ClusterSignature: strings.TrimSpace(item.ClusterSignature),
		BlockReason:      strings.TrimSpace(item.BlockReason),
		TraceSignature:   strings.TrimSpace(item.TraceSignature),
		Mode:             strings.TrimSpace(item.Mode),
		ItemJSON:         string(data),
		Outcome:          strings.TrimSpace(item.Outcome),
		EscapeScore:      item.EscapeScore,
		SuccessCount:     maxInt(item.SuccessCount, 0),
		FailCount:        maxInt(item.FailCount, 0),
		LastUsedAt:       item.LastUsedAt,
		RecordCreatedAt:  item.CreatedAt,
		RecordModifiedAt: item.LastUsedAt,
	}
}

func (row recoveryMemoryRow) toRecord() RecoveryMemoryRecord {
	result := RecoveryMemoryRecord{
		MemoryKey:        row.MemoryKey,
		PageSignature:    row.PageSignature,
		ClusterSignature: row.ClusterSignature,
		BlockReason:      row.BlockReason,
		TraceSignature:   row.TraceSignature,
		Mode:             row.Mode,
		Outcome:          row.Outcome,
		EscapeScore:      row.EscapeScore,
		SuccessCount:     row.SuccessCount,
		FailCount:        row.FailCount,
		LastUsedAt:       row.LastUsedAt,
		CreatedAt:        row.RecordCreatedAt,
	}
	if result.CreatedAt.IsZero() {
		result.CreatedAt = row.CreatedAt
	}
	if strings.TrimSpace(row.ItemJSON) != "" {
		var c perception.Candidate
		if err := json.Unmarshal([]byte(row.ItemJSON), &c); err == nil {
			result.Item = cloneCandidate(c)
		}
	}
	return result
}

func actionKeyFromRecord(item RecoveryMemoryRecord) string {
	if item.Item.Command == nil {
		return "_nil_action"
	}
	key := strings.TrimSpace(item.Item.Command.ToJSON())
	if key == "" {
		return "_nil_action"
	}
	return key
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
	item.Item = cloneCandidate(src.Item)
	return item
}

func cloneCandidate(src perception.Candidate) perception.Candidate {
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
	if delta.Item.Command != nil {
		result.Item = cloneCandidate(delta.Item)
	}
	return result
}

func recordAggregateKey(item RecoveryMemoryRecord) string {
	act := ""
	if item.Item.Command != nil {
		act = item.Item.Command.ToJSON()
	}
	return strings.TrimSpace(item.MemoryKey) + "|" + act
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
