package etch

import (
	"bytes"
	"encoding/csv"
	"unicode/utf8"
)

func evalCSVTable(op Operation, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in CSV input")
	}
	r := csv.NewReader(bytes.NewReader(raw))
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
	out = withUTF8BOM(out, bom)
	return out, !bytes.Equal(out, before), nil
}
