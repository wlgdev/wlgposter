package cache

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sqlx.DB
	filePath string
}

type MigrationStats struct {
	IDCacheRows     int
	MediaCacheRows  int
	BannedWordsRows int
}

var sqliteSchemaStatements = []string{
	// --- MAX TABLES ---
	`CREATE TABLE IF NOT EXISTS MaxIDCache (
		row_id INTEGER PRIMARY KEY AUTOINCREMENT,
		tg_id INTEGER NOT NULL UNIQUE,
		platform_id TEXT NOT NULL DEFAULT '',
		hash TEXT NOT NULL DEFAULT '',
		channel_id INTEGER NOT NULL DEFAULT 0,
		access_hash INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_maxidcache_row_id ON MaxIDCache(row_id DESC)`,

	`CREATE TABLE IF NOT EXISTS MaxMediaCache (
		row_id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		tg_file_id TEXT NOT NULL,
		platform_token TEXT NOT NULL DEFAULT '',
		file_name TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL DEFAULT 0,
		UNIQUE(type, tg_file_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_maxmediacache_row_id ON MaxMediaCache(row_id DESC)`,

	// --- VK TABLES ---
	`CREATE TABLE IF NOT EXISTS VkIDCache (
		row_id INTEGER PRIMARY KEY AUTOINCREMENT,
		tg_id INTEGER NOT NULL UNIQUE,
		platform_id TEXT NOT NULL DEFAULT '',
		hash TEXT NOT NULL DEFAULT '',
		channel_id INTEGER NOT NULL DEFAULT 0,
		access_hash INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_vkidcache_row_id ON VkIDCache(row_id DESC)`,

	`CREATE TABLE IF NOT EXISTS VkMediaCache (
		row_id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		tg_file_id TEXT NOT NULL,
		platform_token TEXT NOT NULL DEFAULT '',
		file_name TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL DEFAULT 0,
		UNIQUE(type, tg_file_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_vkmediacache_row_id ON VkMediaCache(row_id DESC)`,

	// --- BANNED WORDS ---
	`CREATE TABLE IF NOT EXISTS BannedWords (
		row_id INTEGER PRIMARY KEY AUTOINCREMENT,
		word TEXT NOT NULL UNIQUE,
		platform_vk BOOLEAN NOT NULL DEFAULT 1,
		platform_max BOOLEAN NOT NULL DEFAULT 1
	)`,
	`CREATE INDEX IF NOT EXISTS idx_bannedwords_row_id ON BannedWords(row_id ASC)`,
}

func NewDB(filePath string) (*DB, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o777); err != nil {
		return nil, fmt.Errorf("error creating db directory: %w", err)
	}

	sqliteDB, err := sqlx.Open("sqlite", absPath)
	if err != nil {
		return nil, fmt.Errorf("error opening sqlite db: %w", err)
	}

	sqliteDB.SetMaxOpenConns(1)

	if err := sqliteDB.Ping(); err != nil {
		_ = sqliteDB.Close()
		return nil, fmt.Errorf("error pinging sqlite db: %w", err)
	}

	db := &DB{
		DB:       sqliteDB,
		filePath: absPath,
	}

	if err := db.Init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (db *DB) Init() error {
	for _, statement := range sqliteSchemaStatements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("error initializing sqlite schema: %w", err)
		}
	}

	return nil
}
