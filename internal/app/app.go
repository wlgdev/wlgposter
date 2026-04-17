package app

import (
	"context"
	"path/filepath"
	"wlgposter/internal/cache"
	"wlgposter/internal/config"
	"wlgposter/internal/post"
	"wlgposter/internal/publisher"
	"wlgposter/internal/publisher/max"
	"wlgposter/internal/publisher/vk"
	"wlgposter/internal/telegram"
	"wlgposter/internal/utils"

	"github.com/rs/zerolog"
)

const (
	ID_CACHE_MAX_SIZE    = 500
	MEDIA_CACHE_MAX_SIZE = 1000
)

type idCacheStore interface {
	Get(tgID int32) (cache.IDCacheEntry, bool)
	Set(tgID int32, platformID string, hash string, channelID int64, accessHash int64)
	Delete(tgID int32)
	ListRecent(limit int) []cache.IDCacheEntry
}

type mediaCacheStore interface {
	Get(mediaType, tgFileID string) (cache.MediaCacheEntry, bool)
	SetMany(entries []cache.MediaCacheEntry)
	Set(mediaType, tgFileID, platformToken, fileName string, size int64)
	Delete(mediaType, tgFileID string)
}

type bannedWordsStore interface {
	List() ([]cache.BannedWord, error)
	Add(word string, platformVK bool, platformMax bool) error
	Delete(word string) (bool, error)
	Contains(text string) (cache.BannedWord, bool)
}

type App struct {
	cfg           *config.Config
	log           zerolog.Logger
	db            *cache.DB
	maxIDCache    idCacheStore
	maxMediaCache mediaCacheStore
	vkIDCache     idCacheStore
	vkMediaCache  mediaCacheStore
	bannedWords   bannedWordsStore
	tg            *telegram.Telegram
	targets       []*Target
	jobs          chan PostJob
}

func New(ctx context.Context, cfg *config.Config, log zerolog.Logger) *App {
	db, err := cache.NewDB(filepath.Join(cfg.DBPath, "kv.db"))
	if err != nil {
		log.Fatal().Err(err).Msg("DB init failed")
	}

	maxIDCache := cache.NewIDCacheDB(ID_CACHE_MAX_SIZE, db, "Max")
	maxMediaCache := cache.NewMediaCacheDB(MEDIA_CACHE_MAX_SIZE, db, "Max")
	vkIDCache := cache.NewIDCacheDB(ID_CACHE_MAX_SIZE, db, "Vk")
	vkMediaCache := cache.NewMediaCacheDB(MEDIA_CACHE_MAX_SIZE, db, "Vk")

	bannedWords, err := cache.NewBannedWordsDB(db)
	if err != nil {
		log.Fatal().Err(err).Msg("Banned words init failed")
	}

	tg := telegram.New(cfg)

	var mx *max.Max
	if cfg.MaxBotToken != "" {
		var err error
		mx, err = max.New(ctx, cfg)
		if err != nil {
			log.Fatal().Err(err).Msg("MAX init failed")
		} else {
			log.Info().Msg("MAX init success")
		}
	}

	var vkClient *vk.Vk
	if cfg.VkToken != "" {
		vkClient = vk.New(ctx, cfg)
		log.Info().Msg("VK init success")
	}

	targets := make([]*Target, 0, 2)
	if mx != nil {
		targets = append(targets, &Target{
			Name:        TargetMax,
			DisplayName: "MAX",
			IDCache:     maxIDCache,
			MediaCache:  maxMediaCache,
			Hash:        max.Hash,
			Publisher:   publisher.Client(mx),
			CanPublish:  max.CanPublish,
			GetMediaToken: func(m *post.Media) string {
				return m.MaxToken
			},
			SetMediaToken: func(m *post.Media, token string) {
				m.MaxToken = token
			},
		})
	}
	if vkClient != nil {
		targets = append(targets, &Target{
			Name:        TargetVK,
			DisplayName: "VK",
			IDCache:     vkIDCache,
			MediaCache:  vkMediaCache,
			Hash:        vk.Hash,
			Publisher:   publisher.Client(vkClient),
			CanPublish:  vk.CanPublish,
			GetMediaToken: func(m *post.Media) string {
				return m.VkToken
			},
			SetMediaToken: func(m *post.Media, token string) {
				m.VkToken = token
			},
		})
	}

	jobs := make(chan PostJob, 100)

	return &App{
		cfg:           cfg,
		log:           log,
		db:            db,
		maxIDCache:    maxIDCache,
		maxMediaCache: maxMediaCache,
		vkIDCache:     vkIDCache,
		vkMediaCache:  vkMediaCache,
		bannedWords:   bannedWords,
		tg:            tg,
		targets:       targets,
		jobs:          jobs,
	}
}

func (a *App) Run(ctx context.Context) {
	go utils.DailyClearDirectory(ctx, a.cfg.TmpDir, 3)

	a.tg.OnPost(a.onTelegramPost)
	a.tg.OnPostEdit(a.onTelegramPostEdit)
	a.tg.OnError(a.onTelegramError)
	a.tg.OnPrivateMessage(a.onAdminMessage)

	a.startPostWorker(ctx)
	a.tg.Run(ctx)
	a.startTelegramDeleteWatcher(ctx)

	<-ctx.Done()
}
