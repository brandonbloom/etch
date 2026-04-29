package etch

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestRetryReplansAfterRefCASConflict(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	writeFile(t, dir, "other.txt", "base\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	hooked := false
	beforeUpdateRefHook = func(attempt int) {
		if hooked {
			return
		}
		hooked = true
		writeFile(t, dir, "other.txt", "concurrent\n")
		testGit(t, dir, "add", "other.txt")
		testGit(t, dir, "commit", "-m", "concurrent")
	}
	t.Cleanup(func() { beforeUpdateRefHook = nil })

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"--retries", "1", "set", "state.json", "status", "complete"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	log := testGit(t, dir, "log", "--format=%s", "-2")
	if !strings.Contains(log, "etch set state.json $.status") || !strings.Contains(log, "concurrent") {
		t.Fatalf("history did not contain retry result:\n%s", log)
	}
	if got := testGit(t, dir, "show", "HEAD:other.txt"); got != "concurrent\n" {
		t.Fatalf("concurrent commit lost: %q", got)
	}
}

func TestRetryBudgetExhaustion(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	beforeUpdateRefHook = func(attempt int) {
		path := fmt.Sprintf("tick-%d.txt", attempt)
		writeFile(t, dir, path, "tick\n")
		testGit(t, dir, "add", path)
		testGit(t, dir, "commit", "-m", "tick")
	}
	t.Cleanup(func() { beforeUpdateRefHook = nil })

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"--retries", "1", "set", "state.json", "status", "complete"}, &out, &errb)
	if err == nil || code != exitFailure {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(err.Error(), "retry budget exhausted") {
		t.Fatalf("err = %v", err)
	}
	got := testGit(t, dir, "show", "HEAD:state.json")
	if strings.Contains(got, "complete") {
		t.Fatalf("etch commit landed despite exhausted retries: %s", got)
	}
}
