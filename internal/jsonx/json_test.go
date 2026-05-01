package jsonx

import (
	"reflect"
	"testing"
)

func TestDecodeValuePreservesNumbers(t *testing.T) {
	got, err := DecodeValue([]byte(`{"n":9007199254740993,"items":[1.0,1e2]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"items": []any{Number("1.0"), Number("1e2")},
		"n":     Number("9007199254740993"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DecodeValue = %#v, want %#v", got, want)
	}
}

func TestDecodeValueAllowsDuplicateObjectNames(t *testing.T) {
	got, err := DecodeValue([]byte(`{"a":1,"a":2}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"a": Number("2")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DecodeValue = %#v, want %#v", got, want)
	}
}

func TestNumberMarshalsAsJSONNumber(t *testing.T) {
	got, err := Marshal(map[string]any{"n": Number("9007199254740993")})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"n":9007199254740993}`
	if string(got) != want {
		t.Fatalf("Marshal = %s, want %s", got, want)
	}
}

func TestNumberMarshalRejectsInvalidNumber(t *testing.T) {
	if _, err := Marshal(Number("01")); err == nil {
		t.Fatal("Marshal accepted invalid JSON number")
	}
}
