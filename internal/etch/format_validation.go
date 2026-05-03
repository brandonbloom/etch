package etch

import (
	"errors"
	"unicode/utf8"
)

func validateInferredOutput(path, format, verb string, out []byte) error {
	raw, _ := trimUTF8BOM(out)
	switch format {
	case "json":
		if !utf8.Valid(raw) {
			return failf("%s would not be valid JSON after %s: invalid UTF-8", path, verb)
		}
		if _, err := decodeJSONSpans(raw); err != nil {
			var parseErr *jsonInputParseError
			if errors.As(err, &parseErr) {
				return failf("%s would not be valid JSON after %s (parse error near offset %d)", path, verb, parseErr.offset)
			}
			return err
		}
	case "yaml":
		if !utf8.Valid(raw) {
			return failf("%s would not be valid YAML after %s: invalid UTF-8", path, verb)
		}
		if _, err := parseYAMLFile(raw); err != nil {
			return failf("%s would not be valid YAML after %s: %v", path, verb, err)
		}
	case "jsonl":
		if err := validateJSONLFile(raw); err != nil {
			return failf("%s would not be valid JSONL after %s: %v", path, verb, err)
		}
	}
	return nil
}
