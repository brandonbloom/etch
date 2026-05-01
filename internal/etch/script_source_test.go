package etch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseScriptAtUsesInjectedScriptSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ops.etch")
	source := &fakeScriptSource{
		files: map[string][]byte{
			path: []byte("set state.json status complete\n"),
		},
	}

	stmts, err := parseScriptAt(dir, "ops.etch", source)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := source.reads, []string{path}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("script reads = %v, want %v", got, want)
	}
	if len(stmts) != 1 || stmts[0].Loc.Name != "ops.etch" || stmts[0].Tokens[0] != "set" {
		t.Fatalf("statements = %#v", stmts)
	}
}

func TestParseScriptAtUsesInjectedStdin(t *testing.T) {
	source := &fakeScriptSource{stdin: []byte("set stdin.json x 1\n")}

	stmts, err := parseScriptAt("ignored", "-", source)
	if err != nil {
		t.Fatal(err)
	}
	if !source.stdinRead {
		t.Fatal("stdin was not read")
	}
	if len(stmts) != 1 || stmts[0].Loc.Name != "<stdin>" || stmts[0].Tokens[1] != "stdin.json" {
		t.Fatalf("statements = %#v", stmts)
	}
}

type fakeScriptSource struct {
	files     map[string][]byte
	stdin     []byte
	reads     []string
	stdinRead bool
}

func (s *fakeScriptSource) readFile(path string) ([]byte, error) {
	s.reads = append(s.reads, path)
	b, ok := s.files[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", os.ErrNotExist, path)
	}
	return append([]byte(nil), b...), nil
}

func (s *fakeScriptSource) readStdin() ([]byte, error) {
	s.stdinRead = true
	return append([]byte(nil), s.stdin...), nil
}
