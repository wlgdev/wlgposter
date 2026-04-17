package vk

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
	"wlgposter/internal/post"
	"wlgposter/internal/utils"

	tg "github.com/amarnathcjd/gogram/telegram"
)

func Hash(p *post.Post) string {
	return p.HashText(RenderText(p))
}

func RenderText(p *post.Post) string {
	if p.Text == "" {
		return ""
	}

	body := renderVKBody(p.Text, p.Entities)
	body, hashtags := stripVKHashtags(body)

	return buildVKText(body, collectVKEntityLinks(p.Text, p.Entities), hashtags)
}

func renderVKBody(text string, entities []tg.MessageEntity) string {
	if text == "" {
		return ""
	}

	type textTag struct {
		index     int
		text      string
		isClosing bool
		length    int
	}

	var tags []textTag
	for _, ent := range entities {
		ranges := entityToVKRanges(text, ent)
		for _, r := range ranges {
			tags = append(tags, textTag{index: r.start, text: r.open, isClosing: false, length: r.length})
			tags = append(tags, textTag{index: r.end, text: r.close, isClosing: true, length: r.length})
		}
	}

	for i := range tags {
		if tags[i].index < 0 {
			tags[i].index = 0
		}
		if tags[i].index > len(text) {
			tags[i].index = len(text)
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
	b.Grow(len(text) + extra)

	last := 0
	for _, tag := range tags {
		if tag.index < last {
			tag.index = last
		}
		if tag.index > last {
			b.WriteString(text[last:tag.index])
			last = tag.index
		}

		b.WriteString(tag.text)
	}

	if last < len(text) {
		b.WriteString(text[last:])
	}

	return b.String()
}

type vkEntityLink struct {
	label string
	url   string
}

func buildVKText(body string, links []string, hashtags []string) string {
	hashtags = filterVKFooterHashtags(hashtags)

	sections := make([]string, 0, 3)
	if strings.TrimSpace(body) != "" {
		sections = append(sections, strings.TrimRightFunc(body, unicode.IsSpace))
	}
	if len(links) > 0 {
		sections = append(sections, strings.Join(links, "\n"))
	}

	footerTags := make([]string, 0, len(hashtags)+1)
	footerTags = append(footerTags, "#welovegames")
	footerTags = append(footerTags, hashtags...)
	sections = append(sections, strings.Join(footerTags, " "))

	return strings.Join(sections, "\n\n")
}

func filterVKFooterHashtags(hashtags []string) []string {
	filtered := make([]string, 0, len(hashtags))
	for _, hashtag := range hashtags {
		if strings.EqualFold(hashtag, "#welovegames") {
			continue
		}
		filtered = append(filtered, hashtag)
	}

	return filtered
}

func stripVKHashtags(text string) (string, []string) {
	type hashtagMatch struct {
		start int
		end   int
		tag   string
	}

	matches := make([]hashtagMatch, 0)
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if r != '#' || !isVKHashtagBoundary(text, i) {
			i += size
			continue
		}

		end := i + size
		for end < len(text) {
			next, nextSize := utf8.DecodeRuneInString(text[end:])
			if !isVKHashtagRune(next) {
				break
			}
			end += nextSize
		}
		if end == i+size {
			i += size
			continue
		}

		matches = append(matches, hashtagMatch{
			start: i,
			end:   end,
			tag:   text[i:end],
		})
		i = end
	}

	if len(matches) == 0 {
		return text, nil
	}

	hashtags := make([]string, 0, len(matches))
	var b strings.Builder
	last := 0
	for _, match := range matches {
		segment := text[last:match.start]
		hashtags = append(hashtags, match.tag)

		segment = trimRightVKInlineSpaces(segment)
		b.WriteString(segment)

		prevRune, hasPrevRune := lastNonSpaceRune(segment)
		if !hasPrevRune {
			prevRune, hasPrevRune = lastNonSpaceRune(b.String())
		}

		nextStart := skipVKInlineSpaces(text, match.end)
		nextRune, hasNextRune := nextVKVisibleRune(text, nextStart)
		if hasPrevRune && hasNextRune && isVKHashtagWordRune(prevRune) && isVKHashtagWordRune(nextRune) {
			b.WriteByte(' ')
		}

		last = nextStart
	}
	b.WriteString(text[last:])

	cleaned := strings.TrimRightFunc(b.String(), unicode.IsSpace)
	return cleaned, hashtags
}

func isVKHashtagBoundary(text string, index int) bool {
	if index == 0 {
		return true
	}

	prev, _ := utf8.DecodeLastRuneInString(text[:index])
	return unicode.IsSpace(prev)
}

func isVKHashtagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
}

func isVKHashtagWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func lastNonSpaceRune(s string) (rune, bool) {
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if !unicode.IsSpace(r) {
			return r, true
		}
		s = s[:len(s)-size]
	}

	return 0, false
}

