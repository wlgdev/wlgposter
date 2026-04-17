package utils

import (
	"unicode"
	"unicode/utf8"
)

func UTF16IndexToByteIndex(s string, utf16Index int) int {
	if utf16Index <= 0 {
		return 0
	}
	utf16Count := 0
	for i, r := range s {
		if utf16Count >= utf16Index {
			return i
		}
		if r > 0xFFFF {
			utf16Count += 2
		} else {
			utf16Count++
		}
	}
	return len(s)
}

func UTF16Slice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start == end {
		return ""
	}

	utf16Count := 0
	byteStart := -1
	byteEnd := -1
	for i, r := range s {
		if utf16Count == start && byteStart == -1 {
			byteStart = i
		}
		if utf16Count == end && byteEnd == -1 {
			byteEnd = i
			break
		}
		if r > 0xFFFF {
			utf16Count += 2
		} else {
			utf16Count++
		}
	}
	if byteStart == -1 {
		byteStart = len(s)
	}
	if byteEnd == -1 {
		byteEnd = len(s)
	}
	if byteStart > byteEnd {
		byteStart = byteEnd
	}
	return s[byteStart:byteEnd]
}

func CountLeadingSpacesUTF16(s string) int {
	count := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			break
		}
		if r > 0xFFFF {
			count += 2
		} else {
			count++
		}
	}
	return count
}

func CountTrailingSpacesUTF16(s string) int {
	count := 0
	for i := len(s); i > 0; {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if !unicode.IsSpace(r) {
			break
		}
		if r > 0xFFFF {
			count += 2
		} else {
			count++
		}
		i -= size
	}
	return count
}

func UTF16Len(s string) int {
	count := 0
	for _, r := range s {
		if r > 0xFFFF {
			count += 2
		} else {
			count++
		}
	}
	return count
}
