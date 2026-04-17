package app

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"wlgposter/internal/cache"
	"wlgposter/internal/post"
)

type PostType string

const (
	TypeNew  PostType = "new"
	TypeEdit PostType = "edit"
)

type PostJob struct {
	Post     *post.Post
	PostType PostType
	Plans    []TargetPlan
}

func (a *App) startPostWorker(ctx context.Context) {
	go func() {
		for {
			select {
			case p := <-a.jobs:
				func() {
					defer a.recoverPostJobPanic(p)
					a.processPostJob(p)
				}()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (a *App) processPostJob(p PostJob) {
	a.hydratePostMediaTokens(p.Post, p.Plans)

	errsDownload := p.Post.DownloadAllMedia(a.cfg.TmpDir, a.cfg.FileSizeLimit)
	for _, err := range errsDownload {
		a.log.Error().Err(err).Msg("Telegram media download failed")
	}

	errsMakePost := make([]error, 0, len(p.Plans))
	hasSuccess := false

	for _, plan := range p.Plans {
		success, errs := a.executeTargetPlan(p, plan)
		if success {
			hasSuccess = true
		}
		errsMakePost = append(errsMakePost, errs...)
	}

	if hasSuccess {
		a.persistPostMediaTokens(p.Post, p.Plans)
	}

	errsDelete := p.Post.DeleteAllMedia()
	for _, err := range errsDelete {
		a.log.Error().Err(err).Msg("Telegram media delete failed")
	}

	a.notifyPostErrors(p, errsDownload, errsMakePost, errsDelete)
}

func (a *App) executeTargetPlan(job PostJob, plan TargetPlan) (success bool, errs []error) {
	actionLabel := plan.Target.DisplayName
	defer func() {
		if r := recover(); r != nil {
			panicErr := fmt.Errorf("%s publisher panic during %s: %v", actionLabel, job.PostType, r)
			a.log.Error().
				Int32("tgid", job.Post.MessageID).
				Str("target", string(plan.Target.Name)).
				Str("operation", string(job.PostType)).
				Interface("panic", r).
				Str("stack", string(debug.Stack())).
				Msg("Target plan panicked")
			success = false
			errs = append(errs, panicErr)
		}
	}()

	switch job.PostType {
	case TypeNew:
		platformID, errs := plan.Target.Publisher.Create(job.Post, plan.Options)
		a.logTargetErrors(job, plan, errs)
		if platformID == "" {
			return false, errs
		}

		plan.Target.IDCache.Set(job.Post.MessageID, platformID, plan.Target.Hash(job.Post), job.Post.ChatID, job.Post.AccessHash)
		a.log.Info().
			Int32("tgid", job.Post.MessageID).
			Str("target", string(plan.Target.Name)).
			Str("platform_id", platformID).
			Msg(actionLabel + " post made")
		return true, errs
	case TypeEdit:
		cached := plan.Cached
		if cached.PlatformID == "" {
			latest, ok := plan.Target.IDCache.Get(job.Post.MessageID)
			if ok {
				cached = latest
			}
		}

		if cached.PlatformID == "" {
			a.log.Debug().
				Int32("tgid", job.Post.MessageID).
				Str("target", string(plan.Target.Name)).
				Msg("Edit skipped, target post not created yet")
			return false, nil
		}

		platformID, ok, errs := plan.Target.Publisher.Edit(cached.PlatformID, job.Post, plan.Options)
		a.logTargetErrors(job, plan, errs)
		if !ok {
			return false, errs
		}
		if platformID == "" {
			platformID = cached.PlatformID
		}

		plan.Target.IDCache.Set(job.Post.MessageID, platformID, plan.Target.Hash(job.Post), job.Post.ChatID, job.Post.AccessHash)
		a.log.Info().
			Int32("tgid", job.Post.MessageID).
			Str("target", string(plan.Target.Name)).
			Str("platform_id", platformID).
			Msg(actionLabel + " post edited")
		return true, errs
	default:
		return false, nil
	}
}

func (a *App) recoverPostJobPanic(job PostJob) {
	if r := recover(); r != nil {
		a.log.Error().
			Int32("tgid", job.Post.MessageID).
			Str("operation", string(job.PostType)).
			Interface("panic", r).
			Str("stack", string(debug.Stack())).
			Msg("Post job panicked")
		a.tg.MessageToAdmins(fmt.Sprintf("❌ Паника при обработке поста [%d](%s) (операция %s): %v", job.Post.MessageID, job.Post.Link, job.PostType, r))
	}
}

func (a *App) logTargetErrors(job PostJob, plan TargetPlan, errs []error) {
	actionLabel := plan.Target.DisplayName

	for _, err := range errs {
		a.log.Error().
			Int32("tgid", job.Post.MessageID).
			Str("target", string(plan.Target.Name)).
			Err(err).
			Msg(actionLabel + " post errors")
	}
}

func (a *App) hydratePostMediaTokens(p *post.Post, plans []TargetPlan) {
	for _, media := range p.Media {
		media.NeedsDownload = false
		for _, plan := range plans {
			if plan.Target.GetMediaToken(media) != "" {
				continue
			}
			if entry, ok := plan.Target.MediaCache.Get(media.Type, media.FileId); ok {
				plan.Target.SetMediaToken(media, entry.PlatformToken)
			} else {
				media.NeedsDownload = true
			}
		}
	}
}

func (a *App) persistPostMediaTokens(p *post.Post, plans []TargetPlan) {
	for _, plan := range plans {
		entries := make([]cache.MediaCacheEntry, 0, len(p.Media))
		for _, media := range p.Media {
			if media.FileId == "" || media.Type == "" {
				continue
			}

			token := plan.Target.GetMediaToken(media)
			if token == "" {
				continue
			}

			entries = append(entries, cache.MediaCacheEntry{
				TgFileID:      media.FileId,
				Type:          media.Type,
				PlatformToken: token,
				FileName:      media.FileName,
				Size:          media.Size,
			})
		}

		if len(entries) > 0 {
			plan.Target.MediaCache.SetMany(entries)
		}
	}
}

func (a *App) notifyPostErrors(job PostJob, errsDownload, errsMakePost, errsDelete []error) {
	if len(errsDownload) == 0 && len(errsMakePost) == 0 && len(errsDelete) == 0 {
		return
	}

	errsStr := make([]string, 0, len(errsDownload)+len(errsDelete)+len(errsMakePost))
	for _, err := range errsDownload {
		errsStr = append(errsStr, "• "+err.Error())
	}
	for _, err := range errsDelete {
		errsStr = append(errsStr, "• "+err.Error())
	}
	for _, err := range errsMakePost {
		errsStr = append(errsStr, "• "+err.Error())
	}

	msg := fmt.Sprintf("❌ При обработке поста [%d](%s) произошли ошибки (операция %s):\n%s", job.Post.MessageID, job.Post.Link, job.PostType, strings.Join(errsStr, "\n"))
	a.tg.MessageToAdmins(msg)
}

