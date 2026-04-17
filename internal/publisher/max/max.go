package max

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
	"wlgposter/internal/config"
	"wlgposter/internal/post"
	"wlgposter/internal/publisher"
	"wlgposter/internal/utils"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"github.com/rs/zerolog/log"
)

const (
	TIMEOUT     = 30 // seconds
	MAX_RETRIES = 300
	RETRY_DELAY = 10 * time.Second
)

var sleep = time.Sleep

var _ publisher.Client = (*Max)(nil)

type Max struct {
	cfg    *config.Config
	Client *maxbot.Api
	ctx    context.Context
}

func New(ctx context.Context, cfg *config.Config) (*Max, error) {
	opts := []maxbot.Option{
		maxbot.WithHTTPClient(&http.Client{Timeout: TIMEOUT * time.Second}),
		maxbot.WithApiTimeout(TIMEOUT * time.Second),
		maxbot.WithPauseTimeout(TIMEOUT * time.Second),
	}

	client, err := maxbot.New(cfg.MaxBotToken, opts...)
	if err != nil {
		return nil, err
	}

	m := &Max{
		cfg:    cfg,
		Client: client,
		ctx:    ctx,
	}
	m.consumeClientErrors()

	return m, nil
}

func (m *Max) consumeClientErrors() {
	errs := m.Client.GetErrors()

	go func() {
		for {
			select {
			case <-m.ctx.Done():
				return
			case err, ok := <-errs:
				if !ok {
					return
				}
				if err == nil || isRetryNoise(err) {
					continue
				}

				log.Warn().Err(err).Msg("MAX client internal error")
			}
		}
	}()
}

func isRetryNoise(err error) bool {
	if err == nil {
		return false
	}

	s := err.Error()
	return strings.Contains(s, "attachment.not.ready") ||
		strings.Contains(s, "errors.process.attachment")
}

func (m *Max) Delete(mid string) error {
	result, err := m.Client.Messages.DeleteMessage(m.ctx, mid)
	if err != nil {
		return err
	}
	if result.Success == true {
		return nil
	}
	return errors.New(result.Message)
}

func (m *Max) Create(post *post.Post, opts publisher.Options) (string, []error) {
	return doPost(m, post, opts.ReplyID, func(msg *maxbot.Message) (string, error) {
		result, err := m.Client.Messages.SendWithResult(m.ctx, msg)
		if err != nil {
			return "", err
		}
		return result.Body.Mid, nil
	})
}

func (m *Max) Edit(id string, post *post.Post, _ publisher.Options) (string, bool, []error) {
	ok, errs := doPost(m, post, "", func(msg *maxbot.Message) (bool, error) {
		err := m.Client.Messages.EditMessage(m.ctx, id, msg)
		return err == nil, err
	})

	return id, ok, errs
}

func doPost[T any](m *Max, post *post.Post, replyMaxMessageID string, action func(msg *maxbot.Message) (T, error)) (T, []error) {
	var zero T
	errs := make([]error, 0)

	msg := maxbot.NewMessage()
	msg.SetFormat(schemes.HTML)

	if m.cfg.ENV == "production" {
		msg.SetChat(m.cfg.MaxTargetChatID)
	} else {
		msg.SetUser(m.cfg.MaxTargetChatID)
	}

	if replyMaxMessageID != "" {
		msg.SetReply(RenderText(post), replyMaxMessageID)
	} else {
		msg.SetText(RenderText(post))
	}

	for _, media := range post.Media {
		if err := m.addMediaAttachment(msg, media); err != nil {
			errs = append(errs, err)
		}
	}

	if len(post.Keyboard) > 0 {
		kb := &maxbot.Keyboard{}
		for _, row := range post.Keyboard {
			r := kb.AddRow()
			for _, button := range row {
				r.AddLink(button.Text, schemes.DEFAULT, button.URL)
			}
		}

		msg.AddKeyboard(kb)
	}

	for attempt := 1; attempt <= MAX_RETRIES; attempt++ {
		result, err := action(msg)

		if err != nil {
			s := err.Error()

			if strings.Contains(s, "attachment.not.ready") || strings.Contains(s, "errors.process.attachment") {
				log.Warn().Msgf("MAX attachment not ready, retry in %s (%d/%d)", RETRY_DELAY, attempt, MAX_RETRIES)

				if attempt >= MAX_RETRIES {
					errs = append(errs, err)
					return zero, errs
				}

				sleep(RETRY_DELAY)
				continue
			} else {
				errs = append(errs, err)
				return zero, errs
			}
		} else {
			return result, errs
		}
	}

	return zero, errs
}

