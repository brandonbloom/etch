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
set posts/hello.md title "Hello, world"
section replace posts/hello.md "## Summary" <<EOF
$FOO is literal
EOF
`))
	if err != nil {
		t.Fatalf("ParseScriptBytes: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2", len(stmts))
	}
	if got := strings.Join(stmts[0].Tokens, "|"); got != "set|posts/hello.md|title|Hello, world" {
		t.Fatalf("statement 0 tokens = %q", got)
	}
	if got := stmts[1].Tokens[4]; got != "$FOO is literal\n" {
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
	direct, err := DecodeOperation(Statement{Tokens: []string{"set", "task.md", "status", "complete"}})
	if err != nil {
		t.Fatalf("DecodeOperation direct: %v", err)
	}
	stmts, err := ParseScriptBytes("ops.etch", []byte(`set task.md status complete`))
	if err != nil {
		t.Fatalf("ParseScriptBytes: %v", err)
	}
	fromScript, err := DecodeOperation(stmts[0])
	if err != nil {
		t.Fatalf("DecodeOperation script: %v", err)
	}
	direct.Loc = SourceLoc{}
	fromScript.Loc = SourceLoc{}
	if direct.Verb != fromScript.Verb || direct.Kind != fromScript.Kind || direct.Target != fromScript.Target || direct.Value != fromScript.Value || direct.ValueMode != fromScript.ValueMode {
		t.Fatalf("direct %#v != script %#v", direct, fromScript)
	}
	if direct.Target.Part != "frontmatter" || direct.Target.Selector != "$.status" {
		t.Fatalf("target = %#v", direct.Target)
	}
}

func TestDecodeAssignmentSetExpandsOperations(t *testing.T) {
	ops, err := DecodeOperations(Statement{Tokens: []string{"json", "set", "state.json", "a=1", "b:=2", `$["a=b"]=value`}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 3 {
		t.Fatalf("ops = %#v", ops)
	}
	wants := []struct {
		selector string
		value    string
		mode     ValueMode
	}{
		{"$.a", "1", ValueModeString},
		{"$.b", "2", ValueModeJSON},
		{`$["a=b"]`, "value", ValueModeString},
	}
	for i, want := range wants {
		if ops[i].Target.Selector != want.selector || ops[i].Value != want.value || ops[i].ValueMode != want.mode {
			t.Fatalf("op %d = %#v, want selector=%s value=%s mode=%s", i, ops[i], want.selector, want.value, want.mode)
		}
	}
}

func TestDecodeAssignmentSetRejectsDuplicateTargets(t *testing.T) {
	if _, err := DecodeOperations(Statement{Tokens: []string{"set", "state.json", "a=1", "$.a=2"}}); err == nil {
		t.Fatal("duplicate assignment targets succeeded")
	}
}

func TestDecodeAssignmentItemsAreSetOnlyWithoutBlockingLiteralValues(t *testing.T) {
	op, err := DecodeOperation(Statement{Tokens: []string{"json", "append", "state.json", "items", "a=b"}})
	if err != nil {
		t.Fatal(err)
	}
	if op.Value != "a=b" || op.ValueMode != ValueModeString {
		t.Fatalf("append op = %#v", op)
	}
	if _, err := DecodeOperations(Statement{Tokens: []string{"json", "append", "state.json", "items:=1"}}); err == nil {
		t.Fatal("append accepted assignment item")
	}
}

func TestDecodeMarkdownFieldAddressValidation(t *testing.T) {
	if _, err := DecodeOperation(Statement{Tokens: []string{"set", "note.md", "file.name", "Bad"}}); err == nil || !strings.Contains(err.Error(), "implicit field") {
		t.Fatalf("file.* err = %v", err)
	}
	if _, err := DecodeOperation(Statement{Tokens: []string{"set", "note.md", "done", "yes", "--item-type", "task"}}); err == nil || !strings.Contains(err.Error(), "--item-type requires") {
		t.Fatalf("item-type err = %v", err)
	}
	if _, err := DecodeOperation(Statement{Tokens: []string{"set", "note.md", "status", "done", "--task", "Send"}}); err == nil || !strings.Contains(err.Error(), "task/list implicit field") {
		t.Fatalf("task implicit err = %v", err)
	}
}

func TestDecodeMarkdownTaskListCommands(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		verb    string
		class   CommandClass
		value   string
		section string
		before  string
	}{
		{
			name:    "task close",
			tokens:  []string{"task", "close", "note.md", "Send follow-up", "--section", "Actions"},
			verb:    "task close",
			class:   ClassIdempotent,
			value:   "Send follow-up",
			section: "Actions",
		},
		{
			name:    "task add flags before text",
			tokens:  []string{"task", "add", "note.md", "--section", "Actions", "--before", "Later", "Send follow-up"},
			verb:    "task add",
			class:   ClassNonIdempotent,
			value:   "Send follow-up",
			section: "Actions",
			before:  "Later",
		},
		{
			name:    "list add task shorthand",
			tokens:  []string{"list", "add", "note.md", "Send follow-up", "--task", "--section", "Actions"},
			verb:    "task add",
			class:   ClassNonIdempotent,
			value:   "Send follow-up",
			section: "Actions",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := DecodeOperation(Statement{Tokens: tc.tokens})
			if err != nil {
				t.Fatal(err)
			}
			if op.Verb != tc.verb || op.Kind != "md-task-list" || op.Class != tc.class || op.Path != "note.md" || op.Value != tc.value || op.Markdown.Section != tc.section || op.Markdown.Before != tc.before {
				t.Fatalf("op = %#v", op)
			}
		})
	}
	if _, err := DecodeOperation(Statement{Tokens: []string{"task", "add", "note.md", "Send", "--task"}}); err == nil || !strings.Contains(err.Error(), "--task is only accepted by list add") {
		t.Fatalf("task add --task err = %v", err)
	}
	if _, err := DecodeOperation(Statement{Tokens: []string{"list", "add", "note.md", "Send", "--before", "A", "--after", "B"}}); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("list add before/after err = %v", err)
	}
}

func TestDecodeJSONLAppend(t *testing.T) {
	tests := []struct {
		name string
		args []string
		verb string
		path string
	}{
		{
			name: "porcelain jsonl",
			args: []string{"append", "events.jsonl", `{"kind":"prompt"}`},
			verb: "append",
			path: "events.jsonl",
		},
		{
			name: "porcelain ndjson",
			args: []string{"append", "events.ndjson", `true`},
			verb: "append",
			path: "events.ndjson",
		},
		{
			name: "plumbing",
			args: []string{"jsonl", "append", "events.log", `12`},
			verb: "jsonl append",
			path: "events.log",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, err := DecodeOperation(Statement{Tokens: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			if op.Verb != tc.verb || op.Kind != "jsonl" || op.Class != ClassNonIdempotent || op.Path != tc.path || op.ValueMode != ValueModeJSON || op.Target.Path != tc.path {
				t.Fatalf("op = %#v", op)
			}
		})
	}
	if _, err := DecodeOperation(Statement{Tokens: []string{"append", "events.jsonl", "items", `{"kind":"prompt"}`}}); err == nil {
		t.Fatal("jsonl append accepted selector form")
	}
}

func TestNormalizeSelector(t *testing.T) {
	tests := map[string]string{
		"a.b[0]":                 "$.a.b[0]",
		`$["key.with.dots"]`:     `$["key.with.dots"]`,
		`$['key.with.dots']`:     `$["key.with.dots"]`,
		`["sp ace"]`:             `$["sp ace"]`,
		`$["fo\u00f8"]`:          "$.foø",
		`$["tune \uD834\uDD1E"]`: `$["tune 𝄞"]`,
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
	for _, bad := range []string{
		"$..a",
		"$.a[*]",
		"$.a[-1]",
		"$.a[0:2]",
		`$["a","b"]`,
		"$[?(@.a)]",
		"$.items[?(@.status == 'open')]",
		"$.items[?count(@.tags) > 0]",
		"$.items[?match(@.name, 'a.*')]",
	} {
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
	byName := map[string]VerbInfo{}
	validClasses := map[CommandClass]bool{
		ClassGuard:         true,
		ClassIdempotent:    true,
		ClassNonIdempotent: true,
	}
	for _, verb := range verbs {
		if verb.Name == "" || verb.Signature == "" || verb.Description == "" {
			t.Fatalf("verb has empty required field: %#v", verb)
		}
		if !validClasses[verb.Class] {
			t.Fatalf("verb %s has invalid class %q", verb.Name, verb.Class)
		}
		if !verb.Canonical {
			t.Fatalf("verb %s is not marked canonical", verb.Name)
		}
		if _, ok := byName[verb.Name]; ok {
			t.Fatalf("duplicate verb %q", verb.Name)
		}
		byName[verb.Name] = verb
	}
	for name, class := range map[string]CommandClass{
		"set":                 ClassIdempotent,
		"append":              ClassNonIdempotent,
		"jsonl append":        ClassNonIdempotent,
		"exists":              ClassGuard,
		"table row append":    ClassNonIdempotent,
		"md table row delete": ClassIdempotent,
	} {
		verb, ok := byName[name]
		if !ok {
			t.Fatalf("verbs JSON missing %q", name)
		}
		if verb.Class != class {
			t.Fatalf("verb %s class = %q, want %q", name, verb.Class, class)
		}
	}
}

func TestRunWithoutScriptPathParsesStdin(t *testing.T) {
	oldReadStdin := readStdin
	readStdin = func() ([]byte, error) {
		return []byte("set a.json x --json 1\n"), nil
	}
	t.Cleanup(func() { readStdin = oldReadStdin })

	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")

	var out, stderr bytes.Buffer
	code, err := runCLIAt(dir, []string{"run"}, &out, &stderr)
	if err != nil || code != exitOK {
		t.Fatalf("run code=%d err=%v stderr=%s", code, err, stderr.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:a.json")
	if !strings.Contains(headBytes, `"x": 1`) {
		t.Fatalf("HEAD a.json = %s", headBytes)
	}
}
