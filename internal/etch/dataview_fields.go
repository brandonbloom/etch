package etch

import (
	"strings"
	"unicode"
)

func dataviewFieldName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var out strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsSpace(r):
			if out.Len() > 0 && !lastDash {
				out.WriteByte('-')
				lastDash = true
			}
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-':
			out.WriteRune(r)
			lastDash = r == '-'
		case dataviewKeepsEmojiRune(r):
			out.WriteRune(r)
			lastDash = false
		case strings.ContainsRune("*`~[](){}", r):
			// Formatting punctuation is ignored.
		default:
			// Dataview simplified names drop punctuation instead of making it
			// addressable source syntax.
		}
	}
	return strings.Trim(out.String(), "-")
}

func dataviewKeepsEmojiRune(r rune) bool {
	return unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) || r == '\u200d' || r == '\ufe0e' || r == '\ufe0f'
}
