package app

import (
	"slices"
)

func (a *App) isTargetChannel(chatID int64) bool {
	return slices.Contains(a.cfg.TelegramTargetChannelID, chatID)
}

func (a *App) replyMaxIDFor(replyMessageID int32) string {
	replyMaxID := ""
	if replyMessageID != 0 {
		if replyItem, ok := a.maxIDCache.Get(replyMessageID); ok {
			replyMaxID = replyItem.PlatformID
		}
	}

	return replyMaxID
}

func (a *App) enqueueJob(job PostJob) {
	select {
	case a.jobs <- job:
	default:
		a.log.Error().Msg("MAX post queue is full")
	}
}
