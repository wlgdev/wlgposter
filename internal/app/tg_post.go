package app

import (
	"wlgposter/internal/post"
)

func (a *App) onTelegramPost(p *post.Post) {

	if len(a.targets) == 0 {
		return
	}
	if !a.isTargetChannel(p.ChatID) {
		return
	}
	if p.IsForward {
		a.log.Warn().Int32("tgid", p.MessageID).Msg("Telegram post new ignored (forward)")
		return
	}
	if len(p.Text) == 0 && len(p.Media) == 0 {
		a.log.Warn().Int32("tgid", p.MessageID).Msg("Telegram post new ignored (empty)")
		return
	}

	word, ok := a.bannedWords.Contains(p.Text)
	if ok && word.PlatformVK && word.PlatformMax {
		a.log.Warn().Str("type", "new").Str("text", p.Text).Str("word", word.Word).Bool("vk", word.PlatformVK).Bool("max", word.PlatformMax).Msg("Telegram post contains banned word on both platforms, skipping")
		return
	}

	plans := a.plansForNewPost(p, word, ok)
	if len(plans) == 0 {
		a.log.Debug().Int32("tgid", p.MessageID).Msg("Telegram post new ignored (no active targets)")
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
		Str("type", string(TypeNew)).
		Strs("targets", targets).
		Msg("Telegram post")

	a.enqueueJob(PostJob{
		Post:     p,
		PostType: TypeNew,
		Plans:    plans,
	})
}