func nextNonSpaceRune(s string, start int) (rune, bool) {
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !unicode.IsSpace(r) {
			return r, true
		}
		start += size
	}

	return 0, false
}

func trimRightVKInlineSpaces(s string) string {
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r != ' ' && r != '\t' {
			return s
		}
		s = s[:len(s)-size]
	}

	return s
}

func skipVKInlineSpaces(s string, start int) int {
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if r != ' ' && r != '\t' {
			return start
		}
		start += size
	}

	return start
}

func nextVKVisibleRune(s string, start int) (rune, bool) {
	if start >= len(s) {
		return 0, false
	}

	r, _ := utf8.DecodeRuneInString(s[start:])
	return r, true
}

func collectVKEntityLinks(text string, entities []tg.MessageEntity) []string {
	links := make([]vkEntityLink, 0)
	seen := make(map[string]struct{})
	allowed := map[string]bool{"YouTube": true, "Steam": true}
	if strings.Contains(text, "#запись") || strings.Contains(text, "Шортс доступен") {
		allowed["Twitch"] = true
		allowed["VK"] = true
		allowed["TikTok"] = true
		allowed["Boosty"] = true
	}

	for _, ent := range entities {
		rawURL, ok := entityURL(text, ent)
		if !ok {
			continue
		}

		link, ok := formatVKEntityLink(rawURL, allowed)
		if !ok {
			continue
		}
		key := link.label + "\x00" + strings.ToLower(strings.TrimSpace(rawURL))
		if _, exists := seen[key]; exists {
			continue
		}

		seen[key] = struct{}{}
		links = append(links, link)
	}

	return formatVKEntityLinks(links)
}

func entityURL(text string, ent tg.MessageEntity) (string, bool) {
	switch e := ent.(type) {
	case *tg.MessageEntityTextURL:
		if e.URL == "" {
			return "", false
		}
		return e.URL, true
	case *tg.MessageEntityURL:
		if e.Length <= 0 {
			return "", false
		}
		raw := utils.UTF16Slice(text, int(e.Offset), int(e.Offset+e.Length))
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return "", false
		}
		return raw, true
	default:
		return "", false
	}
}

func formatVKEntityLink(rawURL string, allowed map[string]bool) (vkEntityLink, bool) {
	normalizedURL := strings.TrimSpace(rawURL)
	if normalizedURL == "" {
		return vkEntityLink{}, false
	}
	label := classifyVKEntityLink(normalizedURL)
	if label == "" || !allowed[label] {
		return vkEntityLink{}, false
	}

	return vkEntityLink{
		label: label,
		url:   normalizedURL,
	}, true
}

func classifyVKEntityLink(rawURL string) string {
	matchURL := strings.ToLower(strings.TrimSpace(rawURL))
	switch {
	case strings.Contains(matchURL, "youtu.be/"), strings.Contains(matchURL, "youtube.com/shorts/"):
		return "YouTube"
	case strings.Contains(matchURL, "store.steampowered.com/app/"):
		return "Steam"
	case strings.Contains(matchURL, "twitch.tv/"):
		return "Twitch"
	case strings.Contains(matchURL, "vk.com/"), strings.Contains(matchURL, "vkvideo.ru/"), strings.Contains(matchURL, "vk.ru/"):
		return "VK"
	case strings.Contains(matchURL, "tiktok.com/"):
		return "TikTok"
	case strings.Contains(matchURL, "boosty.to/"):
		return "Boosty"
	default:
		return ""
	}
}

func formatVKEntityLinks(links []vkEntityLink) []string {
	if len(links) == 0 {
		return nil
	}

	totalByLabel := make(map[string]int, len(links))
	for _, link := range links {
		totalByLabel[link.label]++
	}

	currentByLabel := make(map[string]int, len(links))
	formatted := make([]string, 0, len(links))
	for _, link := range links {
		currentByLabel[link.label]++
		if totalByLabel[link.label] > 1 {
			formatted = append(formatted, fmt.Sprintf("%s #%d: %s", link.label, currentByLabel[link.label], link.url))
			continue
		}
		formatted = append(formatted, fmt.Sprintf("%s: %s", link.label, link.url))
	}

	return formatted
}

type vkRange struct {
	start  int
	end    int
	open   string
	close  string
	length int
}

func entityToVKRanges(text string, ent tg.MessageEntity) []vkRange {
	offset, length, openTag, closeTag, ok := entityToVKTags(ent)
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

	return []vkRange{{
		start:  startIdx,
		end:    endIdx,
		open:   openTag,
		close:  closeTag,
		length: adjustedEnd - adjustedStart,
	}}
}

func entityToVKTags(ent tg.MessageEntity) (offset int, length int, openTag string, closeTag string, ok bool) {
	switch e := ent.(type) {
	case *tg.MessageEntityBlockquote:
		return int(e.Offset), int(e.Length), "«", "»", true
	default:
		return 0, 0, "", "", false
	}
}
