package etch

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/brandonbloom/etch/internal/jsonx"
)

func parseValue(raw string) any {
	if v, err := jsonx.DecodeValue([]byte(raw)); err == nil {
		return normalizeJSONValue(v)
	}
	return raw
}

func normalizeJSONValue(v any) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = normalizeJSONValue(x[i])
		}
		return x
	case map[string]any:
		for k, v := range x {
			x[k] = normalizeJSONValue(v)
		}
		return x
	default:
		return v
	}
}

func valuePreview(raw string, max int) string {
	if !utf8.ValidString(raw) {
		return fmt.Sprintf("<binary, %d bytes>", len(raw))
	}
	v := parseValue(raw)
	var rendered string
	if s, ok := v.(string); ok {
		b, _ := jsonx.Marshal(strings.ReplaceAll(s, "\r\n", "\n"))
		rendered = string(b)
	} else {
		rendered = compactJSON(v)
	}
	rendered = strings.ReplaceAll(rendered, "\n", `\n`)
	if len(rendered) <= max {
		return rendered
	}
	if max < 8 {
		return "..."
	}
	if strings.HasPrefix(rendered, `"`) {
		prefix := rendered
		if len(prefix) > max-4 {
			prefix = prefix[:max-4]
		}
		prefix = strings.TrimSuffix(prefix, `\`)
		return prefix + `..."`
	}
	return rendered[:max-3] + "..."
}

func semanticEqual(a, b any) bool {
	return reflect.DeepEqual(canonicalSemantic(a), canonicalSemantic(b))
}

func canonicalSemantic(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = canonicalSemantic(v)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[fmt.Sprint(k)] = canonicalSemantic(v)
		}
		return m
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = canonicalSemantic(v)
		}
		return out
	default:
		return x
	}
}
