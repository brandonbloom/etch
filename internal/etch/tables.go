package etch

import (
	"strconv"
	"strings"

	"github.com/brandonbloom/etch/internal/jsonx"
)

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
			return nil
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
