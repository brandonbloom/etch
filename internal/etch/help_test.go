package etch

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpTopicsSnapshotSmoke(t *testing.T) {
	for _, topic := range []string{"", "model", "scripts", "selectors", "values", "fields", "plans", "security", "conflicts", "section", "table", "csv"} {
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
	for _, hidden := range []string{"json set", "yaml set", "frontmatter set", "md section replace", "csv set"} {
		if strings.Contains(text, hidden) {
			t.Fatalf("default help contains plumbing command %q:\n%s", hidden, text)
		}
	}
	for _, shown := range []string{"set <path>", "table set", "section replace", "section append", "section prepend"} {
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
	for _, shown := range []string{"json set", "yaml set", "frontmatter set", "md section replace", "csv set"} {
		if !strings.Contains(text, shown) {
			t.Fatalf("help --all missing plumbing command %q:\n%s", shown, text)
		}
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
	if out.String() == shortHelp || !strings.Contains(out.String(), "Porcelain commands:") {
		t.Fatalf("help did not produce long help:\n%s", out.String())
	}
}

func TestShortHelpMentionsCoreFlags(t *testing.T) {
	for _, want := range []string{"--plan", "-n, --dry-run", "--no-checkout", "--untracked", "--allow-empty"} {
		if !strings.Contains(shortHelp, want) {
			t.Fatalf("short help missing %s", want)
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
