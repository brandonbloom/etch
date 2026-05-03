package etch

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

var markdownEngine = goldmark.New(goldmark.WithExtensions(extension.GFM))

type markdownSection struct {
	Heading markdownHeading
	BodyEnd int
}

type markdownHeadingSelector struct {
	Level    int
	HasLevel bool
	Content  string
}

type markdownHeading struct {
	Level     int
	Content   string
	LineStart int
	BodyStart int
}

func markdownNewline(raw []byte) string {
	if bytes.Contains(raw, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func markdownBlankStringLine(line string) bool {
	return strings.Trim(line, " \t") == ""
}

func trimTrailingMarkdownBlankLines(b []byte) []byte {
	end := len(b)
	for end > 0 {
		lineEnd := end
		if b[lineEnd-1] == '\n' {
			lineEnd--
			if lineEnd > 0 && b[lineEnd-1] == '\r' {
				lineEnd--
			}
		}
		lineStart := bytes.LastIndexByte(b[:lineEnd], '\n') + 1
		if !markdownBlankBytesLine(b[lineStart:lineEnd]) {
			return b[:lineEnd]
		}
		end = lineStart
	}
	return b[:0]
}

func trimLeadingMarkdownBlankLines(b []byte) []byte {
	start := 0
	for start < len(b) {
		lineEnd := markdownLineEnd(b, start)
		contentEnd := lineEnd
		if contentEnd > start && b[contentEnd-1] == '\n' {
			contentEnd--
			if contentEnd > start && b[contentEnd-1] == '\r' {
				contentEnd--
			}
		}
		if !markdownBlankBytesLine(b[start:contentEnd]) {
			return b[start:]
		}
		start = lineEnd
	}
	return b[len(b):]
}

func markdownBodyStartsWithBlankLine(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	lineEnd := markdownLineEnd(b, 0)
	contentEnd := lineEnd
	if contentEnd > 0 && b[contentEnd-1] == '\n' {
		contentEnd--
		if contentEnd > 0 && b[contentEnd-1] == '\r' {
			contentEnd--
		}
	}
	return markdownBlankBytesLine(b[:contentEnd])
}

func markdownBodyEndsWithBlankLine(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	end := len(b)
	if b[end-1] == '\n' {
		end--
		if end > 0 && b[end-1] == '\r' {
			end--
		}
	}
	if end == 0 {
		return true
	}
	lineStart := bytes.LastIndexByte(b[:end], '\n') + 1
	return markdownBlankBytesLine(b[lineStart:end])
}

func markdownBlankBytesLine(line []byte) bool {
	return len(bytes.Trim(line, " \t\r")) == 0
}

func parseMarkdownHeadingSelector(heading string) (markdownHeadingSelector, error) {
	if strings.ContainsAny(heading, "\r\n") {
		return markdownHeadingSelector{}, usagef("section selector must be a single Markdown heading or title")
	}
	line := strings.TrimSpace(heading)
	if line == "" {
		return markdownHeadingSelector{}, usagef("section selector must not be blank")
	}
	level, content, ok := parseATXHeadingLine([]byte(line))
	if ok {
		return markdownHeadingSelector{Level: level, HasLevel: true, Content: content}, nil
	}
	return markdownHeadingSelector{Content: line}, nil
}

func markdownHeadings(raw []byte) []markdownHeading {
	doc := parseMarkdownDocument(raw)
	var headings []markdownHeading
	_ = goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		heading, ok := n.(*goldast.Heading)
		if !ok {
			return goldast.WalkContinue, nil
		}
		if heading.Parent() != doc || heading.Lines().Len() == 0 {
			return goldast.WalkSkipChildren, nil
		}
		lineStart, bodyStart := markdownHeadingBlockRange(raw, heading)
		headings = append(headings, markdownHeading{
			Level:     heading.Level,
			Content:   markdownHeadingContent(raw, heading, lineStart),
			LineStart: lineStart,
			BodyStart: bodyStart,
		})
		return goldast.WalkSkipChildren, nil
	})
	return headings
}

func parseMarkdownDocument(raw []byte) goldast.Node {
	return markdownEngine.Parser().Parse(text.NewReader(raw))
}

func markdownSections(raw []byte, selector string) ([]markdownSection, error) {
	target, err := parseMarkdownHeadingSelector(selector)
	if err != nil {
		return nil, err
	}
	headings := markdownHeadings(raw)
	var sections []markdownSection
	for i, h := range headings {
		if (!target.HasLevel || h.Level == target.Level) && h.Content == target.Content {
			sections = append(sections, markdownSection{
				Heading: h,
				BodyEnd: markdownSectionBodyEnd(raw, headings, i),
			})
		}
	}
	return sections, nil
}

func markdownSectionBodyEnd(raw []byte, headings []markdownHeading, idx int) int {
	for _, next := range headings[idx+1:] {
		if next.Level <= headings[idx].Level {
			return next.LineStart
		}
	}
	return len(raw)
}

func markdownHeadingContent(raw []byte, heading *goldast.Heading, lineStart int) string {
	lineEnd := markdownLineEnd(raw, lineStart)
	if level, content, ok := parseATXHeadingLine(raw[lineStart:lineEnd]); ok {
		_ = level
		return content
	}
	return strings.TrimSpace(string(heading.Lines().Value(raw)))
}

func markdownHeadingBlockRange(raw []byte, heading *goldast.Heading) (int, int) {
	lines := heading.Lines()
	first := lines.At(0)
	start := markdownLineStart(raw, first.Start)
	firstEnd := markdownLineEnd(raw, start)
	if markdownLineIsATXHeading(raw[start:firstEnd]) {
		return start, firstEnd
	}
	last := lines.At(lines.Len() - 1)
	textLineEnd := markdownLineEnd(raw, last.Stop)
	return start, markdownLineEnd(raw, textLineEnd)
}

func markdownLineStart(raw []byte, pos int) int {
	if pos > len(raw) {
		pos = len(raw)
	}
	return bytes.LastIndexByte(raw[:pos], '\n') + 1
}

func markdownLineEnd(raw []byte, pos int) int {
	if pos >= len(raw) {
		return len(raw)
	}
	if i := bytes.IndexByte(raw[pos:], '\n'); i >= 0 {
		return pos + i + 1
	}
	return len(raw)
}

func markdownLineIsATXHeading(line []byte) bool {
	_, _, ok := parseATXHeadingLine(line)
	return ok
}

func parseATXHeadingLine(line []byte) (int, string, bool) {
	line = bytes.TrimLeft(line, " \t")
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	if n < len(line) && line[n] != ' ' && line[n] != '\t' && line[n] != '\r' && line[n] != '\n' {
		return 0, "", false
	}
	return n, stripClosingATXMarkers(strings.TrimSpace(string(line[n:]))), true
}

func stripClosingATXMarkers(content string) string {
	end := len(content)
	for end > 0 && content[end-1] == '#' {
		end--
	}
	if end == len(content) {
		return content
	}
	if end == 0 {
		return ""
	}
	if end > 0 && (content[end-1] == ' ' || content[end-1] == '\t') {
		return strings.TrimSpace(content[:end])
	}
	return content
}
