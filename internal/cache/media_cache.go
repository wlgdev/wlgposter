package cache

import (
	"database/sql"
	"fmt"
	"sync"
)

type MediaCache struct {
	capacity  int
	db        *DB
	tableName string

	mu sync.RWMutex
}

type MediaCacheEntry struct {
	RowID         int64  `db:"row_id"`
	TgFileID      string `db:"tg_file_id"`
	Type          string `db:"type"`
	PlatformToken string `db:"platform_token"`
	FileName      string `db:"file_name"`
	Size          int64  `db:"size"`
}

func NewMediaCacheDB(capacity int, db *DB, prefix string) *MediaCache {
	return &MediaCache{
		capacity:  capacity,
		db:        db,
		tableName: prefix + "MediaCache",
	}
}

func (c *MediaCache) Get(mediaType, tgFileID string) (MediaCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return MediaCacheEntry{}, false
	}

	var entry MediaCacheEntry
	err := c.db.Get(
		&entry,
		fmt.Sprintf(`SELECT type, tg_file_id, platform_token, file_name, size FROM %s WHERE type = ? AND tg_file_id = ?`, c.tableName),
		mediaType,
		tgFileID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return MediaCacheEntry{}, false
		}
		return MediaCacheEntry{}, false
	}

	return entry, true
}

func (c *MediaCache) ListRecent(limit int) []MediaCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil || limit <= 0 {
		return nil
	}

	entries := make([]MediaCacheEntry, 0, limit)
	if err := c.db.Select(&entries, fmt.Sprintf(`SELECT type, tg_file_id, platform_token, file_name, size FROM %s ORDER BY row_id DESC LIMIT ?`, c.tableName), limit); err != nil {
		return nil
	}

	return entries
}

func (c *MediaCache) SetMany(entries []MediaCacheEntry) {
	if len(entries) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return
	}

	tx, err := c.db.Beginx()
	if err != nil {
		return
	}
	defer tx.Rollback()

	inserted := 0
	for _, entry := range entries {
		if entry.TgFileID == "" || entry.Type == "" || entry.PlatformToken == "" {
			continue
		}

		result, err := tx.Exec(
			fmt.Sprintf(`UPDATE %s SET platform_token = ?, file_name = ?, size = ? WHERE type = ? AND tg_file_id = ?`, c.tableName),
			entry.PlatformToken,
			entry.FileName,
			entry.Size,
			entry.Type,
			entry.TgFileID,
		)
		if err != nil {
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return
		}

		if rowsAffected == 0 {
			if _, err := tx.Exec(
				fmt.Sprintf(`INSERT INTO %s (type, tg_file_id, platform_token, file_name, size) VALUES (?, ?, ?, ?, ?)`, c.tableName),
				entry.Type,
				entry.TgFileID,
				entry.PlatformToken,
				entry.FileName,
				entry.Size,
			); err != nil {
				return
			}
			inserted++
		}
	}

	if inserted > 0 {
		if err := purgeOverflowRows(tx, c.tableName, c.normalizedCapacity()); err != nil {
			return
		}
	}

	_ = tx.Commit()
}

func (c *MediaCache) Set(mediaType, tgFileID, platformToken, fileName string, size int64) {
	c.SetMany([]MediaCacheEntry{{
		TgFileID:      tgFileID,
		Type:          mediaType,
		PlatformToken: platformToken,
		FileName:      fileName,
		Size:          size,
	}})
}

func (c *MediaCache) Delete(mediaType, tgFileID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return
	}

	_, _ = c.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE type = ? AND tg_file_id = ?`, c.tableName), mediaType, tgFileID)
}

func (c *MediaCache) normalizedCapacity() int {
	if c.capacity < 0 {
		return 0
	}

	return c.capacity
}
