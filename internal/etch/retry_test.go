package etch

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRetryReplansAfterRefCASConflict(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	writeFile(t, dir, "other.txt", "base\n")
	commitAll(t, dir, "initial")

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
	code, err := runCLIAt(dir, []string{"--retries", "1", "set", "state.json", "status", "complete"}, &out, &errb)
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

	beforeUpdateRefHook = func(attempt int) {
		path := fmt.Sprintf("tick-%d.txt", attempt)
		writeFile(t, dir, path, "tick\n")
		testGit(t, dir, "add", path)
		testGit(t, dir, "commit", "-m", "tick")
	}
	t.Cleanup(func() { beforeUpdateRefHook = nil })

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"--retries", "1", "set", "state.json", "status", "complete"}, &out, &errb)
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

func TestRetryBackoffWindows(t *testing.T) {
	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{attempt: 0},
		{attempt: 1},
		{attempt: 2, min: 50 * time.Millisecond, max: 150 * time.Millisecond},
		{attempt: 3, min: 100 * time.Millisecond, max: 300 * time.Millisecond},
		{attempt: 4, min: 200 * time.Millisecond, max: 600 * time.Millisecond},
		{attempt: 5, min: 400 * time.Millisecond, max: 1200 * time.Millisecond},
		{attempt: 6, min: 800 * time.Millisecond, max: 2000 * time.Millisecond},
		{attempt: 9, min: 800 * time.Millisecond, max: 2000 * time.Millisecond},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tc.attempt), func(t *testing.T) {
			got := retryBackoffWindow(tc.attempt)
			if got.Min != tc.min || got.Max != tc.max {
				t.Fatalf("retryBackoffWindow(%d) = (%s, %s), want (%s, %s)", tc.attempt, got.Min, got.Max, tc.min, tc.max)
			}
		})
	}
}

func TestSleepRetryUsesRandomizedWindow(t *testing.T) {
	oldSleep := retrySleep
	oldRandom := retryRandomDuration
	t.Cleanup(func() {
		retrySleep = oldSleep
		retryRandomDuration = oldRandom
	})

	var gotWindow retryWindow
	retryRandomDuration = func(w retryWindow) time.Duration {
		gotWindow = w
		return 123 * time.Millisecond
	}
	var slept time.Duration
	retrySleep = func(d time.Duration) {
		slept = d
	}

	sleepRetry(2)
	if gotWindow != (retryWindow{Min: 50 * time.Millisecond, Max: 150 * time.Millisecond}) {
		t.Fatalf("randomized window = %#v", gotWindow)
	}
	if slept != 123*time.Millisecond {
		t.Fatalf("slept %s, want 123ms", slept)
	}

	gotWindow = retryWindow{}
	slept = 0
	sleepRetry(1)
	if gotWindow != (retryWindow{}) || slept != 0 {
		t.Fatalf("first retry should be immediate, window=%#v slept=%s", gotWindow, slept)
	}
}
