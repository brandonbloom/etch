package etch

import (
	"testing"

	"github.com/brandonbloom/etch/internal/jsonx"
)

func TestParseStructuredValueModes(t *testing.T) {
	if got, err := parseStructuredValue("1", ValueModeString); err != nil || got != "1" {
		t.Fatalf("string value = %#v err=%v", got, err)
	}
	if got, err := parseStructuredValue("1", ValueModeJSON); err != nil || got != jsonx.Number("1") {
		t.Fatalf("JSON number value = %#v err=%v", got, err)
	}
	if _, err := parseStructuredValue("0 abc", ValueModeJSON); err == nil {
		t.Fatal("JSON value accepted trailing junk")
	}
	hexdump := "00000000  aa bb cc dd  |....|\n00000010\n"
	if got, err := parseStructuredValue(hexdump, ValueModeString); err != nil || got != hexdump {
		t.Fatalf("string hexdump value = %#v err=%v", got, err)
	}
}

func TestDecodeJSONSpansRejectsTrailingJunk(t *testing.T) {
	if _, err := decodeJSONSpans([]byte(`{"ok": true} trailing`)); err == nil {
		t.Fatal("decodeJSONSpans accepted trailing junk")
	}
}

func TestSemanticEqualComparesNumbersExactly(t *testing.T) {
	if semanticEqual(jsonx.Number("9007199254740993"), jsonx.Number("9007199254740992")) {
		t.Fatal("semanticEqual treated distinct large integers as equal")
	}
	if !semanticEqual(jsonx.Number("1.0"), jsonx.Number("1e0")) {
		t.Fatal("semanticEqual treated equivalent JSON numbers as unequal")
	}
	if !semanticEqual(jsonx.Number("3.5"), float32(3.5)) {
		t.Fatal("semanticEqual treated equivalent YAML and JSON numbers as unequal")
	}
}
