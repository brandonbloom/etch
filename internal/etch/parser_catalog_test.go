package etch

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestScriptParserQuotingAndHeredoc(t *testing.T) {
	stmts, err := ParseScriptBytes("ops.etch", []byte(`
# comment
set posts/hello.md frontmatter.title "Hello, world"
replace-section posts/hello.md "## Summary" <<EOF
$FOO is literal
EOF
`))
	if err != nil {
		t.Fatalf("ParseScriptBytes: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2", len(stmts))
	}
	if got := strings.Join(stmts[0].Tokens, "|"); got != "set|posts/hello.md|frontmatter.title|Hello, world" {
		t.Fatalf("statement 0 tokens = %q", got)
	}
	if got := stmts[1].Tokens[3]; got != "$FOO is literal\n" {
		t.Fatalf("heredoc = %q", got)
	}
	if stmts[1].Loc.Name != "ops.etch" || stmts[1].Loc.Line != 4 {
		t.Fatalf("loc = %#v", stmts[1].Loc)
	}
}

func TestScriptParserMissingHeredocLocation(t *testing.T) {
	_, err := ParseScriptBytes("ops.etch", []byte("contains a.txt <<END\nmissing\n"))
	if err == nil {
		t.Fatal("expected missing heredoc error")
	}
	if !strings.Contains(err.Error(), "ops.etch:1: missing heredoc terminator") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeDirectAndScriptEquivalence(t *testing.T) {
	direct, err := DecodeOperation(Statement{Tokens: []string{"set", "task.md", "frontmatter.status", "complete"}})
	if err != nil {
		t.Fatalf("DecodeOperation direct: %v", err)
	}
	stmts, err := ParseScriptBytes("ops.etch", []byte(`set task.md frontmatter.status complete`))
	if err != nil {
		t.Fatalf("ParseScriptBytes: %v", err)
	}
	fromScript, err := DecodeOperation(stmts[0])
	if err != nil {
		t.Fatalf("DecodeOperation script: %v", err)
	}
	direct.Loc = SourceLoc{}
	fromScript.Loc = SourceLoc{}
	if direct.Verb != fromScript.Verb || direct.Kind != fromScript.Kind || direct.Target != fromScript.Target || direct.Value != fromScript.Value {
		t.Fatalf("direct %#v != script %#v", direct, fromScript)
	}
	if direct.Target.Part != "frontmatter" || direct.Target.Selector != "$.status" {
		t.Fatalf("target = %#v", direct.Target)
	}
}

func TestNormalizeSelector(t *testing.T) {
	tests := map[string]string{
		"a.b[0]":             "$.a.b[0]",
		`$["key.with.dots"]`: `$["key.with.dots"]`,
		`["sp ace"]`:         `$["sp ace"]`,
	}
	for input, want := range tests {
		got, err := NormalizeSelector(input)
		if err != nil {
			t.Fatalf("NormalizeSelector(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("NormalizeSelector(%q) = %q, want %q", input, got, want)
		}
	}
	for _, bad := range []string{"$..a", "$.a[*]", "$.a[-1]", "$.a[0:2]"} {
		if _, err := NormalizeSelector(bad); err == nil {
			t.Fatalf("NormalizeSelector(%q) succeeded", bad)
		}
	}
}

func TestIntrospectionDoesNotRequireGit(t *testing.T) {
	var out, stderr bytes.Buffer
	code, err := runCLI([]string{"version"}, &out, &stderr)
	if err != nil || code != exitOK {
		t.Fatalf("version code=%d err=%v stderr=%s", code, err, stderr.String())
	}
	if !strings.Contains(out.String(), "etch 0.1.0") {
		t.Fatalf("version output = %q", out.String())
	}

	out.Reset()
	stderr.Reset()
	code, err = runCLI([]string{"verbs", "--json"}, &out, &stderr)
	if err != nil || code != exitOK {
		t.Fatalf("verbs code=%d err=%v stderr=%s", code, err, stderr.String())
	}
	var verbs []VerbInfo
	if err := json.Unmarshal(out.Bytes(), &verbs); err != nil {
		t.Fatalf("verbs json: %v\n%s", err, out.String())
	}
	if len(verbs) < 20 {
		t.Fatalf("got %d verbs, want catalog", len(verbs))
	}
}
