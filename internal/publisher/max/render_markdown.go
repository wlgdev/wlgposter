package max

import (
	"sort"
	"strings"
	"wlgposter/internal/post"
	"wlgposter/internal/utils"

	tg "github.com/amarnathcjd/gogram/telegram"
)

const MaxBlockquoteBlankLinePlaceholder = "\u200B"

type markdownRange struct {
	start  int
	end    int
	open   string
	close  string
	length int
}

func RenderMarkdown(p *post.Post) string {
	if p.Text == "" || len(p.Entities) == 0 {
		return p.Text
	}

	type markdownTag struct {
		index     int
		text      string
		isClosing bool
		length    int
	}

	tags := make([]markdownTag, 0, len(p.Entities)*2)
	for _, ent := range p.Entities {
		ranges := entityToMaxRanges(p.Text, ent)
		for _, r := range ranges {
			tags = append(tags, markdownTag{index: r.start, text: r.open, isClosing: false, length: r.length})
			tags = append(tags, markdownTag{index: r.end, text: r.close, isClosing: true, length: r.length})
		}
	}

	if len(tags) == 0 {
		return p.Text
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
	for _, tag := range tags {
		if tag.index < last {
			tag.index = last
		}
		if tag.index > last {
			b.WriteString(p.Text[last:tag.index])
			last = tag.index
		}
		b.WriteString(tag.text)
	}
	if last < len(p.Text) {
		b.WriteString(p.Text[last:])
	}
	return b.String()
}

func entityToMaxRanges(text string, ent tg.MessageEntity) []markdownRange {
	offset, length, openTag, closeTag, ok := entityToMaxTags(ent)
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

	if _, ok := ent.(*tg.MessageEntityBlockquote); ok {
		return splitBlockquoteRanges(text, startIdx, endIdx, openTag, closeTag)
	}

	return []markdownRange{{
		start:  startIdx,
		end:    endIdx,
		open:   openTag,
		close:  closeTag,
		length: adjustedEnd - adjustedStart,
	}}
}

func entityToMaxTags(ent tg.MessageEntity) (offset int, length int, openTag string, closeTag string, ok bool) {
	switch e := ent.(type) {
	case *tg.MessageEntityBlockquote:
		return int(e.Offset), int(e.Length), ">", "", true
	case *tg.MessageEntityItalic:
		return int(e.Offset), int(e.Length), "_", "_", true
	case *tg.MessageEntityBold:
		return int(e.Offset), int(e.Length), "**", "**", true
	case *tg.MessageEntityStrike:
		return int(e.Offset), int(e.Length), "~~", "~~", true
	case *tg.MessageEntityUnderline:
		return int(e.Offset), int(e.Length), "++", "++", true
	case *tg.MessageEntityCode:
		return int(e.Offset), int(e.Length), "`", "`", true
	case *tg.MessageEntityPre:
		return int(e.Offset), int(e.Length), "`", "`", true
	case *tg.MessageEntityTextURL:
		return int(e.Offset), int(e.Length), "[", "](" + e.URL + ")", true
	default:
		return 0, 0, "", "", false
	}
}

func splitBlockquoteRanges(fullText string, start int, end int, openTag string, closeTag string) []markdownRange {
	text := fullText[start:end]
	ranges := make([]markdownRange, 0, 2)

	for offset := 0; offset < len(text); {
		lineStart := offset
		lineEnd := len(text)
		sepLen := 0

		if newline := strings.IndexByte(text[offset:], '\n'); newline >= 0 {
			lineEnd = offset + newline
			sepLen = 1
		}

		line := text[lineStart:lineEnd]
		open := openTag
		if line == "" {
			open += MaxBlockquoteBlankLinePlaceholder
		}

		ranges = append(ranges, markdownRange{
			start:  start + lineStart,
			end:    start + lineEnd,
			open:   open,
			close:  closeTag,
			length: utils.UTF16Len(line),
		})

		if sepLen == 0 {
			break
		}
		offset = lineEnd + sepLen
	}

	if len(ranges) == 0 {
		return nil
	}

	if end < len(fullText) && fullText[end] == '\n' {
		ranges[len(ranges)-1].close += "\n"
	}

	return ranges
}
