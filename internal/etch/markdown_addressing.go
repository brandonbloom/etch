package etch

import (
	"bytes"
	"strconv"
	"strings"
	"unicode"

	goldast "github.com/yuin/goldmark/ast"
	goldtext "github.com/yuin/goldmark/text"
)

type markdownRange struct {
	Start int
	End   int
}

type markdownAddress struct {
	Body      bool
	Section   string
	Item      string
	Task      string
	ItemTypes []string
	Before    string
	After     string
	Head      bool
	Tail      bool
	Hidden    bool
}

func (a markdownAddress) hasBodyLocation() bool {
	return a.Body || a.Section != "" || a.Item != "" || a.Task != "" ||
		a.Before != "" || a.After != "" || a.Head || a.Tail
}

func (a markdownAddress) hasItemLocation() bool {
	return a.Item != "" || a.Task != ""
}

func markdownBodyRange(raw []byte) (markdownRange, error) {
	_, body, _, err := splitFrontmatter(raw)
	if err != nil {
		return markdownRange{}, err
	}
	return markdownRange{Start: len(raw) - len(body), End: len(raw)}, nil
}

func resolveMarkdownSection(raw []byte, path, selector string) (markdownSection, error) {
	sections, err := markdownSections(raw, selector)
	if err != nil {
		return markdownSection{}, err
	}
	if len(sections) == 0 {
		return markdownSection{}, failf("heading %q not found in %s", selector, path)
	}
	if len(sections) > 1 {
		return markdownSection{}, failf("heading %q is ambiguous in %s", selector, path)
	}
	return sections[0], nil
}

type markdownPlacementKind string

const (
	markdownPlacementDefault markdownPlacementKind = ""
	markdownPlacementHead    markdownPlacementKind = "head"
	markdownPlacementTail    markdownPlacementKind = "tail"
	markdownPlacementBefore  markdownPlacementKind = "before"
	markdownPlacementAfter   markdownPlacementKind = "after"
)

type markdownPlacement struct {
	Kind   markdownPlacementKind
	Anchor string
}

func markdownPlacementFromFlags(head, tail bool, before, after string) (markdownPlacement, error) {
	var placements []markdownPlacement
	var flags []string
	if head {
		placements = append(placements, markdownPlacement{Kind: markdownPlacementHead})
		flags = append(flags, "--head")
	}
	if tail {
		placements = append(placements, markdownPlacement{Kind: markdownPlacementTail})
		flags = append(flags, "--tail")
	}
	if before != "" {
		placements = append(placements, markdownPlacement{Kind: markdownPlacementBefore, Anchor: before})
		flags = append(flags, "--before")
	}
	if after != "" {
		placements = append(placements, markdownPlacement{Kind: markdownPlacementAfter, Anchor: after})
		flags = append(flags, "--after")
	}
	if len(placements) > 1 {
		return markdownPlacement{}, usagef("%s are mutually exclusive", formatMarkdownFlagList(flags))
	}
	if len(placements) == 0 {
		return markdownPlacement{Kind: markdownPlacementDefault}, nil
	}
	return placements[0], nil
}

func formatMarkdownFlagList(flags []string) string {
	switch len(flags) {
	case 0:
		return ""
	case 1:
		return flags[0]
	case 2:
		return flags[0] + " and " + flags[1]
	default:
		return strings.Join(flags[:len(flags)-1], ", ") + ", and " + flags[len(flags)-1]
	}
}

func resolveMarkdownPlacementPoint(raw []byte, path string, scope markdownRange, placement markdownPlacement) (int, error) {
	if scope.Start < 0 || scope.End < scope.Start || scope.End > len(raw) {
		return 0, failf("invalid Markdown scope in %s", path)
	}
	switch placement.Kind {
	case markdownPlacementHead:
		return scope.Start, nil
	case markdownPlacementTail:
		return scope.End, nil
	case markdownPlacementBefore:
		anchor, err := resolveMarkdownLiteralAnchor(raw, path, scope, placement.Anchor)
		if err != nil {
			return 0, err
		}
		return anchor.Start, nil
	case markdownPlacementAfter:
		anchor, err := resolveMarkdownLiteralAnchor(raw, path, scope, placement.Anchor)
		if err != nil {
			return 0, err
		}
		return anchor.End, nil
	default:
		return 0, usagef("placement is required")
	}
}

