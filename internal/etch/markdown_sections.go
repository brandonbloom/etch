package etch

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

func evalMarkdownSection(path, verb, heading, content string, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in Markdown input")
	}
	section, err := resolveMarkdownSection(raw, path, heading)
	if err != nil {
		return nil, false, err
	}
	newline := markdownNewline(raw)
	repl, err := markdownSectionBody(raw, section, verb, content, newline)
	if err != nil {
		return nil, false, err
	}
	out := make([]byte, 0, len(raw)-(section.BodyEnd-section.Heading.BodyStart)+len(repl))
	out = append(out, raw[:section.Heading.BodyStart]...)
	out = append(out, repl...)
	out = append(out, raw[section.BodyEnd:]...)
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func markdownSectionBody(raw []byte, section markdownSection, verb, content, newline string) ([]byte, error) {
	body := raw[section.Heading.BodyStart:section.BodyEnd]
	prefix := raw[:section.Heading.BodyStart]
	needsHeadingNewline := len(prefix) > 0 && !bytes.HasSuffix(prefix, []byte("\n"))
	switch verb {
	case "section replace":
		return replaceSectionBody(body, content, newline, needsHeadingNewline), nil
	case "section append":
		fragment, err := markdownBlockFragment(content, newline)
		if err != nil {
			return nil, err
		}
		existing := trimTrailingMarkdownBlankLines(body)
		return appendSectionBody(existing, fragment, newline, needsHeadingNewline), nil
	case "section prepend":
		fragment, err := markdownBlockFragment(content, newline)
		if err != nil {
			return nil, err
		}
		existing := trimLeadingMarkdownBlankLines(body)
		return prependSectionBody(existing, fragment, newline, needsHeadingNewline), nil
	default:
		return nil, usagef("unknown section verb %s", verb)
	}
}

func replaceSectionBody(existing []byte, content, newline string, needsHeadingNewline bool) []byte {
	fragment := markdownBlockFragmentContent(content, newline)
	if len(fragment) == 0 {
		return nil
	}
	var out []byte
	if needsHeadingNewline {
		out = append(out, newline...)
	}
	if markdownBodyStartsWithBlankLine(existing) {
		out = append(out, newline...)
	}
	out = append(out, fragment...)
	out = append(out, newline...)
	if markdownBodyEndsWithBlankLine(existing) {
		out = append(out, newline...)
	}
	return out
}

func appendSectionBody(existing, fragment []byte, newline string, needsHeadingNewline bool) []byte {
	var out []byte
	if needsHeadingNewline {
		out = append(out, newline...)
	}
	if len(bytes.Trim(existing, " \t\r\n")) == 0 {
		out = append(out, fragment...)
		out = append(out, newline...)
		return out
	}
	out = append(out, existing...)
	out = append(out, newline...)
	out = append(out, newline...)
	out = append(out, fragment...)
	out = append(out, newline...)
	return out
}

func prependSectionBody(existing, fragment []byte, newline string, needsHeadingNewline bool) []byte {
	var out []byte
	if needsHeadingNewline {
		out = append(out, newline...)
	}
	if len(bytes.Trim(existing, " \t\r\n")) == 0 {
		out = append(out, fragment...)
		out = append(out, newline...)
		return out
	}
	out = append(out, fragment...)
	out = append(out, newline...)
	out = append(out, newline...)
	out = append(out, existing...)
	return out
}

func markdownBlockFragment(content, newline string) ([]byte, error) {
	fragment := markdownBlockFragmentContent(content, newline)
	if len(fragment) == 0 {
		return nil, usagef("section fragment must not be blank")
	}
	return fragment, nil
}

func markdownBlockFragmentContent(content, newline string) []byte {
	s := strings.ReplaceAll(content, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	start, end := 0, len(lines)
	for start < end && markdownBlankStringLine(lines[start]) {
		start++
	}
	for end > start && markdownBlankStringLine(lines[end-1]) {
		end--
	}
	if start == end {
		return nil
	}
	return []byte(strings.Join(lines[start:end], newline))
}
