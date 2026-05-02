package etch

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	goldast "github.com/yuin/goldmark/ast"
)

type markdownInlineFieldForm string

const (
	markdownInlineFieldFullLine markdownInlineFieldForm = "full-line"
	markdownInlineFieldBracket  markdownInlineFieldForm = "bracket"
	markdownInlineFieldParen    markdownInlineFieldForm = "paren"
)

type markdownInlineField struct {
	Form       markdownInlineFieldForm
	RawName    string
	Normalized string
	Start      int
	End        int
	ValueStart int
	ValueEnd   int
	LineStart  int
	LineEnd    int
}

func decodeMarkdownSet(base Operation, args []string) ([]Operation, bool, error) {
	if len(args) < 2 {
		return nil, false, nil
	}
	if assignment, err := splitAssignmentItem(args[1]); err != nil {
		return nil, true, err
	} else if assignment.Present {
		if hasMarkdownAddressArgs(args[2:]) {
			return nil, true, usagef("assignment items cannot be combined with Markdown address flags")
		}
		return nil, false, nil
	}
	if !hasMarkdownAddressArgs(args[3:]) {
		return nil, false, nil
	}
	op, err := decodeMarkdownFieldSet(base, args)
	if err != nil {
		return nil, true, err
	}
	return []Operation{op}, true, nil
}

func decodeMarkdownFieldSet(op Operation, args []string) (Operation, error) {
	if len(args) < 3 {
		return op, usagef("usage: etch set <path.md> <field> <value> [address flags]")
	}
	if args[2] == "--json" {
		return op, usagef("--json is not supported for Markdown inline fields")
	}
	address, err := parseMarkdownAddressArgs(args[3:], true)
	if err != nil {
		return op, err
	}
	return decodeMarkdownField(op, "set", args[0], args[1], args[2], address)
}

func decodeMarkdownFieldDelete(op Operation, args []string) (Operation, error) {
	address, err := parseMarkdownAddressArgs(args[2:], false)
	if err != nil {
		return op, err
	}
	return decodeMarkdownField(op, "delete", args[0], args[1], "", address)
}

func decodeMarkdownField(op Operation, verb, path, field, value string, address markdownAddress) (Operation, error) {
	if strings.TrimSpace(field) == "" {
		return op, usagef("Markdown field name must not be blank")
	}
	if isReservedDataviewImplicitField(field) {
		return op, usagef("Dataview implicit field %q is not writable", field)
	}
	if address.hasItemLocation() && isReservedDataviewItemImplicitField(field) {
		return op, usagef("Dataview task/list implicit field %q is not writable", field)
	}
	op.Verb, op.Kind, op.Class, op.Path, op.Value, op.ValueMode = verb, "md-field", ClassIdempotent, path, value, ValueModeString
	op.Markdown = address
	op.Target = PlanTarget{Path: path, Part: "inline-field", Selector: field}
	if address.Section != "" {
		op.Target.Section = address.Section
	}
	switch {
	case address.Task != "":
		op.Target.Scope = "task"
	case address.Item != "":
		op.Target.Scope = "item"
	case address.Body:
		op.Target.Scope = "body"
	}
	fillDescriptor(&op)
	return op, nil
}

func hasMarkdownAddressArgs(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--body", "--section", "--item", "--item-type", "--task", "--after", "--before", "--head", "--tail", "--hidden":
			return true
		}
	}
	return false
}

