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

	_, errb, code, err := runCLIInDir(t, dir, "set", "state.json", "status", "complete")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if !strings.Contains(headBytes, `"status":"complete"`) {
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

func TestE2EExplicitFalseBoolFlagsDoNotEnablePlanModes(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")

	out, errb, code, err := runCLIInDir(t, dir, "--plan=false", "--dry-run=false", "set", "state.json", "status", "complete")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected stdout for committing invocation:\n%s", out.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if !strings.Contains(headBytes, `"status":"complete"`) {
		t.Fatalf("HEAD state.json = %s", headBytes)
	}
}

func TestE2EJSONSetCreatesMissingFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "set", "a.json", "x", "1")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:a.json")
	if !strings.Contains(headBytes, `"x": 1`) {
		t.Fatalf("HEAD a.json = %s", headBytes)
	}
	wt, readErr := os.ReadFile(filepath.Join(dir, "a.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(wt) != headBytes {
		t.Fatalf("worktree not materialized:\nwt=%s\nhead=%s", wt, headBytes)
	}
}

func TestE2EJSONAppendAndAddCreateMissingFile(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"append", "a.json", "items", "1"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("append code=%d err=%v stderr=%s", code, err, errb.String())
	}
	code, err = runCLI([]string{"add", "b.json", "items", "1"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("add code=%d err=%v stderr=%s", code, err, errb.String())
	}
	for _, path := range []string{"a.json", "b.json"} {
		headBytes := testGit(t, dir, "show", "HEAD:"+path)
		if !strings.Contains(headBytes, `"items": [`) || !strings.Contains(headBytes, "1") {
			t.Fatalf("HEAD %s = %s", path, headBytes)
		}
	}
}

func TestE2ECreateDefaultContent(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"create", "a.json"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:a.json")
	if headBytes != "{}" {
		t.Fatalf("HEAD a.json = %q", headBytes)
	}
}

func TestE2ECreateExtensionDefaults(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	cases := map[string]string{
		"config.yaml": "{}\n",
		"note.md":     "",
		"data.csv":    "",
		"plain.txt":   "{}",
	}
	for path, want := range cases {
		var out, errb bytes.Buffer
		code, err := runCLI([]string{"create", path}, &out, &errb)
		if err != nil || code != exitOK {
			t.Fatalf("create %s code=%d err=%v stderr=%s", path, code, err, errb.String())
		}
		headBytes := testGit(t, dir, "show", "HEAD:"+path)
		if headBytes != want {
			t.Fatalf("HEAD %s = %q, want %q", path, headBytes, want)
		}
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
	if strings.Contains(headBytes, "dirty") || strings.Contains(headBytes, `"local":true`) {
		t.Fatalf("dirty worktree swept into commit: %s", headBytes)
	}
	if !strings.Contains(headBytes, `"status":"complete"`) {
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

	writeFile(t, dir, "ops.etch", "contains state.json open\nset state.json status complete\n")
	_, errb, code, err := runCLIInDir(t, dir, "run", "ops.etch")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got == head {
		t.Fatal("successful run did not commit")
	}

	head = stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
	writeFile(t, dir, "bad.etch", "set state.json other ok\nset state.json missing[9].x nope\n")
	_, errb, code, err = runCLIInDir(t, dir, "run", "bad.etch")
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

func TestE2ERunDefaultsToStdin(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	oldReadStdin := readStdin
	readStdin = func() ([]byte, error) {
		return []byte("set stdin.json x 1\nset stdin.json y 2\n"), nil
	}
	t.Cleanup(func() { readStdin = oldReadStdin })

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"run"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:stdin.json")
	if !strings.Contains(headBytes, `"x": 1`) || !strings.Contains(headBytes, `"y": 2`) {
		t.Fatalf("HEAD stdin.json = %s", headBytes)
	}
}
