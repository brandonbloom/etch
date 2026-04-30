package etch

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/brandonbloom/etch/internal/jsonx"
	"github.com/theory/jsonpath"
	"github.com/theory/jsonpath/spec"
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
	query := selector
	if strings.HasPrefix(selector, ".") {
		return nil, usagef("relative selector must not start with dot")
	} else if !strings.HasPrefix(selector, "$") {
		if strings.HasPrefix(selector, "[") {
			query = "$" + selector
		} else {
			query = "$." + selector
		}
	}
	var parts []selectorPart
	path, err := jsonpath.Parse(query)
	if err != nil {
		return nil, usagef("invalid selector syntax: %v", err)
	}
	for _, segment := range path.Query().Segments() {
		if segment.IsDescendant() {
			return nil, usagef("selector syntax can only address one node")
		}
		selectors := segment.Selectors()
		if len(selectors) != 1 {
			return nil, usagef("selector syntax can only address one node")
		}
		switch sel := selectors[0].(type) {
		case spec.Name:
			parts = append(parts, selectorPart{Key: string(sel), IsKey: true})
		case spec.Index:
			if sel < 0 {
				return nil, usagef("negative indexes are not supported")
			}
			parts = append(parts, selectorPart{Index: int(sel)})
		default:
			return nil, usagef("selector syntax can only address one node")
		}
	}
	return parts, nil
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
