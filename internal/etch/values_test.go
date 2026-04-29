package etch

import "testing"

func TestParseValueRequiresWholeJSONLiteral(t *testing.T) {
	if got := parseValue("0 abc"); got != "0 abc" {
		t.Fatalf("parseValue accepted JSON prefix: %#v", got)
	}
	hexdump := "00000000  aa bb cc dd  |....|\n00000010\n"
	if got := parseValue(hexdump); got != hexdump {
		t.Fatalf("parseValue accepted hexdump prefix: %#v", got)
	}
	if got := parseValue("1"); got != float64(1) {
		t.Fatalf("parseValue valid JSON number = %#v", got)
	}
}

func TestDecodeJSONRejectsTrailingJunk(t *testing.T) {
	if _, _, err := decodeJSON([]byte(`{"ok": true} trailing`)); err == nil {
		t.Fatal("decodeJSON accepted trailing junk")
	}
}
