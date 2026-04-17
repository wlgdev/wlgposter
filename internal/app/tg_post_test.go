package app

import (
	"bytes"
	"strings"
	"testing"
	"wlgposter/internal/cache"
	"wlgposter/internal/config"
	"wlgposter/internal/post"

	"github.com/rs/zerolog"
)

type fakeBannedWordsStore struct {
	word cache.BannedWord
	ok   bool
}

func (f fakeBannedWordsStore) List() ([]cache.BannedWord, error) {
	if !f.ok {
		return nil, nil
	}

	return []cache.BannedWord{f.word}, nil
}

func (f fakeBannedWordsStore) Add(word string, platformVK bool, platformMax bool) error {
	return nil
}

func (f fakeBannedWordsStore) Delete(word string) (bool, error) {
	return false, nil
}

func (f fakeBannedWordsStore) Contains(text string) (cache.BannedWord, bool) {
	if f.ok && strings.Contains(text, f.word.Word) {
		return f.word, true
	}

	return cache.BannedWord{}, false
}

type fakeIDCacheStore struct {
	entries map[int32]cache.IDCacheEntry
}

func (f *fakeIDCacheStore) Get(tgID int32) (cache.IDCacheEntry, bool) {
	if f.entries == nil {
		return cache.IDCacheEntry{}, false
	}

	entry, ok := f.entries[tgID]
	return entry, ok
}

func (f *fakeIDCacheStore) Set(tgID int32, platformID string, hash string, channelID int64, accessHash int64) {
	if f.entries == nil {
		f.entries = make(map[int32]cache.IDCacheEntry)
	}

	f.entries[tgID] = cache.IDCacheEntry{
		TgID:       tgID,
		PlatformID: platformID,
		Hash:       hash,
		ChannelID:  channelID,
		AccessHash: accessHash,
	}
}

func (f *fakeIDCacheStore) Delete(tgID int32) {
	delete(f.entries, tgID)
}

func (f *fakeIDCacheStore) ListRecent(limit int) []cache.IDCacheEntry {
	return nil
}

func TestOnTelegramPost_LogsPerTargetBanwordSkipAndKeepsAllowedTarget(t *testing.T) {
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).Level(zerolog.InfoLevel)

	maxCache := &fakeIDCacheStore{}
	vkCache := &fakeIDCacheStore{}

	app := &App{
		cfg: &config.Config{
			TelegramTargetChannelID: []int64{100},
		},
		log: logger,
		bannedWords: fakeBannedWordsStore{
			word: cache.BannedWord{
				Word:        "cache",
				PlatformVK:  true,
				PlatformMax: false,
			},
			ok: true,
		},
		targets: []*Target{
			{
				Name:        TargetMax,
				DisplayName: "MAX",
				IDCache:     maxCache,
				Hash: func(*post.Post) string {
					return "max-hash"
				},
			},
			{
				Name:        TargetVK,
				DisplayName: "VK",
				IDCache:     vkCache,
				Hash: func(*post.Post) string {
					return "vk-hash"
				},
			},
		},
		jobs: make(chan PostJob, 1),
	}

	app.onTelegramPost(&post.Post{
		ChatID:     100,
		MessageID:  42,
		Text:       "telegram cache post",
		AccessHash: 777,
	})

	select {
	case job := <-app.jobs:
		if len(job.Plans) != 1 {
			t.Fatalf("expected 1 plan, got %d", len(job.Plans))
		}
		if job.Plans[0].Target.Name != TargetMax {
			t.Fatalf("expected MAX target, got %s", job.Plans[0].Target.Name)
		}
	default:
		t.Fatal("expected job to be enqueued")
	}

	if _, ok := maxCache.Get(42); !ok {
		t.Fatal("expected MAX cache entry to be created")
	}
	if _, ok := vkCache.Get(42); ok {
		t.Fatal("expected VK cache entry to be skipped")
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, `"message":"Telegram post skipped for target by banned word"`) {
		t.Fatalf("expected skip log, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"target":"vk"`) {
		t.Fatalf("expected VK target in log, got %s", logOutput)
	}
	if !strings.Contains(logOutput, `"type":"new"`) {
		t.Fatalf("expected new post type in log, got %s", logOutput)
	}
}

func TestOnTelegramPostEdit_IgnoresWithoutCacheWhenNoTargetsAreActive(t *testing.T) {
	var logBuf bytes.Buffer
	logger := zerolog.New(&logBuf).Level(zerolog.DebugLevel)

	app := &App{
		cfg: &config.Config{
			TelegramTargetChannelID: []int64{100},
		},
		log:         logger,
		bannedWords: fakeBannedWordsStore{},
		targets: []*Target{
			{
				Name:        TargetMax,
				DisplayName: "MAX",
				IDCache:     &fakeIDCacheStore{},
				Hash: func(*post.Post) string {
					return "max-hash"
				},
				CanPublish: func(*post.Post) bool {
					return false
				},
			},
			{
				Name:        TargetVK,
				DisplayName: "VK",
				IDCache:     &fakeIDCacheStore{},
				Hash: func(*post.Post) string {
					return "vk-hash"
				},
				CanPublish: func(*post.Post) bool {
					return false
				},
			},
		},
		jobs: make(chan PostJob, 1),
	}

	app.onTelegramPostEdit(&post.Post{
		ChatID:     100,
		MessageID:  77,
		Text:       "callback noise",
		AccessHash: 777,
	})

	select {
	case job := <-app.jobs:
		t.Fatalf("expected no edit job, got %+v", job)
	default:
	}

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "Failed edit post, cache miss for both platforms") {
		t.Fatalf("expected edit without active targets to be ignored, got %s", logOutput)
	}
	if strings.Contains(logOutput, "Telegram post edit skipped for all active targets") {
		t.Fatalf("expected no full-skip edit log, got %s", logOutput)
	}
}
