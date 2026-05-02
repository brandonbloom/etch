package etch

import (
	"bytes"
	"unicode/utf8"
)

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