func (m *Max) addMediaAttachment(msg *maxbot.Message, media *post.Media) error {
	if media.MaxToken != "" && addMediaAttachmentByToken(msg, media) {
		log.Debug().Str("type", media.Type).Str("file_id", media.FileId).Str("token", media.MaxToken).Msg("MAX attachment reused from token")
		return nil
	}

	if !media.Downloaded || media.DownloadedPath == "" {
		return nil
	}

	switch media.Type {
	case "photo":
		photo, err := utils.RetryMediaUpload(
			m.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "max",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (*schemes.PhotoTokens, error) {
				return m.Client.Uploads.UploadPhotoFromFile(m.ctx, media.DownloadedPath)
			},
		)
		if err != nil {
			return fmt.Errorf("upload %s %q from %q: %w", media.Type, media.FileName, media.DownloadedPath, err)
		}

		msg.AddPhoto(photo)
		media.MaxToken = getPhotoReuseToken(photo)
		log.Debug().Str("path", media.DownloadedPath).Str("size", utils.BytesToHuman(media.Size)).Msg("MAX photo uploaded")
		return nil

	case "video":
		video, err := utils.RetryMediaUpload(
			m.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "max",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (*schemes.UploadedInfo, error) {
				return m.Client.Uploads.UploadMediaFromFile(m.ctx, schemes.VIDEO, media.DownloadedPath)
			},
		)
		if err != nil {
			return fmt.Errorf("upload %s %q from %q: %w", media.Type, media.FileName, media.DownloadedPath, err)
		}

		msg.AddVideo(video)
		media.MaxToken = video.Token
		log.Debug().Str("path", media.DownloadedPath).Str("size", utils.BytesToHuman(media.Size)).Msg("MAX video uploaded")
		return nil

	case "audio", "voice":
		audio, err := utils.RetryMediaUpload(
			m.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "max",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (*schemes.UploadedInfo, error) {
				return m.Client.Uploads.UploadMediaFromFile(m.ctx, schemes.AUDIO, media.DownloadedPath)
			},
		)
		if err != nil {
			return fmt.Errorf("upload %s %q from %q: %w", media.Type, media.FileName, media.DownloadedPath, err)
		}

		msg.AddAudio(audio)
		media.MaxToken = audio.Token
		log.Debug().Str("path", media.DownloadedPath).Str("size", utils.BytesToHuman(media.Size)).Msg("MAX audio uploaded")
		return nil
	}

	return nil
}

func addMediaAttachmentByToken(msg *maxbot.Message, media *post.Media) bool {
	switch media.Type {
	case "photo":
		msg.AddPhotoByToken(media.MaxToken)
		return true
	case "video":
		msg.AddVideo(&schemes.UploadedInfo{Token: media.MaxToken})
		return true
	case "audio", "voice":
		msg.AddAudio(&schemes.UploadedInfo{Token: media.MaxToken})
		return true
	default:
		return false
	}
}

func getPhotoReuseToken(photo *schemes.PhotoTokens) string {
	if photo == nil || len(photo.Photos) == 0 {
		return ""
	}

	keys := make([]string, 0, len(photo.Photos))
	for key := range photo.Photos {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if token := photo.Photos[key].Token; token != "" {
			return token
		}
	}

	return ""
}
