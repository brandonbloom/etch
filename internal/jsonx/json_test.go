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
	want := Object{
		{Name: "n", Value: Number("9007199254740993")},
		{Name: "items", Value: []any{Number("1.0"), Number("1e2")}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DecodeValue = %#v, want %#v", got, want)
	}
}

func TestDecodeValuePreservesDuplicateObjectNames(t *testing.T) {
	got, err := DecodeValue([]byte(`{"a":1,"a":2}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Object{{Name: "a", Value: Number("1")}, {Name: "a", Value: Number("2")}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DecodeValue = %#v, want %#v", got, want)
	}
}

func TestObjectMarshalsInMemberOrder(t *testing.T) {
	got, err := Marshal(Object{
		{Name: "z", Value: Number("1")},
		{Name: "a", Value: Object{{Name: "b", Value: true}, {Name: "a", Value: false}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"z":1,"a":{"b":true,"a":false}}`
	if string(got) != want {
		t.Fatalf("Marshal = %s, want %s", got, want)
	}
}

func TestObjectMarshalIndentUsesMemberOrder(t *testing.T) {
	got, err := MarshalIndent(Object{
		{Name: "z", Value: Number("1")},
		{Name: "a", Value: Object{{Name: "b", Value: true}, {Name: "a", Value: false}}},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"z\": 1,\n  \"a\": {\n    \"b\": true,\n    \"a\": false\n  }\n}"
	if string(got) != want {
		t.Fatalf("MarshalIndent = %s, want %s", got, want)
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
