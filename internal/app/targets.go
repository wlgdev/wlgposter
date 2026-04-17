package app

import (
	"wlgposter/internal/cache"
	"wlgposter/internal/post"
	"wlgposter/internal/publisher"
)

type TargetName string

const (
	TargetMax TargetName = "max"
	TargetVK  TargetName = "vk"
)

type Target struct {
	Name          TargetName
	DisplayName   string
	IDCache       idCacheStore
	MediaCache    mediaCacheStore
	Hash          func(*post.Post) string
	Publisher     publisher.Client
	CanPublish    func(*post.Post) bool
	GetMediaToken func(*post.Media) string
	SetMediaToken func(*post.Media, string)
}

type TargetPlan struct {
	Target  *Target
	Cached  cache.IDCacheEntry
	Options publisher.Options
}

func (a *App) logTargetBlockedByBannedWord(p *post.Post, target *Target, word cache.BannedWord, postType PostType) {
	a.log.Info().
		Int32("tgid", p.MessageID).
		Str("target", string(target.Name)).
		Str("type", string(postType)).
		Str("word", word.Word).
		Bool("vk", word.PlatformVK).
		Bool("max", word.PlatformMax).
		Msg("Telegram post skipped for target by banned word")
}

func (a *App) hasActiveTargetsForPost(p *post.Post, word cache.BannedWord, hasWord bool) bool {
	for _, target := range a.targets {
		if a.isTargetBlocked(target.Name, word, hasWord) {
			continue
		}
		if target.CanPublish != nil && !target.CanPublish(p) {
			continue
		}

		return true
	}

	return false
}

func (a *App) isTargetBlocked(name TargetName, word cache.BannedWord, hasWord bool) bool {
	if !hasWord {
		return false
	}

	switch name {
	case TargetMax:
		return word.PlatformMax
	case TargetVK:
		return word.PlatformVK
	default:
		return false
	}
}

func (a *App) publishOptionsFor(target *Target, p *post.Post) publisher.Options {
	opts := publisher.Options{}
	if target.Name == TargetMax {
		opts.ReplyID = a.replyMaxIDFor(p.ReplyMessageID)
	}

	return opts
}

func (a *App) plansForNewPost(p *post.Post, word cache.BannedWord, hasWord bool) []TargetPlan {
	plans := make([]TargetPlan, 0, len(a.targets))

	for _, target := range a.targets {
		if a.isTargetBlocked(target.Name, word, hasWord) {
			a.logTargetBlockedByBannedWord(p, target, word, TypeNew)
			continue
		}
		if target.CanPublish != nil && !target.CanPublish(p) {
			continue
		}

		target.IDCache.Set(p.MessageID, "", target.Hash(p), p.ChatID, p.AccessHash)
		plans = append(plans, TargetPlan{
			Target:  target,
			Options: a.publishOptionsFor(target, p),
		})
	}

	return plans
}

func (a *App) plansForEditPost(p *post.Post, word cache.BannedWord, hasWord bool) ([]TargetPlan, bool, bool) {
	plans := make([]TargetPlan, 0, len(a.targets))
	hasCached := false
	hasChanges := false

	for _, target := range a.targets {
		entry, ok := target.IDCache.Get(p.MessageID)
		if !ok {
			continue
		}

		hasCached = true

		if entry.Hash == target.Hash(p) {
			continue
		}

		hasChanges = true
		if a.isTargetBlocked(target.Name, word, hasWord) {
			continue
		}
		if target.CanPublish != nil && !target.CanPublish(p) {
			continue
		}

		plans = append(plans, TargetPlan{
			Target:  target,
			Cached:  entry,
			Options: a.publishOptionsFor(target, p),
		})
	}

	return plans, hasCached, hasChanges
}

func (a *App) plansForDeletePost(messageID int32) []TargetPlan {
	plans := make([]TargetPlan, 0, len(a.targets))

	for _, target := range a.targets {
		entry, ok := target.IDCache.Get(messageID)
		if !ok || entry.PlatformID == "" {
			continue
		}

		plans = append(plans, TargetPlan{
			Target: target,
			Cached: entry,
		})
	}

	return plans
}

func (a *App) clearAllTargetCaches(messageID int32) {
	a.maxIDCache.Delete(messageID)
	a.vkIDCache.Delete(messageID)
}
