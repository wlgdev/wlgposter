package telegram

import (
	"context"
	"path/filepath"
	"slices"
	"sort"
	"wlgposter/internal/config"
	"wlgposter/internal/post"
	"wlgposter/internal/utils"

	gogram "github.com/amarnathcjd/gogram/telegram"
	"github.com/rs/zerolog/log"
)

const (
	MEDIA_TYPE_PHOTO = "photo"
	MEDIA_TYPE_VIDEO = "video"
	MEDIA_TYPE_AUDIO = "audio"
	MEDIA_TYPE_VOICE = "voice"
)

type Telegram struct {
	cfg              *config.Config
	Client           *gogram.Client
	albumCollector   *albumCollector
	onPost           func(*post.Post)
	onPrivateMessage func(*gogram.NewMessage)
	onPostEdit       func(*post.Post)
	onError          func(error)
	onCallbackQuery  func(*gogram.CallbackQuery)
}

func New(cfg *config.Config) *Telegram {
	cache := gogram.NewCache(
		filepath.Join(cfg.DBPath, "cache.db"),
		&gogram.CacheConfig{
			MaxSize:  1000,
			Memory:   false,
			Disabled: false,
		},
	)

	client, err := gogram.NewClient(gogram.ClientConfig{
		AppID:    cfg.APIID,
		AppHash:  cfg.APIHash,
		Session:  filepath.Join(cfg.DBPath, "session.dat"),
		Cache:    cache,
		LogLevel: gogram.NoLevel,
		ErrorHandler: func(err error) bool {
			log.Error().Err(err).Msg("Telegram error")
			return false
		},
	})

	if err != nil {
		log.Error().Err(err).Msg("Telegram init failed")
	}

	t := &Telegram{
		cfg:    cfg,
		Client: client,
	}

	t.albumCollector = newAlbumCollector(t)

	return t
}

func (t *Telegram) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		if _, err := t.Client.Conn(); err != nil {
			log.Error().Err(err).Msg("Telegram connect failed")
			return
		}
		if err := t.Client.LoginBot(t.cfg.TelegramBotToken); err != nil {
			log.Error().Err(err).Msg("Telegram login failed")
			return
		}

		t.Client.On("", func(message *gogram.NewMessage) error {
			if t.onPrivateMessage != nil && message.Message != nil && message.Message.GroupedID == 0 && message.IsPrivate() {
				t.onPrivateMessage(message)
			}
			return nil
		}, gogram.FilterPrivate, gogram.FromUser(t.cfg.TelegramAdmins...))

		t.Client.On(gogram.OnAlbum, func(album *gogram.Album) error {
			if t.onPrivateMessage != nil && album.Messages[0].IsPrivate() {
				t.onPrivateMessage(album.Messages[0])
			}
			return nil
		}, gogram.FilterPrivate, gogram.FromUser(t.cfg.TelegramAdmins...))

		t.Client.On(gogram.OnCallbackQuery, func(callback *gogram.CallbackQuery) error {
			if t.onCallbackQuery != nil {
				t.onCallbackQuery(callback)
			}
			return nil
		}, gogram.FilterPrivate, gogram.FromUser(t.cfg.TelegramAdmins...))

		t.Client.On(gogram.OnMessage, func(message *gogram.NewMessage) error {
			if message.Message.GroupedID != 0 {
				t.albumCollector.push(message, albumEventNew)
				return nil
			}
			post := t.handleMessageAsPost(message)
			if post != nil && t.onPost != nil {
				t.onPost(post)
			}
			return nil
		}, gogram.FilterChannel)

		t.Client.On(gogram.OnEditMessage, func(message *gogram.NewMessage) error {
			if message.Message != nil && message.Message.GroupedID != 0 {
				t.albumCollector.push(message, albumEventEdit)
				return nil
			}
			post := t.handleEditMessageAsPost(message)
			if post != nil && t.onPostEdit != nil {
				t.onPostEdit(post)
			}
			return nil
		}, gogram.FilterChannel)

		log.Info().Msg("Telegram is running")

		<-ctx.Done()
		if err := t.Client.Stop(); err != nil {
			log.Error().Err(err).Msg("Telegram stop failed")
		}
	}()
}

func (t *Telegram) Stop() {
	_ = t.Client.Stop()
}

func (t *Telegram) adminMiddleware(next gogram.MessageHandler) gogram.MessageHandler {
	return func(ctx *gogram.NewMessage) error {
		if !slices.Contains(t.cfg.TelegramAdmins, ctx.ChatID()) {
			return nil
		}
		return next(ctx)
	}
}

func (t *Telegram) OnPost(fn func(*post.Post)) {
	t.onPost = fn
}

func (t *Telegram) OnPostEdit(fn func(*post.Post)) {
	t.onPostEdit = fn
}

func (t *Telegram) OnError(fn func(error)) {
	t.onError = fn
}

func (t *Telegram) emitError(err error) {
	if err != nil && t.onError != nil {
		t.onError(err)
	}
}

func (t *Telegram) OnPrivateMessage(fn func(*gogram.NewMessage)) {
	t.onPrivateMessage = fn
}

func (t *Telegram) OnCallbackQuery(fn func(*gogram.CallbackQuery)) {
	t.onCallbackQuery = fn
}

func (t *Telegram) MessageToAdmins(text string) {
	for _, admin := range t.cfg.TelegramAdmins {
		t.Client.SendMessage(admin, text, &gogram.SendOptions{
			LinkPreview: false,
			ParseMode:   "markdown",
		})
	}
}

