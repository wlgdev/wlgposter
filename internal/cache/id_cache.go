package cache

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
)

type IDCache struct {
	capacity  int
	db        *DB
	tableName string

	mu sync.RWMutex
}

type IDCacheEntry struct {
	RowID      int64  `db:"row_id"`
	TgID       int32  `db:"tg_id"`
	PlatformID string `db:"platform_id"`
	Hash       string `db:"hash"`
	ChannelID  int64  `db:"channel_id"`
	AccessHash int64  `db:"access_hash"`
}

func NewIDCacheDB(capacity int, db *DB, prefix string) *IDCache {
	return &IDCache{
		capacity:  capacity,
		db:        db,
		tableName: prefix + "IDCache",
	}
}

func (c *IDCache) Get(tgID int32) (IDCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return IDCacheEntry{}, false
	}

	var entry IDCacheEntry
	err := c.db.Get(
		&entry,
		fmt.Sprintf(`SELECT tg_id, platform_id, hash, channel_id, access_hash FROM %s WHERE tg_id = ?`, c.tableName),
		tgID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return IDCacheEntry{}, false
		}
		return IDCacheEntry{}, false
	}

	return entry, true
}

func (c *IDCache) ListRecent(limit int) []IDCacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil || limit <= 0 {
		return nil
	}

	entries := make([]IDCacheEntry, 0, limit)
	if err := c.db.Select(&entries, fmt.Sprintf(`SELECT tg_id, platform_id, hash, channel_id, access_hash FROM %s ORDER BY row_id DESC LIMIT ?`, c.tableName), limit); err != nil {
		return nil
	}

	return entries
}

func (c *IDCache) Set(tgID int32, platformID string, hash string, channelID int64, accessHash int64) {
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

	result, err := tx.Exec(
		fmt.Sprintf(`UPDATE %s SET platform_id = ?, hash = ?, channel_id = ?, access_hash = ? WHERE tg_id = ?`, c.tableName),
		platformID,
		hash,
		channelID,
		accessHash,
		tgID,
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
			fmt.Sprintf(`INSERT INTO %s (tg_id, platform_id, hash, channel_id, access_hash) VALUES (?, ?, ?, ?, ?)`, c.tableName),
			tgID,
			platformID,
			hash,
			channelID,
			accessHash,
		); err != nil {
			return
		}

		if err := purgeOverflowRows(tx, c.tableName, c.normalizedCapacity()); err != nil {
			return
		}
	}

	_ = tx.Commit()
}

func (c *IDCache) Delete(tgID int32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return
	}

	_, _ = c.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE tg_id = ?`, c.tableName), tgID)
}

func (c *IDCache) normalizedCapacity() int {
	if c.capacity < 0 {
		return 0
	}

	return c.capacity
}

func purgeOverflowRows(tx *sqlx.Tx, tableName string, capacity int) error {
	var count int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tableName)
	if err := tx.Get(&count, query); err != nil {
		return fmt.Errorf("error counting rows in %s: %w", tableName, err)
	}

	overflow := count - capacity
	if overflow <= 0 {
		return nil
	}

	deleteQuery := fmt.Sprintf(`DELETE FROM %s WHERE row_id IN (SELECT row_id FROM %s ORDER BY row_id ASC LIMIT ?)`, tableName, tableName)
	if _, err := tx.Exec(deleteQuery, overflow); err != nil {
		return fmt.Errorf("error deleting overflow rows from %s: %w", tableName, err)
	}

	return nil
}