func parseMarkdownAddressArgs(args []string, allowHidden bool) (markdownAddress, error) {
	var address markdownAddress
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--body":
			address.Body = true
		case "--section", "--item", "--item-type", "--task", "--after", "--before":
			if i+1 >= len(args) {
				return markdownAddress{}, usagef("%s requires a value", arg)
			}
			value := args[i+1]
			i++
			switch arg {
			case "--section":
				address.Section = value
			case "--item":
				address.Item = value
			case "--item-type":
				address.ItemTypes = append(address.ItemTypes, value)
			case "--task":
				address.Task = value
			case "--after":
				address.After = value
			case "--before":
				address.Before = value
			}
		case "--head":
			address.Head = true
		case "--tail":
			address.Tail = true
		case "--hidden":
			if !allowHidden {
				return markdownAddress{}, usagef("--hidden is only accepted by set")
			}
			address.Hidden = true
		default:
			if strings.HasPrefix(arg, "--") {
				return markdownAddress{}, usagef("unknown Markdown address flag %s", arg)
			}
			return markdownAddress{}, usagef("unexpected Markdown field argument %q", arg)
		}
	}
	if address.Item != "" && address.Task != "" {
		return markdownAddress{}, usagef("--item and --task are mutually exclusive")
	}
	if len(address.ItemTypes) > 0 && !address.hasItemLocation() {
		return markdownAddress{}, usagef("--item-type requires --item or --task")
	}
	if !address.hasBodyLocation() {
		return markdownAddress{}, usagef("Markdown inline field mutation requires an address flag")
	}
	if _, err := markdownItemTypeConstraintsFromArgs(address.ItemTypes); err != nil {
		return markdownAddress{}, err
	}
	if _, err := markdownPlacementFromFlags(address.Head, address.Tail, address.Before, address.After); err != nil {
		return markdownAddress{}, err
	}
	return address, nil
}

func isReservedDataviewImplicitField(field string) bool {
	field = strings.TrimSpace(field)
	return field == "file" || strings.HasPrefix(field, "file.") || strings.HasPrefix(field, "$.file.")
}

func isReservedDataviewItemImplicitField(field string) bool {
	switch dataviewFieldName(field) {
	case "status", "checked", "completed", "fullycompleted", "text", "line", "section", "children", "task", "parent", "blockid":
		return true
	default:
		return false
	}
}

func evalMarkdownField(path string, op Operation, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in Markdown input")
	}
	scope, err := markdownFieldScope(raw, path, op.Markdown, true)
	if err != nil {
		return nil, false, err
	}
	fields := markdownInlineFields(raw, scope)
	field, found, err := resolveMarkdownInlineField(fields, op.Target.Selector)
	if err != nil {
		return nil, false, err
	}
	if op.Verb == "delete" {
		if !found {
			return before, false, nil
		}
		out := deleteMarkdownInlineField(raw, field)
		out = withUTF8BOM(out, bom)
		return out, !bytes.Equal(out, before), nil
	}
	if found {
		out := replaceMarkdownInlineFieldValue(raw, field, op.Value)
		out = withUTF8BOM(out, bom)
		return out, !bytes.Equal(out, before), nil
	}
	createScope, err := markdownFieldScope(raw, path, op.Markdown, false)
	if err != nil {
		return nil, false, err
	}
	out, err := createMarkdownInlineField(raw, path, createScope, op.Target.Selector, op.Value, op.Markdown)
	if err != nil {
		return nil, false, err
	}
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func markdownFieldScope(raw []byte, path string, address markdownAddress, narrowAnchors bool) (markdownRange, error) {
	scope, err := markdownBodyRange(raw)
	if err != nil {
		return markdownRange{}, err
	}
	if address.Section != "" {
		section, err := resolveMarkdownSection(raw, path, address.Section)
		if err != nil {
			return markdownRange{}, err
		}
		scope = markdownRange{Start: section.Heading.BodyStart, End: section.BodyEnd}
	}
	if narrowAnchors || address.hasItemLocation() {
		scope, err = narrowMarkdownScope(raw, path, scope, address.Before, address.After)
		if err != nil {
			return markdownRange{}, err
		}
	}
	if address.Task != "" {
		item, err := resolveMarkdownTaskInRange(raw, path, scope, address.Task, address.ItemTypes)
		if err != nil {
			return markdownRange{}, err
		}
		return markdownRange{Start: item.LineStart, End: item.LineEnd}, nil
	}
	if address.Item != "" {
		item, err := resolveMarkdownItemInRange(raw, path, scope, address.Item, address.ItemTypes)
		if err != nil {
			return markdownRange{}, err
		}
		return markdownRange{Start: item.LineStart, End: item.LineEnd}, nil
	}
	return scope, nil
}

func narrowMarkdownScope(raw []byte, path string, scope markdownRange, before, after string) (markdownRange, error) {
	if after != "" {
		anchor, err := resolveMarkdownLiteralAnchor(raw, path, scope, after)
		if err != nil {
			return markdownRange{}, err
		}
		scope.Start = anchor.End
	}
	if before != "" {
		anchor, err := resolveMarkdownLiteralAnchor(raw, path, scope, before)
		if err != nil {
			return markdownRange{}, err
		}
		scope.End = anchor.Start
	}
	if scope.End < scope.Start {
		return markdownRange{}, failf("Markdown address range is empty after applying anchors")
	}
	return scope, nil
}

