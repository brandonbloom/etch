package etch

import (
	"bytes"
	"strings"
	"testing"
)

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
