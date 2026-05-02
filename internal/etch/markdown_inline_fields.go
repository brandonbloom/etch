package etch

import (
	"bytes"
	"strings"

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