func (t *Telegram) handleMessageAsPost(message *gogram.NewMessage) *post.Post {
	m := make([]*post.Media, 0)
	msg := message

	if msg.Photo() != nil || msg.Video() != nil || msg.Audio() != nil || msg.Voice() != nil {
		if media := msg.Media(); media != nil {
			t := getMediaType(msg)
			round := false

			if v, ok := msg.Media().(*gogram.MessageMediaDocument); ok {
				round = v.Round
			}

			m = append(m, &post.Media{
				Type:       t,
				FileId:     msg.File.FileID,
				FileName:   msg.File.Name,
				Size:       msg.File.Size,
				Ext:        msg.File.Ext,
				RoundVideo: round,
				DownloadFn: func(path string) (string, error) {
					p, err := msg.Download(&gogram.DownloadOptions{FileName: path})
					if err != nil {
						return "", err
					}

					log.Debug().
						Str("path", path).
						Str("type", t).
						Str("file_id", msg.File.FileID).
						Str("size", utils.BytesToHuman(msg.File.Size)).
						Str("name", msg.File.Name).
						Msgf("Telegram %s downloaded", t)

					return p, nil
				},
			})
		}
	}

	post := &post.Post{
		ChatID:         message.Chat.ID,
		AccessHash:     message.Channel.AccessHash,
		MessageID:      message.ID,
		ReplyMessageID: message.ReplyID(),
		Link:           message.Link(),
		Text:           message.Text(),
		IsForward:      message.IsForward(),
		Entities:       message.Message.Entities,
		Media:          m,
		Keyboard:       getKeyboard(message),
	}

	return post
}

func (t *Telegram) handleEditMessageAsPost(message *gogram.NewMessage) *post.Post {
	if message == nil || message.Message == nil {
		return nil
	}

	if message.Message.GroupedID == 0 {
		return t.handleMessageAsPost(message)
	}

	group, err := message.GetMediaGroup()
	if err != nil {
		log.Error().
			Err(err).
			Int32("tgid", message.ID).
			Int64("grouped_id", message.Message.GroupedID).
			Msg("Telegram media group fetch failed on edit")
		return nil
	}

	sort.Slice(group, func(i, j int) bool {
		return group[i].ID < group[j].ID
	})

	albumMessages := make([]*gogram.NewMessage, 0, len(group))
	for i := range group {
		msg := group[i]
		albumMessages = append(albumMessages, &msg)
	}

	return t.handleAlbumAsPost(&gogram.Album{
		Client:    message.Client,
		GroupedID: message.Message.GroupedID,
		Messages:  albumMessages,
	})
}

func (t *Telegram) handleAlbumAsPost(album *gogram.Album) *post.Post {
	if len(album.Messages) == 0 {
		return nil
	}

	messages := append([]*gogram.NewMessage(nil), album.Messages...)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].ID < messages[j].ID
	})

	m := make([]*post.Media, 0)
	var text string
	var entities []gogram.MessageEntity
	canonicalMessageID := messages[0].ID
	primaryMessage := messages[0]

	for _, message := range messages {
		msg := message
		t := msg.Text()
		if t != "" && text == "" {
			text = t
			if len(msg.Message.Entities) > 0 {
				entities = msg.Message.Entities
			}
		}

		if msg.Photo() != nil || msg.Video() != nil || msg.Audio() != nil || msg.Voice() != nil {
			if media := msg.Media(); media != nil {
				t := getMediaType(msg)

				round := false
				if doc, ok := msg.Media().(*gogram.MessageMediaDocument); ok {
					round = doc.Round
				}

				m = append(m, &post.Media{
					Type:       t,
					FileId:     msg.File.FileID,
					FileName:   msg.File.Name,
					Size:       msg.File.Size,
					Ext:        msg.File.Ext,
					RoundVideo: round,
					DownloadFn: func(path string) (string, error) {
						p, err := msg.Download(&gogram.DownloadOptions{FileName: path})
						if err != nil {
							return "", err
						}

						log.Debug().
							Str("path", path).
							Str("type", t).
							Str("size", utils.BytesToHuman(msg.File.Size)).
							Msg("Telegram media downloaded")
						return p, nil
					},
				})
			}
		}
	}

	var accessHash int64 = 0
	if primaryMessage.Channel != nil {
		accessHash = primaryMessage.Channel.AccessHash
	}

	post := &post.Post{
		ChatID:         primaryMessage.Chat.ID,
		AccessHash:     accessHash,
		MessageID:      canonicalMessageID,
		ReplyMessageID: primaryMessage.ReplyID(),
		Link:           primaryMessage.Link(),
		Text:           text,
		IsForward:      primaryMessage.IsForward(),
		Entities:       entities,
		Media:          m,
		Keyboard:       getKeyboard(primaryMessage),
	}

	return post
}

// Copy only URL buttons
func getKeyboard(message *gogram.NewMessage) [][]*gogram.KeyboardButtonURL {
	keyboard := make([][]*gogram.KeyboardButtonURL, 0)
	if markup, ok := message.Message.ReplyMarkup.(*gogram.ReplyInlineMarkup); ok && markup != nil {
		for _, row := range markup.Rows {
			if len(row.Buttons) == 0 {
				continue
			}

			r := make([]*gogram.KeyboardButtonURL, 0)
			for _, button := range row.Buttons {
				if b, ok := button.(*gogram.KeyboardButtonURL); ok {
					r = append(r, b)
				}
			}

			if len(r) > 0 {
				keyboard = append(keyboard, r)
			}
		}
	}

	return keyboard
}

func getMediaType(message *gogram.NewMessage) string {
	if message.Photo() != nil {
		return MEDIA_TYPE_PHOTO
	}
	if message.Video() != nil {
		return MEDIA_TYPE_VIDEO
	}
	if message.Audio() != nil {
		return MEDIA_TYPE_AUDIO
	}
	if message.Voice() != nil {
		return MEDIA_TYPE_VOICE
	}
	return ""
}
