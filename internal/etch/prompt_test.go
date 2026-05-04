package etch

import (
	"bytes"
	"strings"
	"testing"
)

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
