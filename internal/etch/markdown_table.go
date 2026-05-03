package etch

import (
	"bytes"
	"strings"
	"unicode/utf8"

	goldast "github.com/yuin/goldmark/ast"
	mdast "github.com/yuin/goldmark/extension/ast"
)

type mdTableBlock struct {
	start int
	end   int
	table tableData
	cells [][]mdTableCellRange
}

type mdTableCellRange struct {
	Start int
	End   int
	OK    bool
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
	if op.Verb == "table set" {
		rows, cols, err := resolveTableRange(td, op.Target.Range)
		if err != nil {
			return nil, false, err
		}
		for _, r := range rows {
			for _, c := range cols {
				td.Rows[r][c] = op.Value
			}
		}
		if patched, ok := patchMarkdownTableCells(raw, block, rows, cols, op.Value); ok {
			out := withUTF8BOM(patched, bom)
			return out, !bytes.Equal(out, before), nil
		}
	}
	if op.Verb != "table set" {
		if err := mutateTable(&td, op); err != nil {
			return nil, false, err
		}
	}
	rendered := []byte(renderMarkdownTable(td))
	out := make([]byte, 0, len(raw)-(block.end-block.start)+len(rendered))
	out = append(out, raw[:block.start]...)
	out = append(out, rendered...)
	out = append(out, raw[block.end:]...)
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}

func patchMarkdownTableCells(raw []byte, block mdTableBlock, rows, cols []int, value string) ([]byte, bool) {
	repl := []byte(escapeMarkdownTableCell(value))
	var ranges []jsonByteRange
	for _, r := range rows {
		if r < 0 || r >= len(block.cells) {
			return nil, false
		}
		row := block.cells[r]
		for _, c := range cols {
			if c < 0 || c >= len(row) || !row[c].OK {
				return nil, false
			}
			ranges = append(ranges, jsonByteRange{Start: row[c].Start, End: row[c].End})
		}
	}
	out := raw
	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		out = replaceBytes(out, r.Start, r.End, repl)
	}
	return out, true
}

func markdownTableCellRanges(table *mdast.Table) [][]mdTableCellRange {
	header := table.FirstChild()
	if header == nil {
		return nil
	}
	var rows [][]mdTableCellRange
	for row := header.NextSibling(); row != nil; row = row.NextSibling() {
		var cells []mdTableCellRange
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			lines := cell.Lines()
			if lines.Len() == 0 {
				cells = append(cells, mdTableCellRange{})
				continue
			}
			cells = append(cells, mdTableCellRange{
				Start: lines.At(0).Start,
				End:   lines.At(lines.Len() - 1).Stop,
				OK:    true,
			})
		}
		rows = append(rows, cells)
	}
	return rows
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
			cells: markdownTableCellRanges(table),
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
			b.WriteString(escapeMarkdownTableCell(cell))
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

func escapeMarkdownTableCell(cell string) string {
	return strings.ReplaceAll(cell, "|", "\\|")
}
