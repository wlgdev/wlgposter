package app

import (
	"fmt"
	"wlgposter/internal/post"
)

func (a *App) onTelegramPostDelete(p *post.Post) {
	plans := a.plansForDeletePost(p.MessageID)
	if len(plans) == 0 {
		a.log.Debug().Int32("tgid", p.MessageID).Msg("Telegram post delete ignored (cache miss or empty IDs)")
		a.clearAllTargetCaches(p.MessageID)
		return
	}

	for _, plan := range plans {
		actionLabel := plan.Target.DisplayName

		if err := plan.Target.Publisher.Delete(plan.Cached.PlatformID); err != nil {
			a.log.Error().
				Err(err).
				Int32("tgid", p.MessageID).
				Str("target", string(plan.Target.Name)).
				Str("platform_id", plan.Cached.PlatformID).
				Msg(actionLabel + " post delete failed")

			a.tg.MessageToAdmins(fmt.Sprintf("❌ Не удалось удалить пост `%s` в %s для Telegram-поста `%d`: `%s`", plan.Cached.PlatformID, plan.Target.DisplayName, p.MessageID, err.Error()))
		} else {
			a.log.Info().
				Int32("tgid", p.MessageID).
				Str("target", string(plan.Target.Name)).
				Str("platform_id", plan.Cached.PlatformID).
				Msg("Telegram post deleted, " + actionLabel + " post removed")
		}
	}

	a.clearAllTargetCaches(p.MessageID)
}
