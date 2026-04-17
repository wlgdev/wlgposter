package max

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"wlgposter/internal/post"
	"wlgposter/internal/utils"

	tg "github.com/amarnathcjd/gogram/telegram"
)

type htmlRange struct {
	start  int
	end    int
	open   string
	close  string
	length int
	kind   htmlTagKind
}

type htmlTagKind int

const (
	htmlTagKindGeneric htmlTagKind = iota
	htmlTagKindPre
)

func Hash(p *post.Post) string {
	return p.HashText(RenderText(p))
}

func RenderText(p *post.Post) string {
	if p.Text == "" {
		return ""
	}

	type htmlTag struct {
		index     int
		text      string
		isClosing bool
		length    int
		kind      htmlTagKind
	}

	tags := make([]htmlTag, 0, len(p.Entities)*2)
	for _, ent := range p.Entities {
		ranges := entityToMaxHTMLRanges(p.Text, ent)
		for _, r := range ranges {
			tags = append(tags, htmlTag{index: r.start, text: r.open, isClosing: false, length: r.length, kind: r.kind})
			tags = append(tags, htmlTag{index: r.end, text: r.close, isClosing: true, length: r.length, kind: r.kind})
		}
	}

	for i := range tags {
		if tags[i].index < 0 {
			tags[i].index = 0
		}
		if tags[i].index > len(p.Text) {
			tags[i].index = len(p.Text)
		}
	}

	sort.SliceStable(tags, func(i, j int) bool {
		if tags[i].index != tags[j].index {
			return tags[i].index < tags[j].index
		}
		if tags[i].isClosing != tags[j].isClosing {
			return !tags[i].isClosing
		}
		if tags[i].isClosing {
			if tags[i].length != tags[j].length {
				return tags[i].length < tags[j].length
			}
			return false
		}
		if tags[i].length != tags[j].length {
			return tags[i].length > tags[j].length
		}
		return false
	})

	extra := 0
	for _, tag := range tags {
		extra += len(tag.text)
	}

	var b strings.Builder
	b.Grow(len(p.Text) + extra)

	last := 0
	preDepth := 0
	for _, tag := range tags {
		if tag.index < last {
			tag.index = last
		}
		if tag.index > last {
			writeMaxHTMLText(&b, p.Text[last:tag.index], preDepth > 0)
			last = tag.index
		}

		b.WriteString(tag.text)
		if tag.kind == htmlTagKindPre {
			if tag.isClosing {
				if preDepth > 0 {
					preDepth--
				}
			} else {
				preDepth++
			}
		}
	}
	if last < len(p.Text) {
		writeMaxHTMLText(&b, p.Text[last:], preDepth > 0)
	}

	return b.String()
}

func entityToMaxHTMLRanges(text string, ent tg.MessageEntity) []htmlRange {
	offset, length, openTag, closeTag, kind, ok := entityToMaxHTMLTags(ent)
	if !ok || length <= 0 {
		return nil
	}

	entityText := utils.UTF16Slice(text, offset, offset+length)
	leading := utils.CountLeadingSpacesUTF16(entityText)
	trailing := utils.CountTrailingSpacesUTF16(entityText)
	if leading >= length {
		return nil
	}

	adjustedStart := offset + leading
	adjustedEnd := offset + length - trailing
	if adjustedEnd <= adjustedStart {
		return nil
	}

	startIdx := utils.UTF16IndexToByteIndex(text, adjustedStart)
	endIdx := utils.UTF16IndexToByteIndex(text, adjustedEnd)
	if startIdx >= endIdx {
		return nil
	}

	return []htmlRange{{
		start:  startIdx,
		end:    endIdx,
		open:   openTag,
		close:  closeTag,
		length: adjustedEnd - adjustedStart,
		kind:   kind,
	}}
}

func entityToMaxHTMLTags(ent tg.MessageEntity) (offset int, length int, openTag string, closeTag string, kind htmlTagKind, ok bool) {
	switch e := ent.(type) {
	case *tg.MessageEntityBlockquote:
		return int(e.Offset), int(e.Length), "<blockquote>", "</blockquote>", htmlTagKindGeneric, true
	case *tg.MessageEntityItalic:
		return int(e.Offset), int(e.Length), "<i>", "</i>", htmlTagKindGeneric, true
	case *tg.MessageEntityBold:
		return int(e.Offset), int(e.Length), "<b>", "</b>", htmlTagKindGeneric, true
	case *tg.MessageEntityStrike:
		return int(e.Offset), int(e.Length), "<del>", "</del>", htmlTagKindGeneric, true
	case *tg.MessageEntityUnderline:
		return int(e.Offset), int(e.Length), "<ins>", "</ins>", htmlTagKindGeneric, true
	case *tg.MessageEntityCode:
		return int(e.Offset), int(e.Length), "<code>", "</code>", htmlTagKindGeneric, true
	case *tg.MessageEntityPre:
		return int(e.Offset), int(e.Length), "<pre>", "</pre>", htmlTagKindPre, true
	case *tg.MessageEntityTextURL:
		return int(e.Offset), int(e.Length), fmt.Sprintf(`<a href="%s">`, html.EscapeString(e.URL)), "</a>", htmlTagKindGeneric, true
	default:
		return 0, 0, "", "", htmlTagKindGeneric, false
	}
}

func writeMaxHTMLText(b *strings.Builder, text string, inPre bool) {
	if text == "" {
		return
	}

	if inPre {
		b.WriteString(html.EscapeString(text))
		return
	}

	start := 0
	prevWasNewline := false
	for i, r := range text {
		if r != '\n' {
			prevWasNewline = false
			continue
		}

		if start < i {
			b.WriteString(html.EscapeString(text[start:i]))
		}
		if prevWasNewline {
			b.WriteString(MaxBlockquoteBlankLinePlaceholder)
		}
		b.WriteString("\n")
		start = i + 1
		prevWasNewline = true
	}

	if start < len(text) {
		b.WriteString(html.EscapeString(text[start:]))
	}
}
