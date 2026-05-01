package etch

import (
	"reflect"
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"
)

func TestYAMLSemanticEquality(t *testing.T) {
	file, err := parseYAMLFile([]byte("defaults: &defaults\n  a: 1\nalias: *defaults\ntag: !custom value\nobject:\n  b: 2\n  a: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	doc := firstYAMLDocument(file)
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

func TestYAMLLiteralSemanticFallback(t *testing.T) {
	if got := yamlNodeSemantic(&ast.LiteralNode{Start: &token.Token{Origin: "| malformed\n"}}); got != "| malformed\n" {
		t.Fatalf("literal fallback = %#v", got)
	}
	if got := yamlNodeSemantic(&ast.LiteralNode{}); got != nil {
		t.Fatalf("literal without start fallback = %#v, want nil", got)
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
		"n": semanticNumber("1"),
		"items": []any{
			semanticNumber("2"),
			map[string]any{"f": semanticNumber("7/2")},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonicalYAMLSemantic = %#v, want %#v", got, want)
	}
}
