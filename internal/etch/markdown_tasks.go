package etch

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"
)

func evalMarkdownTaskList(path string, op Operation, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in Markdown input")
	}
	var (
		out []byte
		err error
	)
	switch op.Verb {
	case "task close":
		out, err = closeMarkdownTask(raw, path, op.Value, op.Markdown)
	case "task open":
		out, err = openMarkdownTask(raw, path, op.Value, op.Markdown)
	case "list add":
		out, err = addMarkdownListItem(raw, path, op.Value, op.Markdown, false)
	case "task add":
		out, err = addMarkdownListItem(raw, path, op.Value, op.Markdown, true)
	default:
		return nil, false, usagef("unknown Markdown task/list verb %s", op.Verb)
	}
	if err != nil {
		return nil, false, err
	}
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func closeMarkdownTask(raw []byte, path, text string, address markdownAddress) ([]byte, error) {
	item, err := resolveMarkdownTaskForToggle(raw, path, text, address)
	if err != nil {
		return nil, err
	}
	switch item.TaskStatus {
	case ' ':
		return replaceMarkdownTaskStatus(raw, item, 'x'), nil
	case 'x', 'X':
		return raw, nil
	default:
		return nil, failf("task %q has unsupported checkbox status %q in %s", text, string(item.TaskStatus), path)
	}
}

func openMarkdownTask(raw []byte, path, text string, address markdownAddress) ([]byte, error) {
	scope, err := markdownTaskSearchScope(raw, path, address)
	if err != nil {
		return nil, err
	}
	matches, err := findMarkdownItemsInRange(raw, path, scope, text, []string{"task"})
	if err != nil {
		return nil, err
	}
	if len(matches) > 1 {
		return nil, failf("item %q is ambiguous in %s", text, path)
	}
	if len(matches) == 0 {
		if !address.hasBodyLocation() {
			return nil, failf("task %q not found in %s; missing task creation requires a destination address", text, path)
		}
		return addMarkdownListItem(raw, path, text, address, true)
	}
	item := matches[0]
	switch item.TaskStatus {
	case ' ':
		return raw, nil
	case 'x', 'X':
		return replaceMarkdownTaskStatus(raw, item, ' '), nil
	default:
		return nil, failf("task %q has unsupported checkbox status %q in %s", text, string(item.TaskStatus), path)
	}
}

func resolveMarkdownTaskForToggle(raw []byte, path, text string, address markdownAddress) (markdownListItem, error) {
	scope, err := markdownTaskSearchScope(raw, path, address)
	if err != nil {
		return markdownListItem{}, err
	}
	return resolveMarkdownTaskInRange(raw, path, scope, text, nil)
}

func markdownTaskSearchScope(raw []byte, path string, address markdownAddress) (markdownRange, error) {
	scope, err := markdownTaskBaseScope(raw, path, address)
	if err != nil {
		return markdownRange{}, err
	}
	return narrowMarkdownScopeByItemAnchors(raw, path, scope, address.Before, address.After)
}

func markdownTaskBaseScope(raw []byte, path string, address markdownAddress) (markdownRange, error) {
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
	return scope, nil
}

func narrowMarkdownScopeByItemAnchors(raw []byte, path string, scope markdownRange, before, after string) (markdownRange, error) {
	if after != "" {
		item, err := resolveMarkdownItemInRange(raw, path, scope, after, nil)
		if err != nil {
			return markdownRange{}, err
		}
		scope.Start = item.LineEnd
	}
	if before != "" {
		item, err := resolveMarkdownItemInRange(raw, path, scope, before, nil)
		if err != nil {
			return markdownRange{}, err
		}
		scope.End = item.LineStart
	}
	if scope.End < scope.Start {
		return markdownRange{}, failf("Markdown task/list address range is empty after applying anchors")
	}
	return scope, nil
}

func replaceMarkdownTaskStatus(raw []byte, item markdownListItem, status byte) []byte {
	if item.TaskStatus == status {
		return raw
	}
	out := append([]byte(nil), raw...)
	out[item.TaskStatusOffset] = status
	return out
}

