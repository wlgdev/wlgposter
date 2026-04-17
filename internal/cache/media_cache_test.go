package cache

import (
	"path/filepath"
	"testing"
)

func TestMediaCacheDB_SetManyPersistsAndEvicts(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	cache := NewMediaCacheDB(2, db, "Max")
	cache.SetMany([]MediaCacheEntry{
		{TgFileID: "file-1", Type: "photo", PlatformToken: "token-1", FileName: "a.jpg", Size: 10},
		{TgFileID: "file-2", Type: "video", PlatformToken: "token-2", FileName: "b.mp4", Size: 20},
	})

	if got, ok := cache.Get("photo", "file-1"); !ok || got.PlatformToken != "token-1" {
		t.Fatalf("cache.Get(photo, file-1) = %+v, %v", got, ok)
	}

	cache.SetMany([]MediaCacheEntry{
		{TgFileID: "file-3", Type: "audio", PlatformToken: "token-3", FileName: "c.mp3", Size: 30},
	})

	if _, ok := cache.Get("photo", "file-1"); ok {
		t.Fatal("expected oldest entry to be evicted")
	}

	if got, ok := cache.Get("audio", "file-3"); !ok || got.PlatformToken != "token-3" {
		t.Fatalf("cache.Get(audio, file-3) = %+v, %v", got, ok)
	}
}

func TestMediaCacheDB_UpdateDoesNotChangeOrder(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	cache := NewMediaCacheDB(2, db, "Max")

	cache.Set("photo", "file-1", "token-1", "a.jpg", 10)
	cache.Set("video", "file-2", "token-2", "b.mp4", 20)
	cache.Set("photo", "file-1", "token-1b", "a-2.jpg", 11)
	cache.Set("audio", "file-3", "token-3", "c.mp3", 30)

	if _, ok := cache.Get("photo", "file-1"); ok {
		t.Fatal("expected file-1 to be evicted as the oldest inserted row")
	}

	if got, ok := cache.Get("video", "file-2"); !ok || got.PlatformToken != "token-2" {
		t.Fatalf("cache.Get(video, file-2) = %+v, %v", got, ok)
	}
	if got, ok := cache.Get("audio", "file-3"); !ok || got.PlatformToken != "token-3" {
		t.Fatalf("cache.Get(audio, file-3) = %+v, %v", got, ok)
	}

	recent := cache.ListRecent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent entries, got %d", len(recent))
	}
	if recent[0].Type != "audio" || recent[0].TgFileID != "file-3" {
		t.Fatalf("unexpected first recent row: %+v", recent[0])
	}
	if recent[1].Type != "video" || recent[1].TgFileID != "file-2" {
		t.Fatalf("unexpected second recent row: %+v", recent[1])
	}
}
