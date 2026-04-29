package etch

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
)

func evalStructuredBytes(path, part, selector, verb, rawValue string, before []byte) ([]byte, bool, error) {
	value := parseValue(rawValue)
	switch {
	case part == "frontmatter":
		return evalFrontmatter(path, selector, verb, value, before)
	case isJSONPath(path):
		root, bom, err := decodeJSON(before)
		if err != nil {
			return nil, false, err
		}
		next, changed, err := mutateStructuredValue(root, selector, verb, value)
		if err != nil {
			return nil, false, err
		}
		out, err := encodeJSON(next, bom)
		return out, changed || !bytes.Equal(out, before), err
	case isYAMLPath(path):
		return evalYAML(selector, verb, value, before)
	default:
		return nil, false, failf("cannot infer structured format for %s", path)
	}
}

func evalYAML(selector, verb string, value any, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in YAML input")
	}
	var root any
	if len(strings.TrimSpace(string(raw))) == 0 {
		root = map[string]any{}
	} else if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, false, err
	}
	root = yamlToJSON(root)
	next, changed, err := mutateStructuredValue(root, selector, verb, value)
	if err != nil {
		return nil, false, err
	}
	out, err := marshalYAML(next)
	if err != nil {
		return nil, false, err
	}
	out = ensureTrailingNewline(out)
	out = withUTF8BOM(out, bom)
	return out, changed || !bytes.Equal(out, before), nil
}

func yamlToJSON(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[k] = yamlToJSON(v)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, v := range x {
			m[fmt.Sprint(k)] = yamlToJSON(v)
		}
		return m
	case []any:
		out := make([]any, len(x))
		for i, v := range x {
			out[i] = yamlToJSON(v)
		}
		return out
	default:
		return x
	}
}

func marshalYAML(v any) ([]byte, error) {
	return yaml.MarshalWithOptions(v, yaml.UseLiteralStyleIfMultiline(true))
}

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
	var root any = map[string]any{}
	if strings.TrimSpace(string(fm)) != "" {
		if err := yaml.Unmarshal(fm, &root); err != nil {
			return nil, false, err
		}
		root = yamlToJSON(root)
	}
	next, changed, err := mutateStructuredValue(root, selector, verb, value)
	if err != nil {
		return nil, false, err
	}
	yamlBytes, err := marshalYAML(next)
	if err != nil {
		return nil, false, err
	}
	yamlBytes = bytes.TrimRight(yamlBytes, "\n")
	var out []byte
	out = append(out, []byte("---\n")...)
	out = append(out, yamlBytes...)
	out = append(out, []byte("\n---\n")...)
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

