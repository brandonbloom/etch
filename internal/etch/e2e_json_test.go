package etch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EJSONSetCommitsAndMaterializes(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"set", "state.json", "status", "complete"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if !strings.Contains(headBytes, `"status": "complete"`) {
		t.Fatalf("HEAD state.json = %s", headBytes)
	}
	wt, readErr := os.ReadFile(filepath.Join(dir, "state.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(wt) != headBytes {
		t.Fatalf("worktree not materialized:\nwt=%s\nhead=%s", wt, headBytes)
	}
	subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s"))
	if subject != `etch set state.json $.status "complete"` {
		t.Fatalf("subject = %q", subject)
	}
}

func TestE2EJSONSetUsesHEADNotDirtyWorktreeForCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open","local":false}`+"\n")
	commitAll(t, dir, "initial")
	writeFile(t, dir, "state.json", `{"status":"dirty","local":true}`+"\n")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"--no-checkout", "set", "state.json", "status", "complete"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if strings.Contains(headBytes, "dirty") || strings.Contains(headBytes, `"local": true`) {
		t.Fatalf("dirty worktree swept into commit: %s", headBytes)
	}
	if !strings.Contains(headBytes, `"status": "complete"`) {
		t.Fatalf("missing structural mutation: %s", headBytes)
	}
	wt, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if !strings.Contains(string(wt), "dirty") {
		t.Fatalf("--no-checkout did not leave worktree alone: %s", wt)
	}
	if !strings.Contains(errb.String(), "checkout skipped by --no-checkout") {
		t.Fatalf("stderr = %s", errb.String())
	}
}

func TestE2ERunGuardAndAtomicFailure(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	head := commitAll(t, dir, "initial")
	chdir(t, dir)

	script := filepath.Join(dir, "ops.etch")
	if err := os.WriteFile(script, []byte("contains state.json open\nset state.json status complete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	code, err := runCLI([]string{"run", script}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got == head {
		t.Fatal("successful run did not commit")
	}

	head = stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
	bad := filepath.Join(dir, "bad.etch")
	if err := os.WriteFile(bad, []byte("set state.json other ok\nset state.json missing[9].x nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	code, err = runCLI([]string{"run", bad}, &out, &errb)
	if err == nil || code == exitOK {
		t.Fatalf("bad run succeeded stderr=%s", errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed multi-op moved HEAD to %s", got)
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if strings.Contains(headBytes, "other") {
		t.Fatalf("failed multi-op leaked earlier mutation: %s", headBytes)
	}
}
