package app

import (
	"fmt"
	"wlgposter/internal/post"
)

func (a *App) onTelegramPostEdit(p *post.Post) {
	if !a.isTargetChannel(p.ChatID) {
		return
	}
	if len(a.targets) == 0 {
		return
	}
	if p.IsForward {
		a.log.Debug().Int32("tgid", p.MessageID).Msg("Telegram post edit ignored (forward)")
		return
	}
	if len(p.Text) == 0 && len(p.Media) == 0 {
		a.log.Warn().Int32("tgid", p.MessageID).Msg("Telegram post new ignored (empty)")
		return
	}

	word, ok := a.bannedWords.Contains(p.Text)
	if ok && word.PlatformVK && word.PlatformMax {
		// a.log.Warn().Str("type", "edit").Str("text", p.Text).Str("word", word.Word).Bool("vk", word.PlatformVK).Bool("max", word.PlatformMax).Msg("Telegram post contains banned word on both platforms, skipping")
		return
	}

	plans, hasCached, hasChanges := a.plansForEditPost(p, word, ok)
	if !hasCached {
		if !a.hasActiveTargetsForPost(p, word, ok) {
			return
		}
		a.log.Error().Int32("tgid", p.MessageID).Msg("Failed edit post, cache miss for both platforms")
		a.tg.MessageToAdmins(fmt.Sprintf("❌ Не удалось изменить пост [%d](%s). Пост не найден ни в одном кэше.", p.MessageID, p.Link))
		return
	}

	if !hasChanges {
		a.log.Debug().Int32("tgid", p.MessageID).Msg("Telegram post edit ignored (no changes)")
		return
	}

	if len(plans) == 0 {
		return
	}

	targets := make([]string, 0, len(plans))
	for _, plan := range plans {
		targets = append(targets, string(plan.Target.Name))
	}

	a.log.Info().
		Str("text", p.Text).
		Int32("tgid", p.MessageID).
		Int32("reply_tgid", p.ReplyMessageID).
		Int("entities", len(p.Entities)).
		Int("media", len(p.Media)).
		Strs("targets", targets).
		Str("type", string(TypeEdit)).
		Msg("Telegram post")

	a.enqueueJob(PostJob{
		Post:     p,
		PostType: TypeEdit,
		Plans:    plans,
	})
}
