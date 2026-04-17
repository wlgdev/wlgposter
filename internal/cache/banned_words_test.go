package cache

import (
	"path/filepath"
	"testing"
)

func TestBannedWordsDB_ListAddDeleteHas(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	store, err := NewBannedWordsDB(db)
	if err != nil {
		t.Fatalf("NewBannedWordsDB() error = %v", err)
	}

	if err := store.Add("foo", true, true); err != nil {
		t.Fatalf("Add(foo) error = %v", err)
	}

	if err := store.Add("bar", false, true); err != nil {
		t.Fatalf("Add(bar) error = %v", err)
	}

	if err := store.Add("foo", true, true); err != nil {
		t.Fatalf("Add(foo duplicate) error = %v", err)
	}

	words, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(words) != 2 {
		t.Fatalf("expected 2 banned words, got %d", len(words))
	}

	if words[0].Word != "foo" || words[1].Word != "bar" {
		t.Fatalf("unexpected banned words order: %+v", words)
	}

	_, hasFoo, err := store.Has("foo")
	if err != nil {
		t.Fatalf("Has(foo) error = %v", err)
	}
	if !hasFoo {
		t.Fatal("expected foo to exist")
	}

	deleted, err := store.Delete("foo")
	if err != nil {
		t.Fatalf("Delete(foo) error = %v", err)
	}
	if !deleted {
		t.Fatal("expected foo to be deleted")
	}

	_, hasFoo, err = store.Has("foo")
	if err != nil {
		t.Fatalf("Has(foo) after delete error = %v", err)
	}
	if hasFoo {
		t.Fatal("expected foo to be absent after delete")
	}
}

func TestBannedWordsDB_ReadsFromCache(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO BannedWords (word) VALUES ('foo')`); err != nil {
		t.Fatalf("seed insert failed: %v", err)
	}

	store, err := NewBannedWordsDB(db)
	if err != nil {
		t.Fatalf("NewBannedWordsDB() error = %v", err)
	}

	if _, err := db.Exec(`INSERT INTO BannedWords (word) VALUES ('bar')`); err != nil {
		t.Fatalf("out-of-band insert failed: %v", err)
	}

	words, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(words) != 1 || words[0].Word != "foo" {
		t.Fatalf("expected cached words [foo], got %+v", words)
	}

	_, hasBar, err := store.Has("bar")
	if err != nil {
		t.Fatalf("Has(bar) error = %v", err)
	}
	if hasBar {
		t.Fatal("expected out-of-band insert to be invisible to cache")
	}

	bw, ok := store.Contains("hello bar world")
	if ok || bw.Word != "" {
		t.Fatalf("expected cached Contains() to ignore out-of-band insert, got %q %v", bw.Word, ok)
	}

	if err := store.Add("baz", true, true); err != nil {
		t.Fatalf("Add(baz) error = %v", err)
	}

	bw, ok = store.Contains("hello baz world")
	if !ok || bw.Word != "baz" {
		t.Fatalf("expected Contains() to see in-memory mutation, got %q %v", bw.Word, ok)
	}
}
