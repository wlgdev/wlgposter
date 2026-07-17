package max

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"wlgposter/internal/config"
	"wlgposter/internal/post"
	"wlgposter/internal/publisher"
	"wlgposter/internal/utils"

	maxbot "github.com/max-messenger/max-bot-api-client-go/v2"
	"github.com/max-messenger/max-bot-api-client-go/v2/model"
	"github.com/rs/zerolog/log"
)

const (
	TIMEOUT     = 30 // seconds
	MAX_RETRIES = 300
	RETRY_DELAY = 10 * time.Second
)

var sleep = time.Sleep

var _ publisher.Client = (*Max)(nil)

//go:embed ru_root_ca_pem.crt
var certBundle []byte

type Max struct {
	cfg    *config.Config
	Client *maxbot.Api
	ctx    context.Context
}

func New(ctx context.Context, cfg *config.Config) (*Max, error) {
	opts := []maxbot.Opt{
		maxbot.WithHTTPClient(createCustomHttpClient()),
		maxbot.WithPollingTimeout(TIMEOUT * time.Second),
		maxbot.WithPollingPause(TIMEOUT * time.Second),
	}

	client, err := maxbot.NewApi(cfg.MaxBotToken, opts...)
	if err != nil {
		return nil, err
	}

	m := &Max{
		cfg:    cfg,
		Client: client,
		ctx:    ctx,
	}

	return m, nil
}

func createCustomHttpClient() *http.Client {
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	if ok := rootCAs.AppendCertsFromPEM(certBundle); !ok {
		log.Fatal().Msg("failed to append certificates")
	}

	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{
		RootCAs: rootCAs,
	}

	return &http.Client{
		Timeout:   TIMEOUT * time.Second,
		Transport: customTransport,
	}
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
	if result.Success {
		return nil
	}
	return errors.New("failed to delete message")
}

func (m *Max) Create(post *post.Post, opts publisher.Options) (string, []error) {
	return doPost(m, post, opts.ReplyID, func(msg *maxbot.Message) (string, error) {
		result, err := m.Client.Messages.Send(m.ctx, msg)
		if err != nil {
			return "", err
		}
		return result.Message.Body.Mid, nil
	})
}

func (m *Max) Edit(id string, post *post.Post, _ publisher.Options) (string, bool, []error) {
	ok, errs := doPost(m, post, "", func(msg *maxbot.Message) (bool, error) {
		result, err := m.Client.Messages.EditMessage(m.ctx, id, msg.MessageBody())
		return result.Success, err
	})

	return id, ok, errs
}

func doPost[T any](m *Max, post *post.Post, replyMaxMessageID string, action func(msg *maxbot.Message) (T, error)) (T, []error) {
	var zero T
	errs := make([]error, 0)

	msg := maxbot.NewMessage()
	msg.SetFormat(model.FormatHTML)

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
		kb := model.NewKeyboard()
		for _, row := range post.Keyboard {
			r := kb.AddRow()
			for _, button := range row {
				r.AddLink(button.Text, button.URL)
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
		token, err := utils.RetryMediaUpload(
			m.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "max",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (string, error) {
				f, err := os.Open(media.DownloadedPath)
				if err != nil {
					return "", err
				}
				defer f.Close()
				return m.Client.Upload.Upload(m.ctx, model.UploadImage, f, media.FileName, media.Size)
			},
		)
		if err != nil {
			return fmt.Errorf("upload %s %q from %q: %w", media.Type, media.FileName, media.DownloadedPath, err)
		}

		msg.AddAttachByToken(token, model.AttachImage)
		media.MaxToken = token
		log.Debug().Str("path", media.DownloadedPath).Str("size", utils.BytesToHuman(media.Size)).Msg("MAX photo uploaded")
		return nil

	case "video":
		token, err := utils.RetryMediaUpload(
			m.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "max",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (string, error) {
				f, err := os.Open(media.DownloadedPath)
				if err != nil {
					return "", err
				}
				defer f.Close()
				return m.Client.Upload.Upload(m.ctx, model.UploadVideo, f, media.FileName, media.Size)
			},
		)
		if err != nil {
			return fmt.Errorf("upload %s %q from %q: %w", media.Type, media.FileName, media.DownloadedPath, err)
		}

		msg.AddAttachByToken(token, model.AttachVideo)
		media.MaxToken = token
		log.Debug().Str("path", media.DownloadedPath).Str("size", utils.BytesToHuman(media.Size)).Msg("MAX video uploaded")
		return nil

	case "audio", "voice":
		token, err := utils.RetryMediaUpload(
			m.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "max",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (string, error) {
				f, err := os.Open(media.DownloadedPath)
				if err != nil {
					return "", err
				}
				defer f.Close()
				return m.Client.Upload.Upload(m.ctx, model.UploadAudio, f, media.FileName, media.Size)
			},
		)
		if err != nil {
			return fmt.Errorf("upload %s %q from %q: %w", media.Type, media.FileName, media.DownloadedPath, err)
		}

		msg.AddAttachByToken(token, model.AttachAudio)
		media.MaxToken = token
		log.Debug().Str("path", media.DownloadedPath).Str("size", utils.BytesToHuman(media.Size)).Msg("MAX audio uploaded")
		return nil
	}

	return nil
}

func addMediaAttachmentByToken(msg *maxbot.Message, media *post.Media) bool {
	switch media.Type {
	case "photo":
		msg.AddAttachByToken(media.MaxToken, model.AttachImage)
		return true
	case "video":
		msg.AddAttachByToken(media.MaxToken, model.AttachVideo)
		return true
	case "audio", "voice":
		msg.AddAttachByToken(media.MaxToken, model.AttachAudio)
		return true
	default:
		return false
	}
}