func resolveMarkdownLiteralAnchor(raw []byte, path string, scope markdownRange, literal string) (markdownRange, error) {
	if literal == "" {
		return markdownRange{}, usagef("anchor literal must not be blank")
	}
	haystack := raw[scope.Start:scope.End]
	needle := []byte(literal)
	var matches []markdownRange
	for offset := 0; offset <= len(haystack); {
		i := bytes.Index(haystack[offset:], needle)
		if i < 0 {
			break
		}
		start := scope.Start + offset + i
		matches = append(matches, markdownRange{Start: start, End: start + len(needle)})
		offset += i + len(needle)
	}
	if len(matches) == 0 {
		return markdownRange{}, failf("anchor %q not found in %s", literal, path)
	}
	if len(matches) > 1 {
		return markdownRange{}, failf("anchor %q is ambiguous in %s", literal, path)
	}
	return matches[0], nil
}

type markdownItemTypeConstraints struct {
	TaskState string
	Marker    string
}

func markdownItemTypeConstraintsFromArgs(types []string) (markdownItemTypeConstraints, error) {
	var c markdownItemTypeConstraints
	for _, typ := range types {
		switch typ {
		case "task", "plain":
			if c.TaskState != "" && c.TaskState != typ {
				return markdownItemTypeConstraints{}, usagef("contradictory item types: %s and %s", c.TaskState, typ)
			}
			c.TaskState = typ
		case "numbered", "bullet":
			if c.Marker != "" && c.Marker != typ {
				return markdownItemTypeConstraints{}, usagef("contradictory item types: %s and %s", c.Marker, typ)
			}
			c.Marker = typ
		default:
			return markdownItemTypeConstraints{}, usagef("unknown item type %s", typ)
		}
	}
	return c, nil
}

func (c markdownItemTypeConstraints) matches(item markdownListItem) bool {
	switch c.TaskState {
	case "task":
		if !item.Task {
			return false
		}
	case "plain":
		if item.Task {
			return false
		}
	}
	switch c.Marker {
	case "numbered":
		if !item.Numbered {
			return false
		}
	case "bullet":
		if item.Numbered {
			return false
		}
	}
	return true
}

type markdownListItem struct {
	LineStart        int
	LineEnd          int
	Normalized       string
	Task             bool
	TaskStatus       byte
	TaskStatusOffset int
	Numbered         bool
	Number           int
	Delimiter        byte
	Indent           string
	Marker           string
	Complex          bool
}

func resolveMarkdownTask(raw []byte, path, literal string, types []string) (markdownListItem, error) {
	scope, err := markdownBodyRange(raw)
	if err != nil {
		return markdownListItem{}, err
	}
	return resolveMarkdownTaskInRange(raw, path, scope, literal, types)
}

func resolveMarkdownItem(raw []byte, path, literal string, types []string) (markdownListItem, error) {
	scope, err := markdownBodyRange(raw)
	if err != nil {
		return markdownListItem{}, err
	}
	return resolveMarkdownItemInRange(raw, path, scope, literal, types)
}

func resolveMarkdownTaskInRange(raw []byte, path string, scope markdownRange, literal string, types []string) (markdownListItem, error) {
	return resolveMarkdownItemInRange(raw, path, scope, literal, append([]string{"task"}, types...))
}

func resolveMarkdownItemInRange(raw []byte, path string, scope markdownRange, literal string, types []string) (markdownListItem, error) {
	matches, err := findMarkdownItemsInRange(raw, path, scope, literal, types)
	if err != nil {
		return markdownListItem{}, err
	}
	if len(matches) == 0 {
		return markdownListItem{}, failf("item %q not found in %s", literal, path)
	}
	if len(matches) > 1 {
		return markdownListItem{}, failf("item %q is ambiguous in %s", literal, path)
	}
	return matches[0], nil
}

func findMarkdownItemsInRange(raw []byte, path string, scope markdownRange, literal string, types []string) ([]markdownListItem, error) {
	constraints, err := markdownItemTypeConstraintsFromArgs(types)
	if err != nil {
		return nil, err
	}
	want := normalizeMarkdownItemText(literal)
	var matches []markdownListItem
	for _, item := range markdownListItems(raw) {
		if item.LineStart < scope.Start || item.LineEnd > scope.End {
			continue
		}
		if constraints.matches(item) && item.Normalized == want {
			if item.Complex {
				return nil, failf("item %q is structurally complex in %s", literal, path)
			}
			matches = append(matches, item)
		}
	}
	return matches, nil
}

