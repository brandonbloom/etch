package etch

import (
	"bytes"
	"unicode/utf8"
)

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
