package etch

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/brandonbloom/etch/internal/jsonx"
	"github.com/yuin/goldmark"
	goldast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	mdast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var markdownEngine = goldmark.New(goldmark.WithExtensions(extension.GFM))

func evalStructuredBytes(path, part, selector, verb, rawValue string, before []byte) ([]byte, bool, error) {
	switch {
	case part == "frontmatter":
		value := parseValue(rawValue)
		return evalFrontmatter(path, selector, verb, value, before)
	case isJSONPath(path):
		return evalJSON(selector, verb, rawValue, before)
	case isYAMLPath(path):
		value := parseValue(rawValue)
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
	file, err := parseYAMLFile(raw)
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
	out := []byte(file.String())
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
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
	sections, err := markdownSections(raw, heading)
	if err != nil {
		return nil, false, err
	}
	if len(sections) == 0 {
		return nil, false, failf("heading %q not found in %s", heading, path)
	}
	if len(sections) > 1 {
		return nil, false, failf("heading %q is ambiguous in %s", heading, path)
	}
	section := sections[0]
	repl := content
	if repl != "" && !strings.HasSuffix(repl, "\n") {
		repl += "\n"
	}
	out := make([]byte, 0, len(raw)-(section.BodyEnd-section.Heading.BodyStart)+len(repl))
	out = append(out, raw[:section.Heading.BodyStart]...)
	out = append(out, repl...)
	out = append(out, raw[section.BodyEnd:]...)
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

type markdownSection struct {
	Heading markdownHeading
	BodyEnd int
}

type markdownHeading struct {
	Level     int
	Content   string
	LineStart int
	BodyStart int
}

func parseMarkdownHeadingSelector(heading string) (markdownHeading, error) {
	raw := []byte(heading)
	if !bytes.HasSuffix(raw, []byte("\n")) {
		raw = append(raw, '\n')
	}
	headings := markdownHeadings(raw)
	if len(headings) != 1 || strings.TrimSpace(string(raw[headings[0].BodyStart:])) != "" {
		return markdownHeading{}, usagef("section selector must be a single Markdown heading")
	}
	return headings[0], nil
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
			Content:   strings.TrimSpace(string(heading.Lines().Value(raw))),
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
		if h.Level == target.Level && h.Content == target.Content {
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
	line = bytes.TrimLeft(line, " \t")
	n := 0
	for n < len(line) && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return false
	}
	return n == len(line) || line[n] == ' ' || line[n] == '\t' || line[n] == '\r' || line[n] == '\n'
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
	_ = w.Write(td.Header)
	for _, row := range td.Rows {
		_ = w.Write(normalizeRow(row, len(td.Header)))
	}
	w.Flush()
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
		_ = jsonx.Unmarshal([]byte(op.Value), &knobs)
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
	if err := jsonx.Unmarshal([]byte(raw), &m); err != nil {
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
	blocks, err := findMarkdownTables(raw, op.Target.Scope)
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
	rendered := []byte(renderMarkdownTable(td))
	out := make([]byte, 0, len(raw)-(block.end-block.start)+len(rendered))
	out = append(out, raw[:block.start]...)
	out = append(out, rendered...)
	out = append(out, raw[block.end:]...)
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func findMarkdownTables(raw []byte, scope string) ([]mdTableBlock, error) {
	scopeStart, scopeEnd, err := markdownScopeRange(raw, scope)
	if err != nil {
		return nil, err
	}
	var blocks []mdTableBlock
	doc := parseMarkdownDocument(raw)
	_ = goldast.Walk(doc, func(n goldast.Node, entering bool) (goldast.WalkStatus, error) {
		if !entering {
			return goldast.WalkContinue, nil
		}
		table, ok := n.(*mdast.Table)
		if !ok {
			return goldast.WalkContinue, nil
		}
		start, end := markdownTableBlockRange(raw, table)
		if start < scopeStart || end > scopeEnd {
			return goldast.WalkSkipChildren, nil
		}
		blocks = append(blocks, mdTableBlock{
			start: start,
			end:   end,
			table: markdownTableData(raw, table),
		})
		return goldast.WalkSkipChildren, nil
	})
	return blocks, nil
}

func markdownScopeRange(raw []byte, scope string) (int, int, error) {
	if scope == "" || scope == "doc" {
		return 0, len(raw), nil
	}
	sections, err := markdownSections(raw, scope)
	if err != nil {
		return 0, 0, err
	}
	if len(sections) == 0 {
		return 0, 0, failf("markdown scope %q not found", scope)
	}
	if len(sections) > 1 {
		return 0, 0, failf("markdown scope %q is ambiguous", scope)
	}
	section := sections[0]
	return section.Heading.BodyStart, section.BodyEnd, nil
}

func markdownTableBlockRange(raw []byte, table *mdast.Table) (int, int) {
	startPos := table.Pos()
	if header := table.FirstChild(); header != nil && header.Pos() >= 0 {
		startPos = header.Pos()
	}
	if startPos < 0 {
		startPos = 0
	}
	start := markdownLineStart(raw, startPos)
	headerEnd := markdownLineEnd(raw, start)
	end := markdownLineEnd(raw, headerEnd)
	for row := table.FirstChild(); row != nil; row = row.NextSibling() {
		if row.Pos() >= 0 {
			end = max(end, markdownLineEnd(raw, row.Pos()))
		}
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			lines := cell.Lines()
			for i := range lines.Len() {
				end = max(end, markdownLineEnd(raw, lines.At(i).Stop))
			}
		}
	}
	return start, end
}

func markdownTableData(raw []byte, table *mdast.Table) tableData {
	header := table.FirstChild()
	if header == nil {
		return tableData{}
	}
	td := tableData{Header: markdownTableCells(raw, header)}
	for row := header.NextSibling(); row != nil; row = row.NextSibling() {
		td.Rows = append(td.Rows, normalizeRow(markdownTableCells(raw, row), len(td.Header)))
	}
	return td
}

func markdownTableCells(raw []byte, row goldast.Node) []string {
	var cells []string
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		text := strings.TrimSpace(string(cell.Text(raw)))
		cells = append(cells, strings.ReplaceAll(text, `\|`, `|`))
	}
	return cells
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