func resolveMarkdownInlineField(fields []markdownInlineField, selector string) (markdownInlineField, bool, error) {
	var exact []markdownInlineField
	for _, field := range fields {
		if field.RawName == selector {
			exact = append(exact, field)
		}
	}
	if len(exact) == 1 {
		return exact[0], true, nil
	}
	if len(exact) > 1 {
		return markdownInlineField{}, false, failf("Markdown field %q is ambiguous", selector)
	}
	normalized := dataviewFieldName(selector)
	var matches []markdownInlineField
	for _, field := range fields {
		if field.Normalized == normalized {
			matches = append(matches, field)
		}
	}
	if len(matches) == 0 {
		return markdownInlineField{}, false, nil
	}
	if len(matches) > 1 {
		return markdownInlineField{}, false, failf("Markdown field %q is ambiguous after Dataview normalization", selector)
	}
	return matches[0], true, nil
}

func replaceMarkdownInlineFieldValue(raw []byte, field markdownInlineField, value string) []byte {
	out := make([]byte, 0, len(raw)-(field.ValueEnd-field.ValueStart)+len(value))
	out = append(out, raw[:field.ValueStart]...)
	out = append(out, value...)
	out = append(out, raw[field.ValueEnd:]...)
	return out
}

func deleteMarkdownInlineField(raw []byte, field markdownInlineField) []byte {
	start, end := field.Start, field.End
	if field.Form == markdownInlineFieldFullLine {
		start, end = field.LineStart, field.LineEnd
	} else if start > field.LineStart && raw[start-1] == ' ' {
		start--
	} else if end < field.LineEnd && raw[end] == ' ' {
		end++
	}
	out := make([]byte, 0, len(raw)-(end-start))
	out = append(out, raw[:start]...)
	out = append(out, raw[end:]...)
	return out
}

func createMarkdownInlineField(raw []byte, path string, scope markdownRange, name, value string, address markdownAddress) ([]byte, error) {
	if address.hasItemLocation() {
		return appendMarkdownItemField(raw, scope, name, value, address.Hidden), nil
	}
	placement, err := markdownPlacementFromFlags(address.Head, address.Tail, address.Before, address.After)
	if err != nil {
		return nil, err
	}
	if placement.Kind == markdownPlacementDefault {
		placement.Kind = markdownPlacementTail
	}
	point, err := resolveMarkdownPlacementPoint(raw, path, scope, placement)
	if err != nil {
		return nil, err
	}
	if placement.Kind == markdownPlacementBefore {
		point = markdownLineStart(raw, point)
	} else if placement.Kind == markdownPlacementAfter {
		point = markdownLineEnd(raw, point)
	}
	if placement.Kind == markdownPlacementTail {
		point = markdownFieldTailPoint(raw, scope)
	}
	if placement.Kind == markdownPlacementHead {
		point = scope.Start
	}
	line := name + ":: " + value
	if address.Hidden {
		line = "(" + line + ")"
	}
	return insertMarkdownFieldLine(raw, point, line, markdownNewline(raw)), nil
}

func markdownFieldTailPoint(raw []byte, scope markdownRange) int {
	end := scope.End
	for end > scope.Start {
		lineEnd := end
		if raw[lineEnd-1] == '\n' {
			lineEnd--
			if lineEnd > scope.Start && raw[lineEnd-1] == '\r' {
				lineEnd--
			}
		}
		lineStart := bytes.LastIndexByte(raw[scope.Start:lineEnd], '\n')
		if lineStart >= 0 {
			lineStart += scope.Start + 1
		} else {
			lineStart = scope.Start
		}
		if !markdownBlankBytesLine(raw[lineStart:lineEnd]) {
			return markdownLineEnd(raw, lineStart)
		}
		end = lineStart
	}
	return scope.Start
}

