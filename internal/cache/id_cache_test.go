package cache

import (
	"path/filepath"
	"testing"
)

func TestIDCacheDB_SetGetDeleteAndListRecent(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	cache := NewIDCacheDB(2, db, "Max")

	cache.Set(10, "max-10", "hash-10", 100, 200)
	cache.Set(20, "max-20", "hash-20", 100, 200)

	if got, ok := cache.Get(10); !ok || got.PlatformID != "max-10" {
		t.Fatalf("cache.Get(10) = %+v, %v", got, ok)
	}

	recent := cache.ListRecent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(recent))
	}
	if recent[0].TgID != 20 || recent[1].TgID != 10 {
		t.Fatalf("unexpected recent order: %+v", recent)
	}

	cache.Delete(10)
	if _, ok := cache.Get(10); ok {
		t.Fatal("expected entry 10 to be deleted")
	}
}

func TestIDCacheDB_UpdateDoesNotChangeOrderAndEvictsOldest(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	cache := NewIDCacheDB(2, db, "Max")

	cache.Set(10, "max-10", "hash-10", 100, 200)
	cache.Set(20, "max-20", "hash-20", 100, 200)
	cache.Set(10, "max-10-updated", "hash-10-updated", 300, 400)
	cache.Set(30, "max-30", "hash-30", 100, 200)

	if _, ok := cache.Get(10); ok {
		t.Fatal("expected entry 10 to be evicted as the oldest inserted row")
	}

	if got, ok := cache.Get(20); !ok || got.PlatformID != "max-20" {
		t.Fatalf("cache.Get(20) = %+v, %v", got, ok)
	}
	if got, ok := cache.Get(30); !ok || got.PlatformID != "max-30" {
		t.Fatalf("cache.Get(30) = %+v, %v", got, ok)
	}

	recent := cache.ListRecent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(recent))
	}
	if recent[0].TgID != 30 || recent[1].TgID != 20 {
		t.Fatalf("unexpected recent order after update: %+v", recent)
	}
}
