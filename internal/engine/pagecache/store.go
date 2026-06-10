package pagecache

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Entry 表示一条页面理解缓存记录。
type Entry struct {
	CacheKey     string
	Strategy     string
	Fingerprint  string
	SyntheticXML string
	RefreshedAt  time.Time
	LastUsedAt   time.Time
	CreatedAt    time.Time
	HitCount     int
}

// Store 管理页面理解缓存的 SQLite 持久化。
type Store struct {
	path      string
	db        *gorm.DB
	mu        sync.RWMutex
	cancel    context.CancelFunc
	baseTTL   time.Duration
	maxTTL    time.Duration
	cleanupFn func(count int) // 可选的清理回调，用于日志记录
}

// CleanupOptions 配置后台清理行为。
type CleanupOptions struct {
	BaseTTL       time.Duration // 基础 TTL
	MaxTTL        time.Duration // 最大 TTL 上限
	CleanupEvery  time.Duration // 清理间隔（默认 10 分钟）
	CleanupFn     func(count int) // 清理回调
}

type pageCacheRow struct {
	ID uint `gorm:"primaryKey"`

	CacheKey     string `gorm:"size:1024;not null;uniqueIndex"`
	Strategy     string `gorm:"size:64;index"`
	Fingerprint  string `gorm:"size:1024;index"`
	SyntheticXML string `gorm:"type:text"`

	LastUsedAt       time.Time `gorm:"index"`
	RecordCreatedAt  time.Time `gorm:"column:record_created_at;index"`
	RecordModifiedAt time.Time `gorm:"column:record_modified_at;index"`
	HitCount         int       `gorm:"column:hit_count;index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (pageCacheRow) TableName() string {
	return "page_control_cache_records"
}

// NewStore 创建页面理解缓存 Store，并初始化 SQLite 表结构。
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
	if err := db.AutoMigrate(&pageCacheRow{}); err != nil {
		return nil, err
	}
	return &Store{
		path: dbPath,
		db:   db,
	}, nil
}

// Close 关闭底层数据库连接。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// StartCleanup 启动后台清理协程，定期删除过期缓存记录。
func (s *Store) StartCleanup(ctx context.Context, opts CleanupOptions) {
	if s == nil || s.db == nil {
		return
	}
	if opts.BaseTTL <= 0 {
		opts.BaseTTL = 1 * time.Hour
	}
	if opts.MaxTTL <= 0 {
		opts.MaxTTL = 72 * time.Hour
	}
	if opts.CleanupEvery <= 0 {
		opts.CleanupEvery = 7 * 24 * time.Hour
	}
	s.baseTTL = opts.BaseTTL
	s.maxTTL = opts.MaxTTL
	s.cleanupFn = opts.CleanupFn

	cleanupCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go func() {
		// 首次清理在启动后立即执行
		s.cleanupExpired()

		ticker := time.NewTicker(opts.CleanupEvery)
		defer ticker.Stop()
		for {
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
				s.cleanupExpired()
			}
		}
	}()
}

// cleanupExpired 删除所有已过期的缓存记录。
func (s *Store) cleanupExpired() {
	if s == nil || s.db == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var rows []pageCacheRow
	if err := s.db.Find(&rows).Error; err != nil {
		return
	}

	now := time.Now()
	var expiredIDs []uint
	for _, row := range rows {
		ttl := s.computeTTL(row.HitCount)
		if ttl > 0 && now.Sub(row.RecordModifiedAt) > ttl {
			expiredIDs = append(expiredIDs, row.ID)
		}
	}

	if len(expiredIDs) == 0 {
		return
	}

	if err := s.db.Where("id IN ?", expiredIDs).Delete(&pageCacheRow{}).Error; err != nil {
		return
	}

	if s.cleanupFn != nil {
		s.cleanupFn(len(expiredIDs))
	}
}

// computeTTL 根据 hitCount 计算有效的 TTL。
func (s *Store) computeTTL(hitCount int) time.Duration {
	baseTTL := s.baseTTL
	if baseTTL <= 0 {
		baseTTL = 1 * time.Hour
	}
	maxTTL := s.maxTTL
	if maxTTL <= 0 {
		maxTTL = 72 * time.Hour
	}

	if hitCount <= 1 {
		return baseTTL
	}
	boost := 1.0 + math.Log(float64(hitCount))
	ttl := time.Duration(float64(baseTTL) * boost)
	if ttl > maxTTL {
		ttl = maxTTL
	}
	return ttl
}

// Stats 返回缓存统计信息。
func (s *Store) Stats() (total, expired int, err error) {
	if s == nil || s.db == nil {
		return 0, 0, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	if err := s.db.Model(&pageCacheRow{}).Count(&count).Error; err != nil {
		return 0, 0, err
	}
	total = int(count)

	var rows []pageCacheRow
	if err := s.db.Find(&rows).Error; err != nil {
		return total, 0, err
	}

	now := time.Now()
	for _, row := range rows {
		ttl := s.computeTTL(row.HitCount)
		if ttl > 0 && now.Sub(row.RecordModifiedAt) > ttl {
			expired++
		}
	}
	return total, expired, nil
}

// Get 根据缓存键读取一条记录。
func (s *Store) Get(cacheKey string) (Entry, bool) {
	if s == nil || s.db == nil {
		return Entry{}, false
	}
	key := strings.TrimSpace(cacheKey)
	if key == "" {
		return Entry{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var row pageCacheRow
	if err := s.db.Where("cache_key = ?", key).Take(&row).Error; err != nil {
		return Entry{}, false
	}
	now := time.Now()
	row.HitCount++
	_ = s.db.Model(&row).Updates(map[string]any{
		"last_used_at": now,
		"hit_count":    row.HitCount,
	}).Error
	return row.toEntry(), true
}

// Put 写入或覆盖一条缓存记录。
func (s *Store) Put(entry Entry) error {
	if s == nil || s.db == nil {
		return nil
	}
	item := normalizeEntry(entry)
	if item.CacheKey == "" || item.SyntheticXML == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var row pageCacheRow
	err := s.db.Where("cache_key = ?", item.CacheKey).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			newRow := rowFromEntry(item)
			return s.db.Create(&newRow).Error
		}
		return err
	}
	next := rowFromEntry(item)
	next.ID = row.ID
	next.CreatedAt = row.CreatedAt
	next.HitCount = row.HitCount // 保留已有命中次数
	return s.db.Save(&next).Error
}

// Delete 删除指定缓存键对应的记录。
func (s *Store) Delete(cacheKey string) error {
	if s == nil || s.db == nil {
		return nil
	}
	key := strings.TrimSpace(cacheKey)
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Where("cache_key = ?", key).Delete(&pageCacheRow{}).Error
}

func resolveSQLitePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "page_control_cache.sqlite"
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

func normalizeEntry(entry Entry) Entry {
	now := time.Now()
	result := entry
	result.CacheKey = strings.TrimSpace(result.CacheKey)
	result.Strategy = strings.TrimSpace(result.Strategy)
	result.Fingerprint = strings.TrimSpace(result.Fingerprint)
	result.SyntheticXML = strings.TrimSpace(result.SyntheticXML)
	if result.CreatedAt.IsZero() {
		result.CreatedAt = now
	}
	if result.RefreshedAt.IsZero() {
		result.RefreshedAt = now
	}
	if result.LastUsedAt.IsZero() {
		result.LastUsedAt = now
	}
	return result
}

func rowFromEntry(entry Entry) pageCacheRow {
	item := normalizeEntry(entry)
	return pageCacheRow{
		CacheKey:         item.CacheKey,
		Strategy:         item.Strategy,
		Fingerprint:      item.Fingerprint,
		SyntheticXML:     item.SyntheticXML,
		LastUsedAt:       item.LastUsedAt,
		RecordCreatedAt:  item.CreatedAt,
		RecordModifiedAt: item.RefreshedAt,
		HitCount:         item.HitCount,
	}
}

func (row pageCacheRow) toEntry() Entry {
	entry := Entry{
		CacheKey:     row.CacheKey,
		Strategy:     row.Strategy,
		Fingerprint:  row.Fingerprint,
		SyntheticXML: row.SyntheticXML,
		RefreshedAt:  row.RecordModifiedAt,
		LastUsedAt:   row.LastUsedAt,
		CreatedAt:    row.RecordCreatedAt,
		HitCount:     row.HitCount,
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = row.CreatedAt
	}
	return entry
}
