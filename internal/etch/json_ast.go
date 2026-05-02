package etch

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"encoding/json/jsontext"

	"github.com/brandonbloom/etch/internal/jsonx"
)

// jsonNode records source spans for JSON values; jsontext owns token validation,
// and edits replace only the selected node's bytes.
type jsonNode struct {
	Kind    byte
	Start   int
	End     int
	Members []jsonMember
	Elems   []*jsonNode
}

type jsonMember struct {
	Key        string
	Start      int
	End        int
	KeyEnd     int
	ColonStart int
	Value      *jsonNode
}

type jsonSpanDecoder struct {
	raw []byte
	dec *jsontext.Decoder
}

func evalJSON(selector, verb, rawValue string, valueMode ValueMode, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in JSON input")
	}
	root, err := decodeJSONSpans(raw)
	if err != nil {
		return nil, false, err
	}
	parts, err := ParseSelector(selector)
	if err != nil {
		return nil, false, err
	}
	value, err := jsonLiteralBytes(rawValue, valueMode)
	if err != nil {
		return nil, false, err
	}
	out, err := editJSON(raw, root, parts, verb, value)
	if err != nil {
		return nil, false, err
	}
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func decodeJSONSpans(raw []byte) (*jsonNode, error) {
	d := jsonSpanDecoder{
		raw: raw,
		dec: jsontext.NewDecoder(bytes.NewReader(raw), jsontext.AllowDuplicateNames(true)),
	}
	node, err := d.decodeValue()
	if err != nil {
		return nil, err
	}
	if skipJSONWhitespace(raw, int(d.dec.InputOffset())) != len(raw) {
		return nil, failf("JSON input contains trailing data")
	}
	return node, nil
}

func (d *jsonSpanDecoder) decodeValue() (*jsonNode, error) {
	start := d.nextTokenStart()
	switch d.dec.PeekKind() {
	case '{':
		return d.decodeObject(start)
	case '[':
		return d.decodeArray(start)
	case 'n', 'f', 't', '"', '0':
		tok, err := d.dec.ReadToken()
		if err != nil {
			return nil, err
		}
		return &jsonNode{Kind: byte(tok.Kind()), Start: start, End: int(d.dec.InputOffset())}, nil
	default:
		if _, err := d.dec.ReadToken(); err != nil {
			return nil, err
		}
		return nil, failf("unexpected JSON token")
	}
}

func (d *jsonSpanDecoder) decodeObject(start int) (*jsonNode, error) {
	tok, err := d.dec.ReadToken()
	if err != nil {
		return nil, err
	}
	if tok.Kind() != '{' {
		return nil, failf("expected JSON object")
	}
	node := &jsonNode{Kind: '{', Start: start}
	for {
		if d.dec.PeekKind() == '}' {
			if _, err := d.dec.ReadToken(); err != nil {
				return nil, err
			}
			node.End = int(d.dec.InputOffset())
			return node, nil
		}
		memberStart := d.nextTokenStart()
		keyTok, err := d.dec.ReadToken()
		if err != nil {
			return nil, err
		}
		if keyTok.Kind() != '"' {
			return nil, failf("object member name must be a string")
		}
		key := keyTok.String()
		keyEnd := int(d.dec.InputOffset())
		valueStart := d.nextTokenStart()
		colon := bytes.IndexByte(d.raw[keyEnd:valueStart], ':')
		if colon < 0 {
			return nil, failf("expected ':' after JSON object member name")
		}
		value, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		node.Members = append(node.Members, jsonMember{
			Key:        key,
			Start:      memberStart,
			End:        value.End,
			KeyEnd:     keyEnd,
			ColonStart: keyEnd + colon,
			Value:      value,
		})
	}
}

func (d *jsonSpanDecoder) decodeArray(start int) (*jsonNode, error) {
	tok, err := d.dec.ReadToken()
	if err != nil {
		return nil, err
	}
	if tok.Kind() != '[' {
		return nil, failf("expected JSON array")
	}
	node := &jsonNode{Kind: '[', Start: start}
	for {
		if d.dec.PeekKind() == ']' {
			if _, err := d.dec.ReadToken(); err != nil {
				return nil, err
			}
			node.End = int(d.dec.InputOffset())
			return node, nil
		}
		value, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		node.Elems = append(node.Elems, value)
	}
}

