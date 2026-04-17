package vk

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"wlgposter/internal/config"
	"wlgposter/internal/post"
	"wlgposter/internal/publisher"
	"wlgposter/internal/utils"

	"github.com/SevereCloud/vksdk/v3/api"
	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
)

type Vk struct {
	cfg    *config.Config
	Client *api.VK
	ctx    context.Context
}

var _ publisher.Client = (*Vk)(nil)

func New(ctx context.Context, cfg *config.Config) *Vk {
	client := api.NewVK(cfg.VkToken)
	return &Vk{
		cfg:    cfg,
		Client: client,
		ctx:    ctx,
	}
}

func (v *Vk) Create(post *post.Post, _ publisher.Options) (string, []error) {
	errs := make([]error, 0)
	attachments := make([]string, 0, len(post.Media))

	for _, media := range post.Media {
		attachment, err := v.addMediaAttachment(media)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		media.VkToken = attachment
		attachments = append(attachments, attachment)
	}

	params := api.Params{
		"owner_id":    fmt.Sprintf("-%d", v.cfg.VkGroupID),
		"message":     RenderText(post),
		"attachments": attachments,
		"from_group":  "1",
	}
	if len(attachments) <= 4 {
		params["primary_attachments_mode"] = "grid"
	}

	wall, err := v.Client.WallPost(params)
	if err != nil {
		errs = append(errs, err)
		// FIXME: debug, remove
		if strings.Contains(err.Error(), "parameters specified") {
			spew.Dump(params)
		}
	}

	if wall.PostID == 0 {
		return "", errs
	}

	return strconv.Itoa(wall.PostID), errs
}

func (v *Vk) Edit(id string, post *post.Post, _ publisher.Options) (string, bool, []error) {
	errs := make([]error, 0)
	attachments := make([]string, 0, len(post.Media))

	for _, media := range post.Media {
		attachment, err := v.addMediaAttachment(media)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		media.VkToken = attachment
		attachments = append(attachments, attachment)
	}

	params := api.Params{
		"owner_id":    fmt.Sprintf("-%d", v.cfg.VkGroupID),
		"post_id":     id,
		"message":     RenderText(post),
		"attachments": attachments,
		"from_group":  "1",
	}
	if len(attachments) <= 4 {
		params["primary_attachments_mode"] = "grid"
	}

	wall, err := v.Client.WallEdit(params)
	if err != nil {
		errs = append(errs, err)
	}

	if wall.PostID == 0 {
		return id, false, errs
	}

	return strconv.Itoa(wall.PostID), true, errs
}

func (v *Vk) Delete(id string) error {
	result, err := v.Client.WallDelete(api.Params{
		"owner_id": fmt.Sprintf("-%d", v.cfg.VkGroupID),
		"post_id":  id,
	})
	if err != nil {
		return err
	}
	if result != 1 {
		return fmt.Errorf("failed to delete post %s", id)
	}
	return nil
}

// return VK attachment string
func (v *Vk) addMediaAttachment(media *post.Media) (string, error) {
	if media.VkToken != "" {
		log.Debug().Str("type", media.Type).Str("file_id", media.FileId).Str("token", media.VkToken).Msg("VK attachment reused from token")
		return media.VkToken, nil
	}

	switch media.Type {
	case "photo":
		photos, err := utils.RetryMediaUpload(
			v.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "vk",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (api.PhotosSaveWallPhotoResponse, error) {
				retryFile, err := os.OpenFile(media.DownloadedPath, os.O_RDONLY, 0)
				if err != nil {
					return nil, err
				}
				defer retryFile.Close()

				return v.Client.UploadGroupWallPhoto(v.cfg.VkGroupID, retryFile)
			},
		)
		if err != nil {
			return "", fmt.Errorf("VK upload photo %s failed: %w", media.DownloadedPath, err)
		}
		if len(photos) == 0 {
			return "", fmt.Errorf("VK upload photo %s failed, empty response", media.DownloadedPath)
		}
		log.Debug().
			Str("path", media.DownloadedPath).
			Str("size", utils.BytesToHuman(media.Size)).
			Str("token", photos[0].ToAttachment()).
			Msg("VK photo uploaded")
		return photos[0].ToAttachment(), nil
	case "video":
		video, err := utils.RetryMediaUpload(
			v.ctx,
			utils.RetryMediaUploadOptions{
				Platform: "vk",
				Type:     media.Type,
				FileName: media.FileName,
				Path:     media.DownloadedPath,
			},
			func() (api.VideoSaveResponse, error) {
				retryFile, err := os.OpenFile(media.DownloadedPath, os.O_RDONLY, 0)
				if err != nil {
					return api.VideoSaveResponse{}, err
				}
				defer retryFile.Close()

				return v.Client.UploadVideo(api.Params{"is_private": "1"}, retryFile)
			},
		)
		if err != nil {
			return "", fmt.Errorf("VK upload video %s failed: %w", media.DownloadedPath, err)
		}
		// TODO: rework to ToAttachment when lib will be fixed
		token := fmt.Sprintf("video%d_%d", video.OwnerID, video.VideoID)
		log.Debug().
			Str("path", media.DownloadedPath).
			Str("size", utils.BytesToHuman(media.Size)).
			Str("token", token).
			Msg("VK video uploaded")
		return token, nil
	case "audio", "voice":
		// FIXME: skip, VK not support it
	}

	return "", fmt.Errorf("VK unsupported media type %s", media.Type)
}
