package etch

import (
	"bytes"
	"unicode/utf8"

	"github.com/brandonbloom/etch/internal/jsonx"
)

func evalJSONLAppend(rawValue string, before []byte) ([]byte, bool, error) {
	raw, bom := trimUTF8BOM(before)
	if !utf8.Valid(raw) {
		return nil, false, failf("invalid UTF-8 in JSONL input")
	}
	if err := validateJSONLAppendBoundary(raw); err != nil {
		return nil, false, err
	}
	value, err := jsonx.DecodeValue([]byte(rawValue))
	if err != nil {
		return nil, false, usagef("invalid JSONL value: %v", err)
	}
	line, err := jsonx.Marshal(value)
	if err != nil {
		return nil, false, err
	}
	out := make([]byte, 0, len(raw)+len(line)+1)
	out = append(out, raw...)
	out = append(out, line...)
	out = append(out, '\n')
	out = withUTF8BOM(out, bom)
	return out, true, nil
}

func validateJSONLAppendBoundary(raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	if raw[len(raw)-1] != '\n' {
		return failf("JSONL append target must end with a newline")
	}
	lineEnd := len(raw) - 1
	if lineEnd > 0 && raw[lineEnd-1] == '\r' {
		lineEnd--
	}
	lineStart := bytes.LastIndexByte(raw[:lineEnd], '\n') + 1
	if len(bytes.Trim(raw[lineStart:lineEnd], " \t\r")) == 0 {
		return failf("JSONL append boundary is blank")
	}
	return nil
}

func validateJSONLFile(raw []byte) error {
	if !utf8.Valid(raw) {
		return failf("invalid UTF-8")
	}
	if err := validateJSONLAppendBoundary(raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		return nil
	}
	lines := bytes.Split(raw, []byte{'\n'})
	for i, line := range lines[:len(lines)-1] {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if len(bytes.Trim(line, " \t")) == 0 {
			return failf("record %d is blank", i+1)
		}
		if _, err := jsonx.DecodeValue(line); err != nil {
			return failf("record %d is not valid JSON", i+1)
		}
	}
	return nil
}
