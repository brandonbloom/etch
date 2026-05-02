package etch

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brandonbloom/etch/internal/jsonx"
)

// semanticNumber is the comparable form used by semantic equality after
// normalizing JSON and YAML numeric scalars without lossy float64 coercion.
type semanticNumber string

func parseStructuredValue(raw string, mode ValueMode) (any, error) {
	switch mode {
	case "", ValueModeString:
		return raw, nil
	case ValueModeJSON:
		v, err := jsonx.DecodeValue([]byte(raw))
		if err != nil {
			return nil, usagef("invalid JSON value: %v", err)
		}
		return normalizeJSONValue(v), nil
	default:
		return nil, usagef("unknown value mode %q", mode)
	}
}

func parseLegacyPreviewValue(raw string) any {
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

func valuePreview(raw string, mode ValueMode, max int) string {
	if !utf8.ValidString(raw) {
		return fmt.Sprintf("<binary, %d bytes>", len(raw))
	}
	var v any
	if mode == "" {
		v = parseLegacyPreviewValue(raw)
	} else {
		parsed, err := parseStructuredValue(raw, mode)
		if err != nil {
			b, _ := jsonx.Marshal(strings.ReplaceAll(raw, "\r\n", "\n"))
			rendered := string(b)
			if len(rendered) <= max {
				return rendered
			}
			if max < 8 {
				return "..."
			}
			return rendered[:max-4] + `..."`
		}
		v = parsed
	}
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
		if scalar, ok := canonicalScalarSemantic(x); ok {
			return scalar
		}
		return x
	}
}

// canonicalScalarSemantic converts numeric scalar values into an exact shared
// representation, so 1, 1.0, and 1e0 compare equal while distinct large
// integers remain distinct.
func canonicalScalarSemantic(v any) (any, bool) {
	switch x := v.(type) {
	case jsonx.Number:
		return canonicalNumberSemantic(string(x)), true
	case int:
		return canonicalIntSemantic(int64(x)), true
	case int8:
		return canonicalIntSemantic(int64(x)), true
	case int16:
		return canonicalIntSemantic(int64(x)), true
	case int32:
		return canonicalIntSemantic(int64(x)), true
	case int64:
		return canonicalIntSemantic(x), true
	case uint:
		return canonicalUintSemantic(uint64(x)), true
	case uint8:
		return canonicalUintSemantic(uint64(x)), true
	case uint16:
		return canonicalUintSemantic(uint64(x)), true
	case uint32:
		return canonicalUintSemantic(uint64(x)), true
	case uint64:
		return canonicalUintSemantic(x), true
	case float32:
		return canonicalFloatSemantic(float64(x), 32), true
	case float64:
		return canonicalFloatSemantic(x, 64), true
	default:
		return nil, false
	}
}

func canonicalIntSemantic(i int64) semanticNumber {
	return canonicalNumberSemantic(strconv.FormatInt(i, 10))
}

func canonicalUintSemantic(u uint64) semanticNumber {
	return canonicalNumberSemantic(strconv.FormatUint(u, 10))
}

func canonicalFloatSemantic(f float64, bitSize int) any {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return f
	}
	return canonicalNumberSemantic(strconv.FormatFloat(f, 'g', -1, bitSize))
}

func canonicalNumberSemantic(s string) semanticNumber {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return semanticNumber(s)
	}
	if r.IsInt() {
		return semanticNumber(r.Num().String())
	}
	return semanticNumber(r.String())
}