func evalReplaceSection(path, heading, content string, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in Markdown input")
	}
	s := string(raw)
	lines := strings.SplitAfter(s, "\n")
	offset := 0
	type hit struct {
		lineStart int
		bodyStart int
		level     int
	}
	var hits []hit
	for _, line := range lines {
		text := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(text) == heading && atxLevel(text) > 0 {
			hits = append(hits, hit{lineStart: offset, bodyStart: offset + len(line), level: atxLevel(text)})
		}
		offset += len(line)
	}
	if len(hits) == 0 {
		return nil, false, failf("heading %q not found in %s", heading, path)
	}
	if len(hits) > 1 {
		return nil, false, failf("heading %q is ambiguous in %s", heading, path)
	}
	h := hits[0]
	end := len(s)
	offset = 0
	for _, line := range lines {
		if offset <= h.lineStart {
			offset += len(line)
			continue
		}
		text := strings.TrimRight(line, "\r\n")
		lvl := atxLevel(text)
		if lvl > 0 && lvl <= h.level {
			end = offset
			break
		}
		offset += len(line)
	}
	repl := content
	if repl != "" && !strings.HasSuffix(repl, "\n") {
		repl += "\n"
	}
	out := []byte(s[:h.bodyStart] + repl + s[end:])
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func atxLevel(line string) int {
	trim := strings.TrimLeft(line, " \t")
	n := 0
	for n < len(trim) && trim[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if len(trim) > n && trim[n] != ' ' && trim[n] != '\t' {
		return 0
	}
	return n
}

type tableData struct {
	Header []string
	Rows   [][]string
}

func evalTable(path string, op Operation, before []byte) ([]byte, bool, error) {
	if isCSVPath(path) {
		return evalCSVTable(op, before)
	}
	if isMarkdownPath(path) {
		return evalMarkdownTable(op, before)
	}
	return nil, false, failf("cannot infer table format for %s", path)
}

func evalCSVTable(op Operation, before []byte) ([]byte, bool, error) {
	r := csv.NewReader(bytes.NewReader(before))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, false, err
	}
	if len(records) == 0 {
		records = [][]string{{}}
	}
	td := tableData{Header: append([]string(nil), records[0]...)}
	for _, row := range records[1:] {
		td.Rows = append(td.Rows, normalizeRow(row, len(td.Header)))
	}
	if err := mutateTable(&td, op); err != nil {
		return nil, false, err
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	if err := w.Write(td.Header); err != nil {
		return nil, false, err
	}
	for _, row := range td.Rows {
		if err := w.Write(normalizeRow(row, len(td.Header))); err != nil {
			return nil, false, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, false, err
	}
	out := b.Bytes()
	return out, !bytes.Equal(out, before), nil
}

func normalizeRow(row []string, n int) []string {
	out := make([]string, n)
	copy(out, row)
	return out
}

func mutateTable(td *tableData, op Operation) error {
	switch op.Verb {
	case "table set":
		rows, cols, err := resolveTableRange(*td, op.Target.Range)
		if err != nil {
			return err
		}
		for _, r := range rows {
			for _, c := range cols {
				td.Rows[r][c] = op.Value
			}
		}
	case "table row append":
		row, err := parseTableRow(op.Value, td.Header)
		if err != nil {
			return err
		}
		td.Rows = append(td.Rows, row)
	case "table row insert":
		row, err := parseTableRow(op.Value, td.Header)
		if err != nil {
			return err
		}
		before, key, _ := strings.Cut(op.Target.Row, " ")
		idx, err := resolveSingleRow(*td, key)
		if err != nil {
			return err
		}
		if before == "--after" {
			idx++
		}
		td.Rows = append(td.Rows[:idx], append([][]string{row}, td.Rows[idx:]...)...)
	case "table row delete":
		rows, err := resolveRows(*td, op.Target.Row)
		if err != nil {
			return err
		}
		remove := map[int]bool{}
		for _, r := range rows {
			remove[r] = true
		}
		out := td.Rows[:0]
		for i, row := range td.Rows {
			if !remove[i] {
				out = append(out, row)
			}
		}
		td.Rows = out
	case "table column add":
		var knobs map[string]string
		_ = json.Unmarshal([]byte(op.Value), &knobs)
		if _, err := columnIndex(td.Header, op.Target.Column); err == nil {
			return nil
		}
		pos := len(td.Header)
		if knobs["after"] != "" {
			idx, err := columnIndex(td.Header, knobs["after"])
			if err != nil {
				return err
			}
			pos = idx + 1
		}
		td.Header = append(td.Header[:pos], append([]string{op.Target.Column}, td.Header[pos:]...)...)
		for i := range td.Rows {
			td.Rows[i] = append(td.Rows[i][:pos], append([]string{knobs["default"]}, td.Rows[i][pos:]...)...)
		}
	case "table column rename":
		idx, err := columnIndex(td.Header, op.Target.Column)
		if err != nil {
			return err
		}
		td.Header[idx] = op.Value
	case "table column delete":
		idx, err := columnIndex(td.Header, op.Target.Column)
		if err != nil {
			return err
		}
		td.Header = append(td.Header[:idx], td.Header[idx+1:]...)
		for i := range td.Rows {
			td.Rows[i] = append(td.Rows[i][:idx], td.Rows[i][idx+1:]...)
		}
	default:
		return usagef("unsupported table operation %s", op.Verb)
	}
	return nil
}

func parseTableRow(raw string, header []string) ([]string, error) {
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, failf("row-json must be a JSON object with string values")
	}
	known := map[string]bool{}
	for _, h := range header {
		known[h] = true
	}
	for k := range m {
		if !known[k] {
			return nil, failf("unknown table column %q", k)
		}
	}
	row := make([]string, len(header))
	for i, h := range header {
		row[i] = m[h]
	}
	return row, nil
}

func resolveTableRange(td tableData, spec string) ([]int, []int, error) {
	a, b, ok := strings.Cut(spec, ",")
	if !ok {
		return nil, nil, failf("table range must be <rows>,<columns>")
	}
	rows, err := resolveRows(td, a)
	if err != nil {
		return nil, nil, err
	}
	cols, err := resolveColumns(td.Header, b)
	if err != nil {
		return nil, nil, err
	}
	return rows, cols, nil
}

func resolveRows(td tableData, spec string) ([]int, error) {
	if spec == "all" {
		rows := make([]int, len(td.Rows))
		for i := range rows {
			rows[i] = i
		}
		return rows, nil
	}
	if strings.HasPrefix(spec, "@") {
		if strings.Contains(spec, "..") {
			a, b, _ := strings.Cut(spec, "..")
			start, err := parseOrdinal(a)
			if err != nil {
				return nil, err
			}
			end, err := parseOrdinal(b)
			if err != nil {
				return nil, err
			}
			if start < 0 || end >= len(td.Rows) || start > end {
				return nil, failf("row range %s out of range", spec)
			}
			rows := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				rows = append(rows, i)
			}
			return rows, nil
		}
		idx, err := parseOrdinal(spec)
		if err != nil {
			return nil, err
		}
		if idx < 0 || idx >= len(td.Rows) {
			return nil, failf("row %s out of range", spec)
		}
		return []int{idx}, nil
	}
	idx, err := resolveSingleRow(td, spec)
	if err != nil {
		return nil, err
	}
	return []int{idx}, nil
}

