package post

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"wlgposter/internal/utils"

	tg "github.com/amarnathcjd/gogram/telegram"
)

type Post struct {
	ChatID         int64
	AccessHash     int64
	MessageID      int32
	ReplyMessageID int32
	Link           string
	Text           string
	IsForward      bool
	Entities       []tg.MessageEntity
	Media          []*Media
	Keyboard       [][]*tg.KeyboardButtonURL
}

func (p *Post) HashText(text string) string {
	h := md5.New()
	h.Write([]byte(text))
	p.writeMediaIDs(h)

	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Post) SourceHash() string {
	h := md5.New()
	h.Write([]byte(p.Text))
	p.writeEntities(h)
	p.writeMediaIDs(h)

	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Post) writeMediaIDs(h interface{ Write([]byte) (int, error) }) {
	mediaIDs := make([]string, 0, len(p.Media))
	for _, m := range p.Media {
		mediaIDs = append(mediaIDs, m.FileId)
	}
	sort.Strings(mediaIDs)

	for _, id := range mediaIDs {
		h.Write([]byte(id))
	}
}

func (p *Post) writeEntities(h interface{ Write([]byte) (int, error) }) {
	for _, ent := range p.Entities {
		fmt.Fprintf(h, "%T|%+v", ent, ent)
	}
}

type Media struct {
	Type           string // "photo", "video", "audio", "voice"
	FileId         string
	FileName       string
	Size           int64
	Ext            string
	RoundVideo     bool
	MaxToken       string
	VkToken        string
	NeedsDownload  bool
	DownloadFn     func(path string) (string, error)
	Downloaded     bool
	DownloadedPath string
}

func (p *Post) DownloadAllMedia(dirPath string, sizeLimitMB int64) []error {
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	errs := make([]error, 0, len(p.Media))

	sizeLimit := sizeLimitMB * 1024 * 1024

	for _, media := range p.Media {
		if media.Downloaded || !media.NeedsDownload {
			continue
		}
		if media.Size > sizeLimit {
			mu.Lock()
			errs = append(errs, fmt.Errorf("%s %s: size is too large (%s)", media.Type, media.FileName, utils.BytesToHuman(media.Size)))
			mu.Unlock()
			continue
		}
		if media.DownloadFn == nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("%s %s: DownloadFn is nil", media.Type, media.FileName))
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(m *Media) {
			defer wg.Done()

			// FIXME: hack for Max, it only supports ogg, mp3
			fileName := strings.ReplaceAll(m.FileName, ".oga", ".ogg")
			fileName = strings.ReplaceAll(fileName, ".spx", ".ogg")

			target := filepath.Join(dirPath, fileName)
			mediaPath, err := m.DownloadFn(target)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			m.Downloaded = true
			m.DownloadedPath = mediaPath
		}(media)
	}

	wg.Wait()

	return errs
}

func (p *Post) DeleteAllMedia() []error {
	errs := make([]error, 0)

	for _, media := range p.Media {
		if media.Downloaded && media.DownloadedPath != "" {
			err := os.Remove(media.DownloadedPath)
			if err != nil {
				errs = append(errs, err)
			}

		}
	}

	return errs
}