func (d *jsonSpanDecoder) nextTokenStart() int {
	pos := skipJSONWhitespace(d.raw, int(d.dec.InputOffset()))
	if pos < len(d.raw) && (d.raw[pos] == ':' || d.raw[pos] == ',') {
		pos = skipJSONWhitespace(d.raw, pos+1)
	}
	return pos
}

func skipJSONWhitespace(raw []byte, pos int) int {
	for pos < len(raw) {
		switch raw[pos] {
		case ' ', '\n', '\r', '\t':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func editJSON(raw []byte, node *jsonNode, parts []selectorPart, verb string, value []byte) ([]byte, error) {
	if len(parts) == 0 {
		return editJSONRoot(raw, node, verb, value)
	}
	part := parts[0]
	if part.IsKey {
		if node.Kind != '{' {
			return nil, failf("selector component %s found %s, want object", part.Key, jsonKindName(node.Kind))
		}
		idx := firstJSONMember(node, part.Key)
		if idx < 0 {
			return editMissingJSONMember(raw, node, parts, verb, value)
		}
		member := node.Members[idx]
		if len(parts) == 1 {
			return editJSONMemberLeaf(raw, node, idx, verb, value)
		}
		return editJSON(raw, member.Value, parts[1:], verb, value)
	}
	if node.Kind != '[' {
		return nil, failf("selector component [%d] found %s, want array", part.Index, jsonKindName(node.Kind))
	}
	if part.Index < 0 || part.Index > len(node.Elems) || (len(parts) > 1 && part.Index == len(node.Elems)) {
		return nil, failf("array index %d out of range", part.Index)
	}
	if len(parts) == 1 {
		return editJSONArrayLeaf(raw, node, part.Index, verb, value)
	}
	return editJSON(raw, node.Elems[part.Index], parts[1:], verb, value)
}

func editJSONRoot(raw []byte, node *jsonNode, verb string, value []byte) ([]byte, error) {
	switch verb {
	case "set":
		return replaceBytes(raw, node.Start, node.End, value), nil
	case "delete", "remove":
		return raw, nil
	case "append", "add":
		if node.Kind != '[' {
			return nil, failf("selector $ does not identify an array")
		}
		return appendJSONElement(raw, node, verb == "add", value)
	default:
		return nil, usagef("unknown structured verb %s", verb)
	}
}

func editJSONMemberLeaf(raw []byte, obj *jsonNode, idx int, verb string, value []byte) ([]byte, error) {
	member := obj.Members[idx]
	switch verb {
	case "set":
		return replaceBytes(raw, member.Value.Start, member.Value.End, value), nil
	case "delete":
		start, end := memberRemoveRange(raw, obj, idx)
		return replaceBytes(raw, start, end, nil), nil
	case "append", "add":
		if member.Value.Kind != '[' {
			return nil, failf("selector $.%s does not identify an array", member.Key)
		}
		return appendJSONElement(raw, member.Value, verb == "add", value)
	case "remove":
		if member.Value.Kind != '[' {
			return nil, failf("selector $.%s does not identify an array", member.Key)
		}
		return removeJSONElements(raw, member.Value, value), nil
	default:
		return nil, usagef("unknown structured verb %s", verb)
	}
}

func editJSONArrayLeaf(raw []byte, arr *jsonNode, idx int, verb string, value []byte) ([]byte, error) {
	switch verb {
	case "set":
		if idx == len(arr.Elems) {
			return appendJSONElement(raw, arr, false, value)
		}
		return replaceBytes(raw, arr.Elems[idx].Start, arr.Elems[idx].End, value), nil
	case "delete":
		if idx >= len(arr.Elems) {
			return raw, nil
		}
		start, end := elementRemoveRange(raw, arr, idx)
		return replaceBytes(raw, start, end, nil), nil
	case "append", "add":
		if idx >= len(arr.Elems) || arr.Elems[idx].Kind != '[' {
			return nil, failf("selector component [%d] does not identify an array", idx)
		}
		return appendJSONElement(raw, arr.Elems[idx], verb == "add", value)
	case "remove":
		if idx >= len(arr.Elems) || arr.Elems[idx].Kind != '[' {
			return nil, failf("selector component [%d] does not identify an array", idx)
		}
		return removeJSONElements(raw, arr.Elems[idx], value), nil
	default:
		return nil, usagef("unknown structured verb %s", verb)
	}
}

func editMissingJSONMember(raw []byte, obj *jsonNode, parts []selectorPart, verb string, value []byte) ([]byte, error) {
	if verb == "delete" || verb == "remove" {
		return raw, nil
	}
	memberValue, err := missingJSONValue(parts[1:], verb, value)
	if err != nil {
		return nil, err
	}
	key, _ := jsonx.Marshal(parts[0].Key)
	member := append(append([]byte{}, key...), objectColon(raw, obj)...)
	member = append(member, memberValue...)
	insertAt, snippet := objectInsert(raw, obj, member)
	return replaceBytes(raw, insertAt, insertAt, snippet), nil
}

func missingJSONValue(parts []selectorPart, verb string, value []byte) ([]byte, error) {
	if len(parts) == 0 {
		if verb == "append" || verb == "add" {
			return []byte("[" + string(value) + "]"), nil
		}
		return value, nil
	}
	child, err := missingJSONValue(parts[1:], verb, value)
	if err != nil {
		return nil, err
	}
	part := parts[0]
	if part.IsKey {
		key, _ := jsonx.Marshal(part.Key)
		out := append([]byte{'{'}, key...)
		out = append(out, ':')
		out = append(out, child...)
		out = append(out, '}')
		return out, nil
	}
	if part.Index != 0 {
		return nil, failf("array index %d out of range", part.Index)
	}
	out := append([]byte{'['}, child...)
	out = append(out, ']')
	return out, nil
}

func appendJSONElement(raw []byte, arr *jsonNode, add bool, value []byte) ([]byte, error) {
	if add {
		for _, elem := range arr.Elems {
			if jsonSemanticEqual(raw[elem.Start:elem.End], value) {
				return raw, nil
			}
		}
	}
	insertAt, snippet := arrayInsert(raw, arr, value)
	return replaceBytes(raw, insertAt, insertAt, snippet), nil
}

type jsonByteRange struct {
	Start int
	End   int
}

func removeJSONElements(raw []byte, arr *jsonNode, value []byte) []byte {
	remove := make([]bool, len(arr.Elems))
	for i, elem := range arr.Elems {
		remove[i] = jsonSemanticEqual(raw[elem.Start:elem.End], value)
	}

	var ranges []jsonByteRange
	for i := 0; i < len(arr.Elems); i++ {
		if !remove[i] {
			continue
		}
		startIdx := i
		for i+1 < len(arr.Elems) && remove[i+1] {
			i++
		}
		endIdx := i
		start, end := elementRunRemoveRange(raw, arr, startIdx, endIdx)
		ranges = append(ranges, jsonByteRange{Start: start, End: end})
	}

	out := raw
	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		out = replaceBytes(out, r.Start, r.End, nil)
	}
	return out
}

func jsonLiteralBytes(raw string, mode ValueMode) ([]byte, error) {
	switch mode {
	case "", ValueModeString:
		return jsonx.Marshal(raw)
	case ValueModeJSON:
		b := []byte(raw)
		v := jsontext.Value(b)
		if v.IsValid(jsontext.AllowDuplicateNames(true)) {
			return b, nil
		}
		return nil, usagef("invalid JSON value")
	default:
		return nil, usagef("unknown value mode %q", mode)
	}
}

func jsonSemanticEqual(a, b []byte) bool {
	av, errA := jsonx.DecodeValue(a)
	bv, errB := jsonx.DecodeValue(b)
	return errA == nil && errB == nil && semanticEqual(av, bv)
}

func firstJSONMember(obj *jsonNode, key string) int {
	for i, member := range obj.Members {
		if member.Key == key {
			return i
		}
	}
	return -1
}

func replaceBytes(raw []byte, start, end int, repl []byte) []byte {
	out := make([]byte, 0, len(raw)-(end-start)+len(repl))
	out = append(out, raw[:start]...)
	out = append(out, repl...)
	out = append(out, raw[end:]...)
	return out
}

func objectInsert(raw []byte, obj *jsonNode, member []byte) (int, []byte) {
	close := obj.End - 1
	if len(obj.Members) == 0 {
		return close, member
	}
	last := obj.Members[len(obj.Members)-1]
	if objectIsMultiline(raw, obj) {
		indent := lineIndent(raw, last.Start)
		snippet := append([]byte(",\n"), indent...)
		snippet = append(snippet, member...)
		return last.End, snippet
	}
	return close, append([]byte{','}, member...)
}

func arrayInsert(raw []byte, arr *jsonNode, value []byte) (int, []byte) {
	close := arr.End - 1
	if len(arr.Elems) == 0 {
		return close, value
	}
	last := arr.Elems[len(arr.Elems)-1]
	if arrayIsMultiline(raw, arr) {
		indent := lineIndent(raw, last.Start)
		snippet := append([]byte(",\n"), indent...)
		snippet = append(snippet, value...)
		return last.End, snippet
	}
	return close, append([]byte{','}, value...)
}

func objectColon(raw []byte, obj *jsonNode) []byte {
	if len(obj.Members) == 0 {
		return []byte(": ")
	}
	member := obj.Members[len(obj.Members)-1]
	return raw[member.KeyEnd:member.Value.Start]
}

func objectIsMultiline(raw []byte, obj *jsonNode) bool {
	return bytes.Contains(raw[obj.Start:obj.End], []byte{'\n'})
}

func arrayIsMultiline(raw []byte, arr *jsonNode) bool {
	return bytes.Contains(raw[arr.Start:arr.End], []byte{'\n'})
}

func lineIndent(raw []byte, pos int) []byte {
	lineStart := bytes.LastIndexByte(raw[:pos], '\n') + 1
	i := lineStart
	for i < pos && (raw[i] == ' ' || raw[i] == '\t') {
		i++
	}
	return raw[lineStart:i]
}

func memberRemoveRange(raw []byte, obj *jsonNode, idx int) (int, int) {
	member := obj.Members[idx]
	if len(obj.Members) == 1 {
		return member.Start, member.End
	}
	if idx < len(obj.Members)-1 {
		comma := bytes.IndexByte(raw[member.End:obj.Members[idx+1].Start], ',')
		if comma >= 0 {
			return member.Start, member.End + comma + 1
		}
	}
	prev := obj.Members[idx-1]
	comma := bytes.LastIndexByte(raw[prev.End:member.Start], ',')
	if comma >= 0 {
		return prev.End + comma, member.End
	}
	return member.Start, member.End
}

func elementRemoveRange(raw []byte, arr *jsonNode, idx int) (int, int) {
	elem := arr.Elems[idx]
	if len(arr.Elems) == 1 {
		return elem.Start, elem.End
	}
	if idx < len(arr.Elems)-1 {
		comma := bytes.IndexByte(raw[elem.End:arr.Elems[idx+1].Start], ',')
		if comma >= 0 {
			return elem.Start, elem.End + comma + 1
		}
	}
	prev := arr.Elems[idx-1]
	comma := bytes.LastIndexByte(raw[prev.End:elem.Start], ',')
	if comma >= 0 {
		return prev.End + comma, elem.End
	}
	return elem.Start, elem.End
}

func elementRunRemoveRange(raw []byte, arr *jsonNode, startIdx, endIdx int) (int, int) {
	first := arr.Elems[startIdx]
	last := arr.Elems[endIdx]
	if endIdx < len(arr.Elems)-1 {
		next := arr.Elems[endIdx+1]
		return first.Start, next.Start
	}
	if startIdx > 0 {
		prev := arr.Elems[startIdx-1]
		comma := bytes.LastIndexByte(raw[prev.End:first.Start], ',')
		if comma >= 0 {
			return prev.End + comma, last.End
		}
	}
	return first.Start, last.End
}

func jsonKindName(kind byte) string {
	switch kind {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case '0':
		return "number"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return fmt.Sprintf("JSON token %q", string(kind))
	}
}