func resolveSingleRow(td tableData, spec string) (int, error) {
	if strings.HasPrefix(spec, "@") {
		idx, err := parseOrdinal(spec)
		if err != nil {
			return 0, err
		}
		if idx < 0 || idx >= len(td.Rows) {
			return 0, failf("row %s out of range", spec)
		}
		return idx, nil
	}
	key, val, ok := strings.Cut(spec, "=")
	if !ok {
		return 0, failf("row selector must be an ordinal or key=value")
	}
	key = unbracket(key)
	val = strings.Trim(val, `"`)
	col, err := columnIndex(td.Header, key)
	if err != nil {
		return 0, err
	}
	match := -1
	for i, row := range td.Rows {
		if row[col] == val {
			if match != -1 {
				return 0, failf("row selector %s is ambiguous", spec)
			}
			match = i
		}
	}
	if match == -1 {
		return 0, failf("row selector %s did not match", spec)
	}
	return match, nil
}

func resolveColumns(header []string, spec string) ([]int, error) {
	if spec == "all" {
		cols := make([]int, len(header))
		for i := range cols {
			cols[i] = i
		}
		return cols, nil
	}
	if strings.HasPrefix(spec, "@") {
		if strings.Contains(spec, "..") {
			a, b, _ := strings.Cut(spec, "..")
			start, err := parseOrdinal(a)
			if err != nil {
				return nil, err
			}
			end, err := parseOrdinal(b)
			if err != nil {
				return nil, err
			}
			if start < 0 || end >= len(header) || start > end {
				return nil, failf("column range %s out of range", spec)
			}
			cols := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				cols = append(cols, i)
			}
			return cols, nil
		}
		idx, err := parseOrdinal(spec)
		if err != nil {
			return nil, err
		}
		if idx < 0 || idx >= len(header) {
			return nil, failf("column %s out of range", spec)
		}
		return []int{idx}, nil
	}
	if strings.Contains(spec, "..") {
		a, b, _ := strings.Cut(spec, "..")
		start, err := columnIndex(header, unbracket(a))
		if err != nil {
			return nil, err
		}
		end, err := columnIndex(header, unbracket(b))
		if err != nil {
			return nil, err
		}
		if start > end {
			return nil, failf("column range %s is reversed", spec)
		}
		cols := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			cols = append(cols, i)
		}
		return cols, nil
	}
	idx, err := columnIndex(header, unbracket(spec))
	if err != nil {
		return nil, err
	}
	return []int{idx}, nil
}

func parseOrdinal(s string) (int, error) {
	if !strings.HasPrefix(s, "@") {
		return 0, failf("ordinal %q must start with @", s)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(s, "@"))
	if err != nil {
		return 0, failf("invalid ordinal %q", s)
	}
	return n, nil
}

