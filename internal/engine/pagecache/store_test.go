package pagecache

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStorePutAndReloadFromSQLite(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "page_control_cache.sqlite")
	store, err := NewStore(filePath)
	if err != nil {
		t.Fatalf("创建 Store 失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	entry := Entry{
		CacheKey:     "ocr|fingerprint-1",
		Strategy:     "ocr",
		Fingerprint:  "fingerprint-1",
		SyntheticXML: `<hierarchy><node text="缓存控件"/></hierarchy>`,
	}
	if err := store.Put(entry); err != nil {
		t.Fatalf("写入缓存失败: %v", err)
	}

	reloaded, err := NewStore(filePath)
	if err != nil {
		t.Fatalf("重载 Store 失败: %v", err)
	}
	defer func() { _ = reloaded.Close() }()

	got, ok := reloaded.Get("ocr|fingerprint-1")
	if !ok {
		t.Fatal("预期重载后能读取到缓存")
	}
	if got.SyntheticXML != entry.SyntheticXML {
		t.Fatalf("缓存 XML 不一致: got=%s want=%s", got.SyntheticXML, entry.SyntheticXML)
	}
	if got.Strategy != "ocr" || got.Fingerprint != "fingerprint-1" {
		t.Fatalf("缓存元数据错误: %+v", got)
	}
}

func TestStorePutOverridesExistingEntry(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "page_control_cache.sqlite"))
	if err != nil {
		t.Fatalf("创建 Store 失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Put(Entry{
		CacheKey:     "llm|fingerprint-2",
		Strategy:     "llm",
		Fingerprint:  "fingerprint-2",
		SyntheticXML: `<hierarchy><node text="旧值"/></hierarchy>`,
		CreatedAt:    time.Date(2026, 5, 19, 10, 0, 0, 0, time.Local),
	}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}
	if err := store.Put(Entry{
		CacheKey:     "llm|fingerprint-2",
		Strategy:     "llm",
		Fingerprint:  "fingerprint-2",
		SyntheticXML: `<hierarchy><node text="新值"/></hierarchy>`,
	}); err != nil {
		t.Fatalf("覆盖写入失败: %v", err)
	}

	got, ok := store.Get("llm|fingerprint-2")
	if !ok {
		t.Fatal("预期能读到覆盖后的记录")
	}
	if got.SyntheticXML != `<hierarchy><node text="新值"/></hierarchy>` {
		t.Fatalf("缓存未被覆盖: %+v", got)
	}
}

func TestStoreDeleteRemovesEntry(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "page_control_cache.sqlite"))
	if err != nil {
		t.Fatalf("创建 Store 失败: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.Put(Entry{
		CacheKey:     "ocr|fingerprint-3",
		Strategy:     "ocr",
		Fingerprint:  "fingerprint-3",
		SyntheticXML: `<hierarchy><node text="待删除"/></hierarchy>`,
	}); err != nil {
		t.Fatalf("写入缓存失败: %v", err)
	}
	if err := store.Delete("ocr|fingerprint-3"); err != nil {
		t.Fatalf("删除缓存失败: %v", err)
	}
	if _, ok := store.Get("ocr|fingerprint-3"); ok {
		t.Fatal("预期缓存已被删除")
	}
}