func addMarkdownListItem(raw []byte, path, text string, address markdownAddress, task bool) ([]byte, error) {
	if err := validateMarkdownListItemText(text); err != nil {
		return nil, err
	}
	scope, err := markdownTaskBaseScope(raw, path, address)
	if err != nil {
		return nil, err
	}
	placement, err := markdownPlacementFromFlags(false, false, address.Before, address.After)
	if err != nil {
		return nil, err
	}
	target, err := resolveMarkdownListInsertion(raw, path, scope, placement)
	if err != nil {
		return nil, err
	}
	line := renderMarkdownListItemLine(text, task, target.Style)
	return insertMarkdownListItemLine(raw, target.Point, line, markdownNewline(raw), target.JoinList), nil
}

func validateMarkdownListItemText(text string) error {
	if strings.TrimSpace(text) == "" {
		return usagef("list item text must not be blank")
	}
	if strings.ContainsAny(text, "\r\n") {
		return usagef("list item text must be a single line")
	}
	if _, ok := parseMarkdownListItemLine([]byte(text)); ok {
		return usagef("list item text must not include a Markdown list marker")
	}
	return nil
}

type markdownListInsertion struct {
	Point    int
	JoinList bool
	Style    markdownListStyle
}

type markdownListStyle struct {
	Indent    string
	Marker    string
	Numbered  bool
	Number    int
	Delimiter byte
}

func resolveMarkdownListInsertion(raw []byte, path string, scope markdownRange, placement markdownPlacement) (markdownListInsertion, error) {
	switch placement.Kind {
	case markdownPlacementBefore, markdownPlacementAfter:
		item, err := resolveMarkdownItemInRange(raw, path, scope, placement.Anchor, nil)
		if err != nil {
			return markdownListInsertion{}, err
		}
		point := item.LineStart
		number := item.Number
		if placement.Kind == markdownPlacementAfter {
			point = item.LineEnd
			if item.Numbered {
				number++
			}
		}
		return markdownListInsertion{
			Point:    point,
			JoinList: true,
			Style:    markdownListStyleFromItem(item, number),
		}, nil
	case markdownPlacementDefault, markdownPlacementTail:
		if item, ok := lastSimpleMarkdownListItemInRange(raw, scope); ok {
			number := item.Number
			if item.Numbered {
				number++
			}
			return markdownListInsertion{
				Point:    item.LineEnd,
				JoinList: true,
				Style:    markdownListStyleFromItem(item, number),
			}, nil
		}
		return markdownListInsertion{
			Point: markdownFieldTailPoint(raw, scope),
			Style: markdownListStyle{Marker: "-"},
		}, nil
	default:
		return markdownListInsertion{}, usagef("unsupported Markdown list placement %s", placement.Kind)
	}
}

func markdownListStyleFromItem(item markdownListItem, number int) markdownListStyle {
	return markdownListStyle{
		Indent:    item.Indent,
		Marker:    item.Marker,
		Numbered:  item.Numbered,
		Number:    number,
		Delimiter: item.Delimiter,
	}
}

func lastSimpleMarkdownListItemInRange(raw []byte, scope markdownRange) (markdownListItem, bool) {
	var last markdownListItem
	found := false
	for _, item := range markdownListItems(raw) {
		if item.Complex || item.LineStart < scope.Start || item.LineEnd > scope.End {
			continue
		}
		last = item
		found = true
	}
	return last, found
}

func renderMarkdownListItemLine(text string, task bool, style markdownListStyle) string {
	marker := style.Marker
	if marker == "" {
		marker = "-"
	}
	if style.Numbered {
		delimiter := style.Delimiter
		if delimiter == 0 {
			delimiter = '.'
		}
		n := style.Number
		if n <= 0 {
			n = 1
		}
		marker = strconv.Itoa(n) + string(delimiter)
	}
	line := style.Indent + marker + " "
	if task {
		line += "[ ] "
	}
	return line + text
}

func insertMarkdownListItemLine(raw []byte, point int, line, newline string, joinList bool) []byte {
	if !joinList {
		return insertMarkdownFieldLine(raw, point, line, newline)
	}
	prefix := raw[:point]
	suffix := raw[point:]
	var insert []byte
	if len(prefix) > 0 && !bytes.HasSuffix(prefix, []byte("\n")) {
		insert = append(insert, newline...)
	}
	insert = append(insert, line...)
	insert = append(insert, newline...)
	out := make([]byte, 0, len(raw)+len(insert))
	out = append(out, prefix...)
	out = append(out, insert...)
	out = append(out, suffix...)
	return out
}