func columnIndex(header []string, name string) (int, error) {
	for i, h := range header {
		if h == name {
			return i, nil
		}
	}
	return 0, failf("unknown table column %q", name)
}

func unbracket(s string) string {
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	}
	return s
}

type mdTableBlock struct {
	start int
	end   int
	table tableData
}

func evalMarkdownTable(op Operation, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in Markdown input")
	}
	blocks, err := findMarkdownTables(string(raw), op.Target.Scope)
	if err != nil {
		return nil, false, err
	}
	if len(blocks) == 0 {
		return nil, false, failf("no markdown table found in scope")
	}
	idx := 0
	if op.Target.Table != "" {
		n, err := parseOrdinal(op.Target.Table)
		if err != nil {
			return nil, false, err
		}
		idx = n
	} else if len(blocks) != 1 {
		return nil, false, failf("scope contains %d tables; supply an ordinal", len(blocks))
	}
	if idx < 0 || idx >= len(blocks) {
		return nil, false, failf("table ordinal @%d out of range", idx)
	}
	block := blocks[idx]
	td := block.table
	if err := mutateTable(&td, op); err != nil {
		return nil, false, err
	}
	rendered := renderMarkdownTable(td)
	s := string(raw)
	out := []byte(s[:block.start] + rendered + s[block.end:])
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func findMarkdownTables(s, scope string) ([]mdTableBlock, error) {
	scopeStart, scopeEnd, err := markdownScope(s, scope)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(s[scopeStart:scopeEnd], "\n")
	offset := scopeStart
	var blocks []mdTableBlock
	for i := 0; i+1 < len(lines); i++ {
		header := strings.TrimSpace(lines[i])
		sep := strings.TrimSpace(lines[i+1])
		if !isPipeRow(header) || !isMarkdownSeparator(sep) {
			offset += len(lines[i])
			continue
		}
		start := offset
		tableLines := []string{lines[i], lines[i+1]}
		offset += len(lines[i]) + len(lines[i+1])
		i++
		for i+1 < len(lines) && isPipeRow(strings.TrimSpace(lines[i+1])) {
			i++
			tableLines = append(tableLines, lines[i])
			offset += len(lines[i])
		}
		td, err := parseMarkdownTable(tableLines)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, mdTableBlock{start: start, end: offset, table: td})
	}
	return blocks, nil
}

func markdownScope(s, scope string) (int, int, error) {
	if scope == "" || scope == "doc" {
		return 0, len(s), nil
	}
	lines := strings.SplitAfter(s, "\n")
	offset := 0
	foundStart := -1
	level := 0
	for _, line := range lines {
		text := strings.TrimRight(line, "\r\n")
		lvl := atxLevel(text)
		if foundStart == -1 {
			if strings.TrimSpace(text) == scope && lvl > 0 {
				foundStart = offset + len(line)
				level = lvl
			}
		} else if lvl > 0 && lvl <= level {
			return foundStart, offset, nil
		}
		offset += len(line)
	}
	if foundStart == -1 {
		return 0, 0, failf("markdown scope %q not found", scope)
	}
	return foundStart, len(s), nil
}

func isPipeRow(s string) bool {
	return strings.Contains(s, "|")
}

func isMarkdownSeparator(s string) bool {
	cells := splitPipeRow(s)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		t := strings.Trim(cell, " :-")
		if t != "" {
			return false
		}
		if !strings.Contains(cell, "-") {
			return false
		}
	}
	return true
}

func parseMarkdownTable(lines []string) (tableData, error) {
	header := splitPipeRow(lines[0])
	td := tableData{Header: header}
	for _, line := range lines[2:] {
		row := splitPipeRow(line)
		td.Rows = append(td.Rows, normalizeRow(row, len(header)))
	}
	return td, nil
}

func splitPipeRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func renderMarkdownTable(td tableData) string {
	var b strings.Builder
	write := func(row []string) {
		b.WriteString("|")
		for _, cell := range normalizeRow(row, len(td.Header)) {
			b.WriteByte(' ')
			b.WriteString(strings.ReplaceAll(cell, "|", "\\|"))
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}
	write(td.Header)
	sep := make([]string, len(td.Header))
	for i := range sep {
		sep[i] = "---"
	}
	write(sep)
	for _, row := range td.Rows {
		write(row)
	}
	return b.String()
}
