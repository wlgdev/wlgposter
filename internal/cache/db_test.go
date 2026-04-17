package cache

import (
	"path/filepath"
	"testing"
)

func TestNewDBInitializesSchema(t *testing.T) {
	db, err := NewDB(filepath.Join(t.TempDir(), "kv.db"))
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()

	tableNames := []string{"MaxIDCache", "VkIDCache", "MaxMediaCache", "VkMediaCache", "BannedWords"}
	for _, tableName := range tableNames {
		var count int
		if err := db.Get(&count, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName); err != nil {
			t.Fatalf("schema lookup for %s failed: %v", tableName, err)
		}
		if count != 1 {
			t.Fatalf("expected table %s to exist, count = %d", tableName, count)
		}
	}
}
