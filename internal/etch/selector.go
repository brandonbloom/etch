package etch

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brandonbloom/etch/internal/jsonx"
)

type selectorPart struct {
	Key   string
	Index int
	IsKey bool
}

func NormalizeSelector(selector string) (string, error) {
	parts, err := ParseSelector(selector)
	if err != nil {
		return "", err
	}
	return renderSelector(parts), nil
}

func ParseSelector(selector string) ([]selectorPart, error) {
	if selector == "" {
		return nil, usagef("empty selector")
	}
	s := selector
	if s == "$" {
		return nil, nil
	}
	if strings.HasPrefix(s, "$") {
		s = s[1:]
	} else if strings.HasPrefix(s, ".") {
		return nil, usagef("relative selector must not start with dot")
	}
	var parts []selectorPart
	for len(s) > 0 {
		switch s[0] {
		case '.':
			s = s[1:]
			if s == "" {
				return nil, usagef("selector ends after dot")
			}
			n := 0
			for n < len(s) {
				r, size := utf8.DecodeRuneInString(s[n:])
				if n == 0 {
					if !(r == '_' || unicode.IsLetter(r)) {
						break
					}
				} else if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
					break
				}
				n += size
			}
			if n == 0 {
				return nil, usagef("invalid dotted selector segment")
			}
			parts = append(parts, selectorPart{Key: s[:n], IsKey: true})
			s = s[n:]
		case '[':
			end, content, err := bracketContent(s)
			if err != nil {
				return nil, err
			}
			if content == "" {
				return nil, usagef("empty bracket selector")
			}
			if content[0] == '"' || content[0] == '\'' {
				if content[0] == '\'' {
					return nil, usagef("selector bracket strings must use JSON double quotes")
				}
				var key string
				if err := jsonx.Unmarshal([]byte(content), &key); err != nil {
					return nil, usagef("invalid bracket string selector")
				}
				parts = append(parts, selectorPart{Key: key, IsKey: true})
			} else {
				if strings.HasPrefix(content, "-") {
					return nil, usagef("negative indexes are not supported")
				}
				if strings.ContainsAny(content, ":,*?() ") {
					return nil, usagef("selector syntax can only address one node")
				}
				n, err := strconv.Atoi(content)
				if err != nil {
					return nil, usagef("invalid array index %q", content)
				}
				parts = append(parts, selectorPart{Index: n})
			}
			s = s[end:]
		default:
			if len(parts) == 0 && !strings.HasPrefix(selector, "$") {
				n := 0
				for n < len(s) {
					c := s[n]
					if c == '.' || c == '[' {
						break
					}
					n++
				}
				if n == 0 {
					return nil, usagef("invalid selector")
				}
				key := s[:n]
				if !isSimpleMember(key) {
					return nil, usagef("invalid selector segment %q", key)
				}
				parts = append(parts, selectorPart{Key: key, IsKey: true})
				s = s[n:]
				continue
			}
			return nil, usagef("invalid selector syntax near %q", s)
		}
	}
	return parts, nil
}

func bracketContent(s string) (int, string, error) {
	if s == "" || s[0] != '[' {
		return 0, "", usagef("internal selector parser error")
	}
	inString := false
	escaped := false
	quote := byte(0)
	for i := 1; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if inString {
			if c == '\\' {
				escaped = true
			} else if c == quote {
				inString = false
			}
			continue
		}
		if c == '"' || c == '\'' {
			inString = true
			quote = c
			continue
		}
		if c == ']' {
			return i + 1, s[1:i], nil
		}
	}
	return 0, "", usagef("unterminated bracket selector")
}

func renderSelector(parts []selectorPart) string {
	var b strings.Builder
	b.WriteByte('$')
	for _, p := range parts {
		if p.IsKey {
			if isSimpleMember(p.Key) {
				b.WriteByte('.')
				b.WriteString(p.Key)
			} else {
				enc, _ := jsonx.Marshal(p.Key)
				b.WriteByte('[')
				b.Write(enc)
				b.WriteByte(']')
			}
		} else {
			fmt.Fprintf(&b, "[%d]", p.Index)
		}
	}
	return b.String()
}

func isSimpleMember(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !(r == '_' || unicode.IsLetter(r)) {
				return false
			}
		} else if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
