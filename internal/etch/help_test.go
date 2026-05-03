package etch

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHelpTopicsSnapshotSmoke(t *testing.T) {
	for _, topic := range []string{"", "model", "invocation", "prompts", "prompt", "scripts", "selectors", "values", "formats", "fields", "files", "guards", "plans", "commits", "security", "conflicts", "addressing", "markdown", "section", "tasks", "table", "csv"} {
		var out bytes.Buffer
		if err := printHelp(&out, topic, false); err != nil {
			t.Fatalf("help %q: %v", topic, err)
		}
		if out.Len() == 0 {
			t.Fatalf("help %q produced no output", topic)
		}
	}
}

func TestDefaultHelpTableExcludesPlumbing(t *testing.T) {
	var out bytes.Buffer
	if err := printHelp(&out, "", false); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, hidden := range []string{"json set", "jsonl append", "yaml set", "frontmatter set", "md section replace", "csv set"} {
		if strings.Contains(text, hidden) {
			t.Fatalf("default help contains plumbing command %q:\n%s", hidden, text)
		}
	}
	for _, shown := range []string{"prompt [--context|--bootstrap]", "set <path>", "table set", "section replace", "section append", "section prepend", "task close", "list add", "replace <path>"} {
		if !strings.Contains(text, shown) {
			t.Fatalf("default help missing porcelain command %q:\n%s", shown, text)
		}
	}
}

func TestHelpAllIncludesPlumbing(t *testing.T) {
	var out bytes.Buffer
	if err := printHelp(&out, "", true); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, shown := range []string{"json set", "jsonl append", "yaml set", "frontmatter set", "md section replace", "csv set"} {
		if !strings.Contains(text, shown) {
			t.Fatalf("help --all missing plumbing command %q:\n%s", shown, text)
		}
	}
	if !strings.Contains(text, "Format-explicit command prefixes select the parser and writer") {
		t.Fatalf("help --all missing advanced format warning:\n%s", text)
	}
}

func TestHelpAllThroughCLI(t *testing.T) {
	var out, errb bytes.Buffer
	code, err := runCLI([]string{"help", "--all"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI(help --all) code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(out.String(), "json set") {
		t.Fatalf("runCLI(help --all) did not include plumbing commands:\n%s", out.String())
	}
}

func TestHelpJSONThroughCLI(t *testing.T) {
	var out, errb bytes.Buffer
	code, err := runCLI([]string{"help", "--json"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI(help --json) code=%d err=%v stderr=%s", code, err, errb.String())
	}

	var reference HelpReference
	if err := json.Unmarshal(out.Bytes(), &reference); err != nil {
		t.Fatalf("help --json returned invalid JSON: %v\n%s", err, out.String())
	}
	if len(reference.Topics) == 0 {
		t.Fatal("help --json returned no topics")
	}
	if reference.Topics[0].ID != "common-commands" || reference.Topics[0].Blocks[0].Kind != "command-table" {
		t.Fatalf("unexpected first reference topic: %#v", reference.Topics[0])
	}
	for _, topic := range reference.Topics {
		if topic.Group == "" {
			t.Fatalf("reference topic missing group: %#v", topic)
		}
	}
}

func TestHelpFlagIsShortReference(t *testing.T) {
	var out, errb bytes.Buffer
	code, err := runCLI([]string{"--help"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI(--help) code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if out.String() != shortHelp {
		t.Fatalf("--help output mismatch:\n%s", out.String())
	}

	out.Reset()
	errb.Reset()
	code, err = runCLI([]string{"help"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI(help) code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if out.String() == shortHelp || !strings.Contains(out.String(), "Common commands:") {
		t.Fatalf("help did not produce long help:\n%s", out.String())
	}
}

func TestShortHelpMentionsCoreFlags(t *testing.T) {
	for _, want := range []string{"--plan", "-n, --dry-run", "--no-checkout", "--untracked", "--allow-empty"} {
		if !strings.Contains(shortHelp, want) {
			t.Fatalf("short help missing %s", want)
		}
	}
	if !strings.Contains(shortHelp, "etch prompt") {
		t.Fatalf("short help missing prompt guidance:\n%s", shortHelp)
	}
}

func TestPromptThroughCLI(t *testing.T) {
	var out, errb bytes.Buffer
	code, err := runCLI([]string{"prompt"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("prompt code=%d err=%v stderr=%s", code, err, errb.String())
	}
	text := out.String()
	for _, want := range []string{"# etch Bootstrap Prompt", "etch prompt --context", "# etch Agent Context"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt output missing %q:\n%s", want, text)
		}
	}

	out.Reset()
	errb.Reset()
	code, err = runCLI([]string{"prompt", "--context"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("prompt --context code=%d err=%v stderr=%s", code, err, errb.String())
	}
	text = out.String()
	for _, want := range []string{"# etch Agent Context", "etch help --all", "conflicts"} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt --context output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "# etch Bootstrap Prompt") {
		t.Fatalf("prompt --context included bootstrap wrapper:\n%s", text)
	}
	if len(text) > 2400 {
		t.Fatalf("prompt --context should stay terse, got %d bytes:\n%s", len(text), text)
	}

	out.Reset()
	errb.Reset()
	code, err = runCLI([]string{"prompt", "--context", "--bootstrap"}, &out, &errb)
	if err == nil || code != exitUsage {
		t.Fatalf("prompt conflicting flags code=%d err=%v stdout=%s stderr=%s", code, err, out.String(), errb.String())
	}
}

func TestScriptsHelpIncludesQuotingExamples(t *testing.T) {
	var out bytes.Buffer
	if err := printHelp(&out, "scripts", false); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		`set posts/hello.md title "Hello, world"`,
		`append events.jsonl '{"kind":"prompt","name":"first"}'`,
		`set state.json payload --json '{"name":"first"}'`,
		`$FOO is not expanded.`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("scripts help missing %q:\n%s", want, text)
		}
	}
}

func TestShellCompletionThroughCLI(t *testing.T) {
	var out, errb bytes.Buffer
	code, err := runCLI([]string{"--generate-shell-completion"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("command completion code=%d err=%v stderr=%s", code, err, errb.String())
	}
	for _, want := range []string{"set\n", "help\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("command completion missing %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	errb.Reset()
	code, err = runCLI([]string{"-", "--generate-shell-completion"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("flag completion code=%d err=%v stderr=%s", code, err, errb.String())
	}
	for _, want := range []string{"--plan\n", "-n\n", "--subject-prefix\n", "--body-suffix\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("flag completion missing %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	errb.Reset()
	code, err = runCLI([]string{"prompt", "-", "--generate-shell-completion"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("prompt flag completion code=%d err=%v stderr=%s", code, err, errb.String())
	}
	for _, want := range []string{"--context\n", "--bootstrap\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("prompt flag completion missing %q:\n%s", want, out.String())
		}
	}
}

func TestCommandPathCompletionsUseCatalog(t *testing.T) {
	got := strings.Join(commandCompletions([]string{"md", "table", "row", ""}), "\n")
	for _, want := range []string{"append", "insert", "delete"} {
		if !strings.Contains(got, want) {
			t.Fatalf("nested command completions missing %q: %q", want, got)
		}
	}

	got = strings.Join(commandLocalFlagCompletions([]string{"set", "state.json", "count"}), "\n")
	if !strings.Contains(got, "--json") {
		t.Fatalf("local flag completions missing --json: %q", got)
	}
}
