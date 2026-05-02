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

func TestE2EMessageOverride(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	head := commitAll(t, dir, "initial")
	message := "custom subject\n\ncustom body"

	_, errb, code, err := runCLIInDir(t, dir, "--message", message, "set", "state.json", "status", "complete")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "log", "-1", "--format=%B")); got != message {
		t.Fatalf("commit message = %q, want %q", got, message)
	}

	_, errb, code, err = runCLIInDir(t, dir, "--message", "custom", "--subject-prefix", "feat: ", "set", "state.json", "status", "closed")
	if err == nil || code != exitUsage {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(err.Error(), "--message is mutually exclusive with subject/body message modifiers") {
		t.Fatalf("error = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD^")); got != head {
		t.Fatalf("rejected invocation changed history base to %s, want %s", got, head)
	}
}

func TestE2ECommitMessageModifiers(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open","old":true}`+"\n")
	commitAll(t, dir, "initial")
	writeFile(t, dir, "ops.etch", "set state.json status complete\ndelete state.json old\n")

	_, errb, code, err := runCLIInDir(t, dir,
		"--subject-prefix", "feat: ",
		"--subject-suffix", " [skip ci]",
		"--body-prefix", "Context: generated",
		"--body-suffix", "Refs: #1",
		"run", "ops.etch",
	)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	want := strings.Join([]string{
		"feat: etch: 2 changes in state.json [skip ci]",
		"",
		"Context: generated",
		"",
		"Changes:",
		`- set state.json $.status "complete"`,
		"- delete state.json $.old",
		"",
		"Refs: #1",
	}, "\n")
	if got := stringsTrim(testGit(t, dir, "log", "-1", "--format=%B")); got != want {
		t.Fatalf("commit message = %q, want %q", got, want)
	}
}

func TestE2EAllowEmptyCommitsMutatingNoop(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	base := commitAll(t, dir, "initial")
	baseTree := stringsTrim(testGit(t, dir, "rev-parse", "HEAD^{tree}"))

	_, errb, code, err := runCLIInDir(t, dir, "--allow-empty", "set", "state.json", "status", "open")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	head := stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
	if head == base {
		t.Fatal("--allow-empty mutating no-op did not create a commit")
	}
	if tree := stringsTrim(testGit(t, dir, "rev-parse", "HEAD^{tree}")); tree != baseTree {
		t.Fatalf("--allow-empty changed tree: %s, want %s", tree, baseTree)
	}
	if subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s")); subject != `etch set state.json $.status "open"` {
		t.Fatalf("empty commit subject = %q", subject)
	}
}

