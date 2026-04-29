package etch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

func parseValue(raw string) any {
	var v any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err == nil && dec.Decode(&struct{}{}) == io.EOF {
		return normalizeJSONValue(v)
	}
	return raw
}

func normalizeJSONValue(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := strconv.ParseInt(string(x), 10, 64); err == nil {
			return float64(i)
		}
		f, err := strconv.ParseFloat(string(x), 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return string(x)
		}
		return f
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
		b, _ := json.Marshal(strings.ReplaceAll(s, "\r\n", "\n"))
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
	case json.Number:
		return normalizeJSONValue(x)
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

func encodeJSON(v any, bom bool) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	b = ensureTrailingNewline(b)
	return withUTF8BOM(b, bom), nil
}

func decodeJSON(b []byte) (any, bool, error) {
	raw, bom := trimUTF8BOM(b)
	if !utf8.Valid(raw) {
		return nil, bom, failf("invalid UTF-8 in JSON input")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, bom, err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, bom, failf("JSON input contains trailing data")
	}
	return normalizeJSONValue(v), bom, nil
}

func mutateStructuredValue(root any, selector string, verb string, value any) (any, bool, error) {
	parts, err := ParseSelector(selector)
	if err != nil {
		return root, false, err
	}
	before := compactJSON(root)
	out, err := mutateAt(root, parts, verb, value)
	if err != nil {
		return root, false, err
	}
	changed := before != compactJSON(out)
	return out, changed, nil
}

func mutateAt(root any, parts []selectorPart, verb string, value any) (any, error) {
	if len(parts) == 0 {
		switch verb {
		case "set":
			return value, nil
		case "delete", "remove":
			return root, nil
		case "append", "add":
			arr, ok := root.([]any)
			if !ok {
				return root, failf("selector $ does not identify an array")
			}
			if verb == "add" {
				for _, item := range arr {
					if semanticEqual(item, value) {
						return root, nil
					}
				}
			}
			return append(arr, value), nil
		default:
			return root, usagef("unknown structured verb %s", verb)
		}
	}
	newRoot, err := cloneContainer(root, parts[0])
	if err != nil {
		return root, err
	}
	if err := mutateDesc(newRoot, parts, verb, value); err != nil {
		return root, err
	}
	return newRoot, nil
}

func cloneContainer(v any, next selectorPart) (any, error) {
	if v == nil {
		if next.IsKey {
			return map[string]any{}, nil
		}
		return []any{}, nil
	}
	return deepCopy(v), nil
}

func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = deepCopy(v)
		}
		return m
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = deepCopy(v)
		}
		return out
	default:
		return x
	}
}

func mutateDesc(cur any, parts []selectorPart, verb string, value any) error {
	p := parts[0]
	last := len(parts) == 1
	if p.IsKey {
		m, ok := cur.(map[string]any)
		if !ok {
			return failf("selector component %s found %T, want object", p.Key, cur)
		}
		if last {
			return mutateMapLeaf(m, p.Key, verb, value)
		}
		child, ok := m[p.Key]
		if !ok || child == nil {
			if parts[1].IsKey {
				child = map[string]any{}
			} else {
				child = []any{}
			}
		} else {
			var err error
			child, err = cloneExistingContainer(child, parts[1])
			if err != nil {
				return err
			}
		}
		m[p.Key] = child
		return mutateDesc(child, parts[1:], verb, value)
	}
	arr, ok := cur.([]any)
	if !ok {
		return failf("selector component [%d] found %T, want array", p.Index, cur)
	}
	if p.Index > len(arr) || (!last && p.Index == len(arr)) {
		return failf("array index %d out of range", p.Index)
	}
	if last {
		return mutateArrayLeaf(&arr, p.Index, verb, value, cur)
	}
	child := arr[p.Index]
	cloned, err := cloneExistingContainer(child, parts[1])
	if err != nil {
		return err
	}
	arr[p.Index] = cloned
	return mutateDesc(cloned, parts[1:], verb, value)
}

func cloneExistingContainer(child any, next selectorPart) (any, error) {
	switch child.(type) {
	case map[string]any, []any:
		return deepCopy(child), nil
	default:
		if next.IsKey {
			return nil, failf("selector intermediate is not an object")
		}
		return nil, failf("selector intermediate is not an array")
	}
}

func mutateMapLeaf(m map[string]any, key string, verb string, value any) error {
	switch verb {
	case "set":
		m[key] = value
	case "delete":
		delete(m, key)
	case "append", "add":
		arr, ok := m[key].([]any)
		if !ok {
			if _, exists := m[key]; exists {
				return failf("selector $.%s does not identify an array", key)
			}
			arr = []any{}
		}
		if verb == "add" {
			for _, item := range arr {
				if semanticEqual(item, value) {
					m[key] = arr
					return nil
				}
			}
		}
		m[key] = append(arr, value)
	case "remove":
		arr, ok := m[key].([]any)
		if !ok {
			if _, exists := m[key]; exists {
				return failf("selector $.%s does not identify an array", key)
			}
			return nil
		}
		out := arr[:0]
		for _, item := range arr {
			if !semanticEqual(item, value) {
				out = append(out, item)
			}
		}
		m[key] = out
	default:
		return usagef("unknown structured verb %s", verb)
	}
	return nil
}

func mutateArrayLeaf(arrp *[]any, idx int, verb string, value any, cur any) error {
	arr := *arrp
	switch verb {
	case "set":
		if idx == len(arr) {
			arr = append(arr, value)
		} else if idx < len(arr) {
			arr[idx] = value
		} else {
			return failf("array index %d out of range", idx)
		}
	case "delete":
		if idx >= len(arr) {
			return nil
		}
		arr = append(arr[:idx], arr[idx+1:]...)
	case "append", "add", "remove":
		if idx >= len(arr) {
			return failf("array index %d out of range", idx)
		}
		nested, ok := arr[idx].([]any)
		if !ok {
			return failf("selector component [%d] does not identify an array", idx)
		}
		if verb == "remove" {
			out := nested[:0]
			for _, item := range nested {
				if !semanticEqual(item, value) {
					out = append(out, item)
				}
			}
			arr[idx] = out
		} else {
			if verb == "add" {
				for _, item := range nested {
					if semanticEqual(item, value) {
						arr[idx] = nested
						*arrp = arr
						return nil
					}
				}
			}
			arr[idx] = append(nested, value)
		}
	default:
		return usagef("unknown structured verb %s", verb)
	}
	switch backing := cur.(type) {
	case []any:
		copy(backing, arr)
		if len(arr) > len(backing) {
			// The root slice growth case is handled by callers that own the root.
			return failf("cannot append through copied array selector")
		}
	}
	*arrp = arr
	return nil
}
