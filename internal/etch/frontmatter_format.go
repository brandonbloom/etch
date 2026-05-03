package etch

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

func evalFrontmatter(path, selector, verb string, value any, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in Markdown input")
	}
	fm, body, had, err := splitFrontmatter(raw)
	if err != nil {
		return nil, false, err
	}
	if !had && (verb == "delete" || verb == "remove") {
		return before, false, nil
	}
	file, err := parseYAMLFile(fm)
	if err != nil {
		return nil, false, err
	}
	changed, err := mutateYAMLFile(file, selector, verb, value)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return before, false, nil
	}
	yamlBytes := bytes.TrimRight([]byte(firstYAMLDocumentString(file)), "\n")
	var out []byte
	out = append(out, []byte("---\n")...)
	out = append(out, yamlBytes...)
	out = append(out, []byte("\n---\n")...)
	if !had && len(body) > 0 && !markdownBodyStartsWithBlankLine(body) {
		out = append(out, '\n')
	}
	out = append(out, body...)
	out = withUTF8BOM(out, bom)
	return out, changed || !bytes.Equal(out, before), nil
}

func splitFrontmatter(b []byte) (fm, body []byte, had bool, err error) {
	s := string(b)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, b, false, nil
	}
	lines := strings.SplitAfter(s, "\n")
	offset := len(lines[0])
	for i := 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if line == "---" || line == "..." {
			end := offset
			bodyStart := offset + len(lines[i])
			return []byte(s[len(lines[0]):end]), []byte(s[bodyStart:]), true, nil
		}
		offset += len(lines[i])
	}
	return nil, nil, false, failf("unterminated YAML frontmatter in markdown file")
}