func appendMarkdownItemField(raw []byte, scope markdownRange, name, value string, hidden bool) []byte {
	end := scope.End
	lineBreak := []byte{}
	if end > scope.Start && raw[end-1] == '\n' {
		lineBreak = []byte{'\n'}
		end--
		if end > scope.Start && raw[end-1] == '\r' {
			lineBreak = []byte{'\r', '\n'}
			end--
		}
	}
	form := "[" + name + ":: " + value + "]"
	if hidden {
		form = "(" + name + ":: " + value + ")"
	}
	insert := " " + form
	out := make([]byte, 0, len(raw)+len(insert))
	out = append(out, raw[:end]...)
	out = append(out, insert...)
	out = append(out, lineBreak...)
	out = append(out, raw[scope.End:]...)
	return out
}

func insertMarkdownFieldLine(raw []byte, point int, line, newline string) []byte {
	prefix := raw[:point]
	suffix := raw[point:]
	var insert []byte
	if len(prefix) > 0 && !bytes.HasSuffix(prefix, []byte("\n")) {
		insert = append(insert, newline...)
	} else if markdownNeedsBlankBeforeInsertedLine(prefix) {
		insert = append(insert, newline...)
	}
	insert = append(insert, line...)
	insert = append(insert, newline...)
	if markdownNeedsBlankAfterInsertedLine(suffix) {
		insert = append(insert, newline...)
	}
	out := make([]byte, 0, len(raw)+len(insert))
	out = append(out, prefix...)
	out = append(out, insert...)
	out = append(out, suffix...)
	return out
}

func markdownNeedsBlankBeforeInsertedLine(prefix []byte) bool {
	if len(prefix) == 0 || !bytes.HasSuffix(prefix, []byte("\n")) {
		return false
	}
	lineEnd := len(prefix) - 1
	if lineEnd > 0 && prefix[lineEnd-1] == '\r' {
		lineEnd--
	}
	lineStart := bytes.LastIndexByte(prefix[:lineEnd], '\n') + 1
	line := prefix[lineStart:lineEnd]
	return !markdownBlankBytesLine(line) && !markdownLineIsATXHeading(line)
}

func markdownNeedsBlankAfterInsertedLine(suffix []byte) bool {
	if len(suffix) == 0 {
		return false
	}
	lineEnd := markdownLineEnd(suffix, 0)
	line := suffix[:lineEnd]
	line = bytes.TrimRight(line, "\r\n")
	return !markdownBlankBytesLine(line) && !markdownLineIsATXHeading(line)
}

func markdownInlineFields(raw []byte, scope markdownRange) []markdownInlineField {
	excluded := markdownExcludedInlineFieldRanges(raw)
	var fields []markdownInlineField
	for start := scope.Start; start < scope.End; {
		lineEnd := markdownLineEnd(raw, start)
		if lineEnd > scope.End {
			lineEnd = scope.End
		}
		line := raw[start:lineEnd]
		if field, ok := parseMarkdownFullLineField(raw, start, line, excluded); ok {
			fields = append(fields, field)
		}
		fields = append(fields, parseMarkdownDelimitedInlineFields(raw, start, lineEnd, excluded)...)
		start = lineEnd
	}
	return fields
}

