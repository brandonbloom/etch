package etch

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpTopicsSnapshotSmoke(t *testing.T) {
	for _, topic := range []string{"", "model", "selectors", "values", "plans", "security", "conflicts", "table", "csv"} {
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
	for _, hidden := range []string{"json set", "yaml set", "frontmatter set", "md replace-section", "csv set"} {
		if strings.Contains(text, hidden) {
			t.Fatalf("default help contains plumbing command %q:\n%s", hidden, text)
		}
	}
	for _, shown := range []string{"set <path>", "table set", "replace-section"} {
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
	for _, shown := range []string{"json set", "yaml set", "frontmatter set", "md replace-section", "csv set"} {
		if !strings.Contains(text, shown) {
			t.Fatalf("help --all missing plumbing command %q:\n%s", shown, text)
		}
	}
}

func TestHelpAllThroughCLI(t *testing.T) {
	for _, args := range [][]string{{"help", "--all"}, {"--help", "--all"}} {
		var out, errb bytes.Buffer
		code, err := runCLI(args, &out, &errb)
		if err != nil || code != exitOK {
			t.Fatalf("runCLI(%v) code=%d err=%v stderr=%s", args, code, err, errb.String())
		}
		if !strings.Contains(out.String(), "json set") {
			t.Fatalf("runCLI(%v) did not include plumbing commands:\n%s", args, out.String())
		}
	}
}

func TestShortHelpMentionsCoreFlags(t *testing.T) {
	for _, want := range []string{"--plan", "-n, --dry-run", "--no-checkout", "--untracked", "--allow-empty"} {
		if !strings.Contains(shortHelp, want) {
			t.Fatalf("short help missing %s", want)
		}
	}
}
