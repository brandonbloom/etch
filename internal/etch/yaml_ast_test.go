package etch

import (
	"reflect"
	"testing"
)

func TestYAMLSemanticEquality(t *testing.T) {
	file, err := parseYAMLFile([]byte("defaults: &defaults\n  a: 1\nalias: *defaults\ntag: !custom value\nobject:\n  b: 2\n  a: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := firstYAMLDocument(file)
	if err != nil {
		t.Fatal(err)
	}
	root, err := yamlMappingNode(doc.Body)
	if err != nil {
		t.Fatal(err)
	}

	object, _, err := findYAMLMapValue(root, "object")
	if err != nil {
		t.Fatal(err)
	}
	if !yamlSemanticEqualNodeValue(object.Value, map[string]any{"a": 1, "b": uint64(2)}) {
		t.Fatalf("object semantic equality failed for %s", object.Value.String())
	}

	alias, _, err := findYAMLMapValue(root, "alias")
	if err != nil {
		t.Fatal(err)
	}
	if !yamlSemanticEqualNodeValue(alias.Value, yamlAliasSemantic{Name: "defaults"}) {
		t.Fatalf("alias semantic equality failed for %s", alias.Value.String())
	}
	if yamlSemanticEqualNodeValue(alias.Value, map[string]any{"a": 1}) {
		t.Fatal("alias compared equal to its resolved value")
	}

	tag, _, err := findYAMLMapValue(root, "tag")
	if err != nil {
		t.Fatal(err)
	}
	if !yamlSemanticEqualNodeValue(tag.Value, yamlTagSemantic{Tag: "!custom", Value: "value"}) {
		t.Fatalf("tag semantic equality failed for %s", tag.Value.String())
	}
}

func TestCanonicalYAMLSemanticNormalizesNestedValues(t *testing.T) {
	got := canonicalYAMLSemantic(map[any]any{
		"n": uint8(1),
		"items": []any{
			int16(2),
			map[string]any{"f": float32(3.5)},
		},
	})
	want := map[string]any{
		"n": float64(1),
		"items": []any{
			float64(2),
			map[string]any{"f": float64(3.5)},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonicalYAMLSemantic = %#v, want %#v", got, want)
	}
}
