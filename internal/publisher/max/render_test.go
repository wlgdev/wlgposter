package max

import (
	"strings"
	"testing"
	"wlgposter/internal/post"
	"wlgposter/internal/utils"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func TestRenderMarkdown_KeepsBlockquoteAcrossBlankLines(t *testing.T) {
	text := strings.Join([]string{
		"Subnautica 2 выйдет в мае. Игра появится в раннем доступе.",
		"",
		"Решение было принято вскоре после завершения судебных разбирательств между издателем Krafton и бывшими руководителями студии Unknown Worlds.",
		"",
		"Команда добилась значительного прогресса, и работает над запуском раннего доступа в мае.",
		"",
		"Хотя мы не согласны с решением суда, вынесенным на этой неделе, и намерены изучить все юридические аспекты, наша цель - свести к минимуму сбои в работе команды. Мы по-прежнему привержены открытому подходу к разработке, тесно сотрудничаем с сообществом и сосредоточены на том, чтобы предоставить игрокам наилучший возможный опыт.",
	}, "\n")

	quote := strings.Join([]string{
		"Команда добилась значительного прогресса, и работает над запуском раннего доступа в мае.",
		"",
		"Хотя мы не согласны с решением суда, вынесенным на этой неделе, и намерены изучить все юридические аспекты, наша цель - свести к минимуму сбои в работе команды. Мы по-прежнему привержены открытому подходу к разработке, тесно сотрудничаем с сообществом и сосредоточены на том, чтобы предоставить игрокам наилучший возможный опыт.",
	}, "\n")

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

	want := strings.Join([]string{
		"Subnautica 2 выйдет в мае. Игра появится в раннем доступе.",
		"",
		"Решение было принято вскоре после завершения судебных разбирательств между издателем Krafton и бывшими руководителями студии Unknown Worlds.",
		"",
		">Команда добилась значительного прогресса, и работает над запуском раннего доступа в мае.",
		">" + MaxBlockquoteBlankLinePlaceholder,
		">Хотя мы не согласны с решением суда, вынесенным на этой неделе, и намерены изучить все юридические аспекты, наша цель - свести к минимуму сбои в работе команды. Мы по-прежнему привержены открытому подходу к разработке, тесно сотрудничаем с сообществом и сосредоточены на том, чтобы предоставить игрокам наилучший возможный опыт.",
	}, "\n")

	if got := RenderMarkdown(p); got != want {
		t.Fatalf("RenderMarkdown() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderMarkdown_PrefixesEachBlockquoteLine(t *testing.T) {
	text := "Вступление\n\nЭто цитата без пустых строк.\nИ это ее продолжение."
	quote := "Это цитата без пустых строк.\nИ это ее продолжение."
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

	want := "Вступление\n\n>Это цитата без пустых строк.\n>И это ее продолжение."
	if got := RenderMarkdown(p); got != want {
		t.Fatalf("RenderMarkdown() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderMarkdown_PreservesBlankLineAfterBlockquote(t *testing.T) {
	text := "Цитата\n\nОбычный текст"
	quote := "Цитата"
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

	want := ">Цитата\n\n\nОбычный текст"
	if got := RenderMarkdown(p); got != want {
		t.Fatalf("RenderMarkdown() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderMarkdown_StopsBlockquoteWithoutBlankLine(t *testing.T) {
	text := "Цитата\nОбычный текст"
	quote := "Цитата"
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

	want := ">Цитата\n\nОбычный текст"
	if got := RenderMarkdown(p); got != want {
		t.Fatalf("RenderMarkdown() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderMarkdown_BlockquoteWithParagraphsAndFollowingText(t *testing.T) {
	text := strings.Join([]string{
		"Вступление",
		"",
		"Первый абзац цитаты.",
		"",
		"Второй абзац цитаты.",
		"Обычный текст после цитаты.",
	}, "\n")

	quote := strings.Join([]string{
		"Первый абзац цитаты.",
		"",
		"Второй абзац цитаты.",
	}, "\n")

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

	want := strings.Join([]string{
		"Вступление",
		"",
		">Первый абзац цитаты.",
		">" + MaxBlockquoteBlankLinePlaceholder,
		">Второй абзац цитаты.",
		"",
		"Обычный текст после цитаты.",
	}, "\n")
	if got := RenderMarkdown(p); got != want {
		t.Fatalf("RenderMarkdown() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_KeepsBlockquoteBlankLines(t *testing.T) {
	text := strings.Join([]string{
		"Вступление",
		"",
		"Первый абзац цитаты.",
		"",
		"Второй абзац цитаты.",
		"",
		"Обычный текст после цитаты.",
	}, "\n")

	quote := strings.Join([]string{
		"Первый абзац цитаты.",
		"",
		"Второй абзац цитаты.",
	}, "\n")

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

	want := strings.Join([]string{
		"Вступление",
		MaxBlockquoteBlankLinePlaceholder,
		"<blockquote>Первый абзац цитаты.",
		MaxBlockquoteBlankLinePlaceholder,
		"Второй абзац цитаты.</blockquote>",
		MaxBlockquoteBlankLinePlaceholder,
		"Обычный текст после цитаты.",
	}, "\n")

	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_EscapesTextAndFormatsEntities(t *testing.T) {
	text := "A < B & C\nСсылка"
	link := "Ссылка"
	offset := strings.Index(text, link)
	if offset < 0 {
		t.Fatal("link not found")
	}

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityTextURL{
				Offset: int32(utils.UTF16Len(text[:offset])),
				Length: int32(utils.UTF16Len(link)),
				URL:    "https://dev.max.ru?a=1&b=2",
			},
		},
	}

	want := "A &lt; B &amp; C\n<a href=\"https://dev.max.ru?a=1&amp;b=2\">Ссылка</a>"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}

func TestRenderText_PreservesPreNewlines(t *testing.T) {
	text := "before\ncode <tag>\nline 2\nafter"
	code := "code <tag>\nline 2"
	offset := strings.Index(text, code)
	if offset < 0 {
		t.Fatal("code not found")
	}

	p := &post.Post{
		Text: text,
		Entities: []tg.MessageEntity{
			&tg.MessageEntityPre{
				Offset: int32(utils.UTF16Len(text[:offset])),
				Length: int32(utils.UTF16Len(code)),
			},
		},
	}

	want := "before\n<pre>code &lt;tag&gt;\nline 2</pre>\nafter"
	if got := RenderText(p); got != want {
		t.Fatalf("RenderText() mismatch\nwant: %q\ngot:  %q", want, got)
	}
}
