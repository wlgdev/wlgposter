package vk

import (
	"strings"
	"testing"
	"wlgposter/internal/post"
	"wlgposter/internal/utils"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func TestRenderText_EmptyAndNoEntities(t *testing.T) {
	p1 := &post.Post{Text: ""}
	if got := RenderText(p1); got != "" {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", "", got)
	}

	p2 := &post.Post{Text: "Обычный текст"}
	if got := RenderText(p2); got != "Обычный текст\n\n#welovegames" {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", "Обычный текст\n\n#welovegames", got)
	}
}

func TestRenderText_WrapsBlockquoteInQuotes(t *testing.T) {
	text := "Это цитата и все\nДа"
	quote := "Это цитата и все"
	offset := strings.Index(text, quote)
	if offset < 0 {
		t.Fatal("quote not found")
	}

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityBlockquote{
				Offset: int32(utils.UTF16Len(text[:offset])),
				Length: int32(utils.UTF16Len(quote)),
			},
		},
	}

	want := "«Это цитата и все»\nДа\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_TrimsSpacesInBlockquote(t *testing.T) {
	text := "Начало \n \n Цитата с пробелами \n \n Конец"
	quote := " \n Цитата с пробелами \n "
	offset := strings.Index(text, quote)
	if offset < 0 {
		t.Fatal("quote not found")
	}

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityBlockquote{
				Offset: int32(utils.UTF16Len(text[:offset])),
				Length: int32(utils.UTF16Len(quote)),
			},
		},
	}

	want := "Начало \n \n «Цитата с пробелами» \n \n Конец\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_IgnoresOtherEntities(t *testing.T) {
	text := "Жирный текст и цитата"
	bold := "Жирный"
	quote := "цитата"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityBold{
				Offset: int32(utils.UTF16Len(text[:strings.Index(text, bold)])),
				Length: int32(utils.UTF16Len(bold)),
			},
			&tg.MessageEntityBlockquote{
				Offset: int32(utils.UTF16Len(text[:strings.Index(text, quote)])),
				Length: int32(utils.UTF16Len(quote)),
			},
		},
	}

	want := "Жирный текст и «цитата»\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_MultipleBlockquotes(t *testing.T) {
	text := "Первая и вторая"
	q1 := "Первая"
	q2 := "вторая"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityBlockquote{
				Offset: int32(utils.UTF16Len(text[:strings.Index(text, q1)])),
				Length: int32(utils.UTF16Len(q1)),
			},
			&tg.MessageEntityBlockquote{
				Offset: int32(utils.UTF16Len(text[:strings.Index(text, q2)])),
				Length: int32(utils.UTF16Len(q2)),
			},
		},
	}

	want := "«Первая» и «вторая»\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_AppendsSupportedEntityLinksBeforeHashtag(t *testing.T) {
	text := "Анонсировали Serious Sam: Shatterverse, кооперативный роуглайк-шутер.\n\nИгроков ждут Сэмы из различных вселенных, а также большое количество уровней и оружия.\n\nРелиз в 2026 году. Есть в Steam."

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 11,
				URL:    "https://youtu.be/fLVa6K7RTtc",
			},
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 5,
				URL:    "https://store.steampowered.com/app/2067210/Serious_Sam_Shatterverse/",
			},
		},
	}

	want := text + "\n\nYouTube: https://youtu.be/fLVa6K7RTtc\nSteam: https://store.steampowered.com/app/2067210/Serious_Sam_Shatterverse/\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_IgnoresUnsupportedLinks(t *testing.T) {
	text := "Текст поста"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 5,
				URL:    "https://store.steampowered.com/news/app/2067210/view/123",
			},
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 5,
				URL:    "https://example.com/video",
			},
		},
	}

	want := "Текст поста\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_MovesExistingHashtagsToFooter(t *testing.T) {
	p := &post.Post{Text: "Текст поста #игры"}

	want := "Текст поста\n\n#welovegames #игры"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_RestoresExistingHashtagsAfterLinks(t *testing.T) {
	text := "Текст поста #игры"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 5,
				URL:    "https://youtu.be/fLVa6K7RTtc",
			},
		},
	}

	want := "Текст поста\n\nYouTube: https://youtu.be/fLVa6K7RTtc\n\n#welovegames #игры"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_UsesURLTextFromEntityURL(t *testing.T) {
	text := "Смотри https://youtu.be/fLVa6K7RTtc"
	rawURL := "https://youtu.be/fLVa6K7RTtc"
	offset := strings.Index(text, rawURL)
	if offset < 0 {
		t.Fatal("url not found")
	}

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityURL{
				Offset: int32(utils.UTF16Len(text[:offset])),
				Length: int32(utils.UTF16Len(rawURL)),
			},
		},
	}

	want := "Смотри https://youtu.be/fLVa6K7RTtc\n\nYouTube: https://youtu.be/fLVa6K7RTtc\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_NumbersDuplicateLinkLabels(t *testing.T) {
	text := "Ссылки"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 6,
				URL:    "https://youtu.be/fLVa6K7RTtc",
			},
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 6,
				URL:    "https://www.youtube.com/shorts/abc123",
			},
			&tg.MessageEntityTextURL{
				Offset: 0,
				Length: 6,
				URL:    "https://store.steampowered.com/app/2067210/Serious_Sam_Shatterverse/",
			},
		},
	}

	want := text + "\n\nYouTube #1: https://youtu.be/fLVa6K7RTtc\nYouTube #2: https://www.youtube.com/shorts/abc123\nSteam: https://store.steampowered.com/app/2067210/Serious_Sam_Shatterverse/\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_IgnoresRecordOnlyLinksWithoutMarkers(t *testing.T) {
	text := "Обычный пост"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityTextURL{Offset: 0, Length: 6, URL: "https://www.twitch.tv/welovegames"},
			&tg.MessageEntityTextURL{Offset: 0, Length: 6, URL: "https://vk.com/wall-1_1"},
			&tg.MessageEntityTextURL{Offset: 0, Length: 6, URL: "https://www.tiktok.com/@welovegames/video/1"},
			&tg.MessageEntityTextURL{Offset: 0, Length: 6, URL: "https://youtu.be/fLVa6K7RTtc"},
		},
	}

	want := text + "\n\nYouTube: https://youtu.be/fLVa6K7RTtc\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_AddsRecordOnlyLinksForShortsAndParsesBareDomains(t *testing.T) {
	text := "Шортс twitch.tv/welovegames youtube.com/shorts/abc123 vk.com/wall-1_2 tiktok.com/@welovegames/video/3"

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			entityURLFromText(t, text, "twitch.tv/welovegames"),
			entityURLFromText(t, text, "youtube.com/shorts/abc123"),
			entityURLFromText(t, text, "vk.com/wall-1_2"),
			entityURLFromText(t, text, "tiktok.com/@welovegames/video/3"),
		},
	}

	want := text + "\n\nYouTube: youtube.com/shorts/abc123\n\n#welovegames"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_RestoresHashtagsAfterRemovingThemFromBody(t *testing.T) {
	p := &post.Post{Text: "Смотри Шортс #игры #новости"}

	want := "Смотри Шортс\n\n#welovegames #игры #новости"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func entityURLFromText(t *testing.T, text, rawURL string) *tg.MessageEntityURL {
	t.Helper()

	offset := strings.Index(text, rawURL)
	if offset < 0 {
		t.Fatalf("url %q not found in text %q", rawURL, text)
	}

	return &tg.MessageEntityURL{
		Offset: int32(utils.UTF16Len(text[:offset])),
		Length: int32(utils.UTF16Len(rawURL)),
	}
}


