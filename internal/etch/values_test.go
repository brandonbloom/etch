package etch

import (
	"testing"

	"github.com/brandonbloom/etch/internal/jsonx"
)

func TestParseValueRequiresWholeJSONLiteral(t *testing.T) {
	if got := parseValue("0 abc"); got != "0 abc" {
		t.Fatalf("parseValue accepted JSON prefix: %#v", got)
	}
	hexdump := "00000000  aa bb cc dd  |....|\n00000010\n"
	if got := parseValue(hexdump); got != hexdump {
		t.Fatalf("parseValue accepted hexdump prefix: %#v", got)
	}
	if got := parseValue("1"); got != jsonx.Number("1") {
		t.Fatalf("parseValue valid JSON number = %#v", got)
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