func markdownListItems(raw []byte) []markdownListItem {
	doc := parseMarkdownDocument(raw)
	var items []markdownListItem
	_ = goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		item, ok := n.(*goldast.ListItem)
		if !ok {
			return goldast.WalkContinue, nil
		}
		start, ok := markdownNodeFirstLineStart(item)
		if !ok {
			return goldast.WalkSkipChildren, nil
		}
		lineStart := markdownLineStart(raw, start)
		lineEnd := markdownLineEnd(raw, lineStart)
		info, ok := parseMarkdownListItemLine(raw[lineStart:lineEnd])
		if !ok {
			return goldast.WalkSkipChildren, nil
		}
		list, _ := item.Parent().(*goldast.List)
		complex := list == nil || list.Parent() != doc || item.ChildCount() != 1 || markdownNodeLineCount(item.FirstChild()) != 1
		items = append(items, markdownListItem{
			LineStart:        lineStart,
			LineEnd:          lineEnd,
			Normalized:       normalizeMarkdownItemText(info.Text),
			Task:             info.Task,
			TaskStatus:       info.TaskStatus,
			TaskStatusOffset: lineStart + info.TaskStatusOffset,
			Numbered:         info.Numbered,
			Number:           info.Number,
			Delimiter:        info.Delimiter,
			Indent:           info.Indent,
			Marker:           info.Marker,
			Complex:          complex,
		})
		return goldast.WalkContinue, nil
	})
	return items
}

func markdownNodeFirstLineStart(n goldast.Node) (int, bool) {
	if start, ok := markdownNodeOwnFirstLineStart(n); ok {
		return start, true
	}
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if start, ok := markdownNodeFirstLineStart(child); ok {
			return start, true
		}
	}
	return 0, false
}

func markdownNodeOwnFirstLineStart(n goldast.Node) (int, bool) {
	switch x := n.(type) {
	case *goldast.Paragraph:
		if x.Lines().Len() > 0 {
			return x.Lines().At(0).Start, true
		}
	case *goldast.TextBlock:
		if x.Lines().Len() > 0 {
			return x.Lines().At(0).Start, true
		}
	}
	return 0, false
}

func markdownNodeLineCount(n goldast.Node) int {
	switch x := n.(type) {
	case *goldast.Paragraph:
		return x.Lines().Len()
	case *goldast.TextBlock:
		return x.Lines().Len()
	default:
		return 0
	}
}

type markdownListItemLine struct {
	Text             string
	Task             bool
	TaskStatus       byte
	TaskStatusOffset int
	Numbered         bool
	Number           int
	Delimiter        byte
	Indent           string
	Marker           string
}

func parseMarkdownListItemLine(line []byte) (markdownListItemLine, bool) {
	line = bytes.TrimRight(line, "\r\n")
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if i > 3 || i >= len(line) {
		return markdownListItemLine{}, false
	}
	indent := string(line[:i])
	numbered := false
	number := 0
	delimiter := byte(0)
	markerStart := i
	if line[i] == '-' || line[i] == '+' || line[i] == '*' {
		delimiter = line[i]
		i++
	} else if line[i] >= '0' && line[i] <= '9' {
		start := i
		for i < len(line) && line[i] >= '0' && line[i] <= '9' {
			i++
		}
		if i == start || i-start > 9 || i >= len(line) || (line[i] != '.' && line[i] != ')') {
			return markdownListItemLine{}, false
		}
		n, err := strconv.Atoi(string(line[start:i]))
		if err != nil {
			return markdownListItemLine{}, false
		}
		number = n
		delimiter = line[i]
		i++
		numbered = true
	} else {
		return markdownListItemLine{}, false
	}
	marker := string(line[markerStart:i])
	if i < len(line) && line[i] != ' ' && line[i] != '\t' {
		return markdownListItemLine{}, false
	}
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	task := false
	taskStatus := byte(0)
	taskStatusOffset := 0
	if i+2 < len(line) && line[i] == '[' && line[i+2] == ']' {
		task = true
		taskStatus = line[i+1]
		taskStatusOffset = i + 1
		i += 3
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
	}
	return markdownListItemLine{
		Text:             string(line[i:]),
		Task:             task,
		TaskStatus:       taskStatus,
		TaskStatusOffset: taskStatusOffset,
		Numbered:         numbered,
		Number:           number,
		Delimiter:        delimiter,
		Indent:           indent,
		Marker:           marker,
	}, true
}

