package app

import (
	"context"
	"time"
	"wlgposter/internal/cache"
	"wlgposter/internal/post"
)

const (
	TELEGRAM_DELETE_CHECK_LIMIT    = 50
	TELEGRAM_DELETE_CHECK_INTERVAL = 60 * time.Second
)

type telegramChannelKey struct {
	channelID  int64
	accessHash int64
}

func (a *App) startTelegramDeleteWatcher(ctx context.Context) {
	if TELEGRAM_DELETE_CHECK_LIMIT == 0 || TELEGRAM_DELETE_CHECK_INTERVAL == 0 {
		a.log.Debug().Msg("Telegram delete watcher disabled")
		return
	}

	go func() {
		a.log.Info().Int("limit", TELEGRAM_DELETE_CHECK_LIMIT).Int("interval", int(TELEGRAM_DELETE_CHECK_INTERVAL.Seconds())).Msg("Telegram delete watcher started")

		ticker := time.NewTicker(TELEGRAM_DELETE_CHECK_INTERVAL)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.checkDeletedTelegramPosts()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (a *App) checkDeletedTelegramPosts() {
	entriesMax := a.maxIDCache.ListRecent(TELEGRAM_DELETE_CHECK_LIMIT)
	entriesVk := a.vkIDCache.ListRecent(TELEGRAM_DELETE_CHECK_LIMIT)
	if len(entriesMax) == 0 && len(entriesVk) == 0 {
		return
	}

	entries := append(entriesMax, entriesVk...)

	groups := make(map[telegramChannelKey][]cache.IDCacheEntry)
	for _, entry := range entries {
		if entry.ChannelID == 0 || entry.AccessHash == 0 {
			continue
		}

		key := telegramChannelKey{
			channelID:  entry.ChannelID,
			accessHash: entry.AccessHash,
		}
		groups[key] = append(groups[key], entry)
	}

	for key, group := range groups {
		ids := make([]int32, 0, len(group))
		for _, entry := range group {
			ids = append(ids, entry.TgID)
		}

		messages, err := a.tg.GetChannelMessages(key.channelID, key.accessHash, ids)
		if err != nil {
			a.log.Error().
				Err(err).
				Int64("channel_id", key.channelID).
				Int("messages", len(ids)).
				Msg("Telegram delete watcher GetMessages failed")
			continue
		}

		// if len(messages) != len(group) {
		// 	a.log.Warn().
		// 		Int64("channel_id", key.channelID).
		// 		Int("requested", len(group)).
		// 		Int("received", len(messages)).
		// 		Msg("Telegram delete watcher response size mismatch")
		// }

		for i, message := range messages {
			if i >= len(group) || !message.IsEmpty() {
				continue
			}

			entry := group[i]
			a.onTelegramPostDelete(&post.Post{
				ChatID:     entry.ChannelID,
				AccessHash: entry.AccessHash,
				MessageID:  entry.TgID,
			})
		}
	}
}