func TestE2EAllowEmptyRejectsGuardOnlyInvocation(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	head := commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "--allow-empty", "exists", "state.json")
	if err == nil || code != exitUsage {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(err.Error(), "--allow-empty requires at least one mutating command") {
		t.Fatalf("error = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("guard-only rejection moved HEAD to %s", got)
	}
}

func TestE2EFileCopyAndMove(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "src.txt", "source\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "copy", "src.txt", "copy.txt")
	if err != nil || code != exitOK {
		t.Fatalf("copy code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:src.txt"); got != "source\n" {
		t.Fatalf("HEAD src.txt = %q", got)
	}
	if got := testGit(t, dir, "show", "HEAD:copy.txt"); got != "source\n" {
		t.Fatalf("HEAD copy.txt = %q", got)
	}

	_, errb, code, err = runCLIInDir(t, dir, "move", "copy.txt", "moved.txt")
	if err != nil || code != exitOK {
		t.Fatalf("move code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if err := testGitMayFail(t, dir, "show", "HEAD:copy.txt"); err == nil {
		t.Fatal("moved source still exists in HEAD")
	}
	if got := testGit(t, dir, "show", "HEAD:moved.txt"); got != "source\n" {
		t.Fatalf("HEAD moved.txt = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "copy.txt")); !isNoSuch(err) {
		t.Fatalf("moved source still exists in worktree: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "moved.txt")); err != nil || string(got) != "source\n" {
		t.Fatalf("worktree moved.txt = %q err=%v", got, err)
	}
}

func TestE2EGuardsUseAdmittedView(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")
	head := stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
	if err := os.Remove(filepath.Join(dir, "state.json")); err != nil {
		t.Fatal(err)
	}

	_, errb, code, err := runCLIInDir(t, dir, "exists", "state.json")
	if err != nil || code != exitOK {
		t.Fatalf("exists tracked-deleted code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(errb.String(), "nothing to do") {
		t.Fatalf("guard-only stderr = %s", errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("guard-only exists moved HEAD to %s", got)
	}

	writeFile(t, dir, "local.txt", "local\n")
	_, errb, code, err = runCLIInDir(t, dir, "missing", "local.txt")
	if err != nil || code != exitOK {
		t.Fatalf("missing untracked code=%d err=%v stderr=%s", code, err, errb.String())
	}

	_, errb, code, err = runCLIInDir(t, dir, "--untracked", "missing", "local.txt")
	if err == nil || code != exitFailure {
		t.Fatalf("--untracked missing code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(err.Error(), "guard failed: missing local.txt") {
		t.Fatalf("error = %v", err)
	}

	_, errb, code, err = runCLIInDir(t, dir, "--untracked", "exists", "local.txt")
	if err != nil || code != exitOK {
		t.Fatalf("--untracked exists code=%d err=%v stderr=%s", code, err, errb.String())
	}
}

func TestE2EContainsGuardWithMultilineHeredoc(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.txt", "line one\nline two\nline three\n")
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")
	writeFile(t, dir, "ops.etch", "contains note.txt <<EOF\nline two\nline three\nEOF\nset state.json status complete\n")

	_, errb, code, err := runCLIInDir(t, dir, "run", "ops.etch")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:state.json"); !strings.Contains(got, `"status":"complete"`) {
		t.Fatalf("HEAD state.json = %s", got)
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

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"append", "a.json", "items", "1"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("append code=%d err=%v stderr=%s", code, err, errb.String())
	}
	code, err = runCLIAt(dir, []string{"add", "b.json", "items", "1"}, &out, &errb)
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

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"create", "a.json"}, &out, &errb)
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

	cases := map[string]string{
		"config.yaml": "{}\n",
		"note.md":     "",
		"data.csv":    "",
		"plain.txt":   "{}",
	}
	for path, want := range cases {
		var out, errb bytes.Buffer
		code, err := runCLIAt(dir, []string{"create", path}, &out, &errb)
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

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"--no-checkout", "set", "state.json", "status", "complete"}, &out, &errb)
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
	if !strings.Contains(err.Error(), "bad.etch:2:") {
		t.Fatalf("bad run error missing script location: %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed multi-op moved HEAD to %s", got)
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if strings.Contains(headBytes, "other") {
		t.Fatalf("failed multi-op leaked earlier mutation: %s", headBytes)
	}
}

func TestE2EDetachedHEADCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	base := commitAll(t, dir, "initial")
	testGit(t, dir, "checkout", "--detach", "HEAD")

	_, errb, code, err := runCLIInDir(t, dir, "set", "state.json", "status", "complete")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	head := stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
	if head == base {
		t.Fatal("detached HEAD invocation did not commit")
	}
	if err := testGitMayFail(t, dir, "symbolic-ref", "-q", "HEAD"); err == nil {
		t.Fatal("detached HEAD became symbolic")
	}
	if got := testGit(t, dir, "show", "HEAD:state.json"); !strings.Contains(got, `"status":"complete"`) {
		t.Fatalf("HEAD state.json = %s", got)
	}
}

func TestE2EUnbornBranchCommit(t *testing.T) {
	dir := initRepo(t)

	_, errb, code, err := runCLIInDir(t, dir, "create", "README.md", "# hi\n")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:README.md"); got != "# hi\n" {
		t.Fatalf("HEAD README.md = %q", got)
	}
	if got := stringsTrim(testGit(t, dir, "symbolic-ref", "-q", "HEAD")); got != "refs/heads/main" {
		t.Fatalf("HEAD ref = %q", got)
	}
}

func TestE2ERunDefaultsToStdin(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")

	oldReadStdin := readStdin
	readStdin = func() ([]byte, error) {
		return []byte("set stdin.json x 1\nset stdin.json y 2\n"), nil
	}
	t.Cleanup(func() { readStdin = oldReadStdin })

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"run"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:stdin.json")
	if !strings.Contains(headBytes, `"x": 1`) || !strings.Contains(headBytes, `"y": 2`) {
		t.Fatalf("HEAD stdin.json = %s", headBytes)
	}
}
