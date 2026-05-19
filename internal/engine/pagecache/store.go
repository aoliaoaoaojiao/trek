package pagecache

import (
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
}

// Store 管理页面理解缓存的 SQLite 持久化。
type Store struct {
	path string
	db   *gorm.DB
	mu   sync.RWMutex
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
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
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
	_ = s.db.Model(&row).Updates(map[string]any{
		"last_used_at": now,
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
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = row.CreatedAt
	}
	return entry
}