func parseMarkdownFullLineField(raw []byte, lineStart int, line []byte, excluded []markdownRange) (markdownInlineField, bool) {
	lineEnd := lineStart + len(line)
	contentEnd := lineEnd
	if contentEnd > lineStart && raw[contentEnd-1] == '\n' {
		contentEnd--
		if contentEnd > lineStart && raw[contentEnd-1] == '\r' {
			contentEnd--
		}
	}
	if rangeOverlaps(excluded, markdownRange{Start: lineStart, End: contentEnd}) {
		return markdownInlineField{}, false
	}
	trimmedRight := raw[lineStart:contentEnd]
	indent := 0
	for indent < len(trimmedRight) && trimmedRight[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(trimmedRight) {
		return markdownInlineField{}, false
	}
	if _, ok := parseMarkdownListItemLine(trimmedRight); ok {
		return markdownInlineField{}, false
	}
	if markdownLineIsATXHeading(trimmedRight) || bytes.HasPrefix(bytes.TrimSpace(trimmedRight), []byte(">")) {
		return markdownInlineField{}, false
	}
	sep := bytes.Index(trimmedRight[indent:], []byte("::"))
	if sep < 0 {
		return markdownInlineField{}, false
	}
	nameStart := lineStart + indent
	nameEnd := lineStart + indent + sep
	name := strings.TrimSpace(string(raw[nameStart:nameEnd]))
	if !validDataviewSourceFieldName(name) {
		return markdownInlineField{}, false
	}
	valueStart := lineStart + indent + sep + 2
	for valueStart < contentEnd && (raw[valueStart] == ' ' || raw[valueStart] == '\t') {
		valueStart++
	}
	return markdownInlineField{
		Form:       markdownInlineFieldFullLine,
		RawName:    name,
		Normalized: dataviewFieldName(name),
		Start:      nameStart,
		End:        contentEnd,
		ValueStart: valueStart,
		ValueEnd:   contentEnd,
		LineStart:  lineStart,
		LineEnd:    lineEnd,
	}, true
}

func parseMarkdownDelimitedInlineFields(raw []byte, lineStart, lineEnd int, excluded []markdownRange) []markdownInlineField {
	var fields []markdownInlineField
	contentEnd := lineEnd
	if contentEnd > lineStart && raw[contentEnd-1] == '\n' {
		contentEnd--
		if contentEnd > lineStart && raw[contentEnd-1] == '\r' {
			contentEnd--
		}
	}
	for i := lineStart; i < contentEnd; i++ {
		if rangeContains(excluded, i) || markdownInlineCodeContains(raw[lineStart:contentEnd], i-lineStart) {
			continue
		}
		close := byte(0)
		form := markdownInlineFieldBracket
		switch raw[i] {
		case '[':
			close = ']'
		case '(':
			close = ')'
			form = markdownInlineFieldParen
		default:
			continue
		}
		relClose := bytes.IndexByte(raw[i+1:contentEnd], close)
		if relClose < 0 {
			continue
		}
		end := i + 1 + relClose
		inner := raw[i+1 : end]
		sep := bytes.Index(inner, []byte("::"))
		if sep < 0 {
			i = end
			continue
		}
		name := strings.TrimSpace(string(inner[:sep]))
		if !validDataviewSourceFieldName(name) {
			i = end
			continue
		}
		valueStart := i + 1 + sep + 2
		for valueStart < end && (raw[valueStart] == ' ' || raw[valueStart] == '\t') {
			valueStart++
		}
		fields = append(fields, markdownInlineField{
			Form:       form,
			RawName:    name,
			Normalized: dataviewFieldName(name),
			Start:      i,
			End:        end + 1,
			ValueStart: valueStart,
			ValueEnd:   end,
			LineStart:  lineStart,
			LineEnd:    lineEnd,
		})
		i = end
	}
	return fields
}

func validDataviewSourceFieldName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "\n") || strings.Contains(name, "\r") {
		return false
	}
	return !strings.ContainsAny(name, "`[](){}<>")
}

func markdownExcludedInlineFieldRanges(raw []byte) []markdownRange {
	doc := parseMarkdownDocument(raw)
	var ranges []markdownRange
	_ = goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		switch n.(type) {
		case *goldast.CodeBlock, *goldast.FencedCodeBlock, *goldast.HTMLBlock:
			if r, ok := markdownNodeLineRange(raw, n); ok {
				ranges = append(ranges, r)
			}
			return goldast.WalkSkipChildren, nil
		default:
			return goldast.WalkContinue, nil
		}
	})
	return ranges
}

func markdownNodeLineRange(raw []byte, n goldast.Node) (markdownRange, bool) {
	lines := n.Lines()
	if lines.Len() == 0 {
		return markdownRange{}, false
	}
	first := lines.At(0)
	last := lines.At(lines.Len() - 1)
	return markdownRange{Start: markdownLineStart(raw, first.Start), End: markdownLineEnd(raw, last.Stop)}, true
}

func rangeOverlaps(ranges []markdownRange, target markdownRange) bool {
	for _, r := range ranges {
		if target.Start < r.End && r.Start < target.End {
			return true
		}
	}
	return false
}

func rangeContains(ranges []markdownRange, pos int) bool {
	for _, r := range ranges {
		if pos >= r.Start && pos < r.End {
			return true
		}
	}
	return false
}

func markdownInlineCodeContains(line []byte, pos int) bool {
	inCode := false
	for i := 0; i < len(line); i++ {
		if line[i] != '`' {
			continue
		}
		j := i + 1
		for j < len(line) && line[j] == '`' {
			j++
		}
		if pos >= i && pos < j {
			return true
		}
		if i < pos {
			inCode = !inCode
		}
		i = j - 1
	}
	return inCode
}

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