func normalizeMarkdownItemText(source string) string {
	if parsed, ok := parseMarkdownListItemLine([]byte(source)); ok {
		source = parsed.Text
	}
	source = stripDataviewInlineFields(source)
	source = stripTrailingMarkdownReferenceAnnotationLinks(source)
	source = markdownInlineText(source)
	return strings.TrimSpace(source)
}

func stripDataviewInlineFields(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		close := byte(0)
		switch source[i] {
		case '[':
			close = ']'
		case '(':
			close = ')'
		}
		if close == 0 {
			out.WriteByte(source[i])
			continue
		}
		end := strings.IndexByte(source[i+1:], close)
		if end < 0 {
			out.WriteByte(source[i])
			continue
		}
		inner := source[i+1 : i+1+end]
		if strings.Contains(inner, "::") {
			i += end + 1
			continue
		}
		out.WriteByte(source[i])
	}
	return out.String()
}

func stripTrailingMarkdownReferenceAnnotationLinks(source string) string {
	for {
		trimmed := strings.TrimRightFunc(source, unicode.IsSpace)
		start, label, ok := trailingMarkdownLink(trimmed)
		if !ok || !markdownReferenceAnnotationLinkLabel(label) {
			return source
		}
		source = trimmed[:start]
	}
}

func markdownInlineText(source string) string {
	raw := []byte(source)
	doc := markdownEngine.Parser().Parse(goldtext.NewReader(raw))
	var out strings.Builder
	_ = goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		switch x := n.(type) {
		case *goldast.Text:
			out.Write(x.Value(raw))
			if x.SoftLineBreak() || x.HardLineBreak() {
				out.WriteByte(' ')
			}
			return goldast.WalkSkipChildren, nil
		case *goldast.String:
			out.Write(x.Value)
			return goldast.WalkSkipChildren, nil
		case *goldast.AutoLink:
			out.Write(x.Label(raw))
			return goldast.WalkSkipChildren, nil
		case *goldast.RawHTML:
			return goldast.WalkSkipChildren, nil
		default:
			return goldast.WalkContinue, nil
		}
	})
	return out.String()
}

func trailingMarkdownLink(source string) (int, string, bool) {
	if len(source) == 0 || source[len(source)-1] != ')' || markdownSourceEscaped(source, len(source)-1) {
		return 0, "", false
	}
	destStart, ok := matchingMarkdownSourceOpen(source, len(source)-1, '(', ')')
	if !ok || destStart == 0 {
		return 0, "", false
	}
	labelEnd := destStart - 1
	if source[labelEnd] != ']' || markdownSourceEscaped(source, labelEnd) {
		return 0, "", false
	}
	labelStart, ok := matchingMarkdownSourceOpen(source, labelEnd, '[', ']')
	if !ok {
		return 0, "", false
	}
	if labelStart > 0 && source[labelStart-1] == '!' && !markdownSourceEscaped(source, labelStart-1) {
		return 0, "", false
	}
	return labelStart, source[labelStart+1 : labelEnd], true
}

func matchingMarkdownSourceOpen(source string, closeIndex int, open, close byte) (int, bool) {
	depth := 0
	for i := closeIndex; i >= 0; i-- {
		if markdownSourceEscaped(source, i) {
			continue
		}
		switch source[i] {
		case close:
			depth++
		case open:
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func markdownSourceEscaped(source string, index int) bool {
	backslashes := 0
	for i := index - 1; i >= 0 && source[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func markdownReferenceAnnotationLinkLabel(label string) bool {
	label = strings.TrimSpace(label)
	for len(label) >= 2 && label[0] == '[' && label[len(label)-1] == ']' {
		label = strings.TrimSpace(label[1 : len(label)-1])
	}
	label = strings.TrimPrefix(label, "^")
	label = strings.TrimSpace(label)
	if label == "" {
		return false
	}
	hasDigit := false
	for _, r := range label {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsSpace(r) || r == ',' || r == ';' || r == ':' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return hasDigit
}
