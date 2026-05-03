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

func TestE2EInvalidJSONInputErrorIsUserFacing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "syntax", input: "not json\n"},
		{name: "trailing data", input: "{} true\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "state.json", tc.input)
			head := commitAll(t, dir, "initial")

			_, errb, code, err := runCLIInDir(t, dir, "set", "state.json", "status", "complete")
			if err == nil || code != exitFailure {
				t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
			}
			msg := err.Error()
			if !strings.Contains(msg, "state.json is not valid JSON (parse error near offset ") {
				t.Fatalf("error = %v", err)
			}
			for _, internal := range []string{"jsontext", "invalid character", "literal null", "trailing data"} {
				if strings.Contains(msg, internal) {
					t.Fatalf("error leaked parser detail %q: %v", internal, err)
				}
			}
			if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
				t.Fatalf("failed JSON edit moved HEAD to %s", got)
			}
		})
	}
}

func TestE2EJSONLAppendCommitsAndMaterializes(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "events.jsonl", `{"kind":"base"}`+"\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "append", "events.jsonl", `{"kind":"prompt","n":2}`)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	want := `{"kind":"base"}` + "\n" + `{"kind":"prompt","n":2}` + "\n"
	headBytes := testGit(t, dir, "show", "HEAD:events.jsonl")
	if headBytes != want {
		t.Fatalf("HEAD events.jsonl = %q, want %q", headBytes, want)
	}
	wt, readErr := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(wt) != headBytes {
		t.Fatalf("worktree not materialized:\nwt=%s\nhead=%s", wt, headBytes)
	}
	subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s"))
	if subject != `etch append events.jsonl {"kind":"prompt","n":2}` {
		t.Fatalf("subject = %q", subject)
	}
}

func TestE2EJSONLAppendCreatesMissingLog(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "jsonl", "append", "events.log", `{"kind":"prompt"}`)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:events.log"); got != `{"kind":"prompt"}`+"\n" {
		t.Fatalf("HEAD events.log = %q", got)
	}
}

func TestE2EJSONLAppendBoundaryErrorDoesNotCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "events.jsonl", `{"kind":"base"}`)
	head := commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "append", "events.jsonl", `{"kind":"prompt"}`)
	if err == nil || code != exitFailure {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(err.Error(), "must end with a newline") {
		t.Fatalf("error = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed JSONL append moved HEAD to %s", got)
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

func TestE2EMissingGuardDoesNotBreakMaterialization(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	writeFile(t, dir, "ops.etch", "missing no-such-file.txt\nset script-output.json x --json true\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "run", "ops.etch")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:script-output.json"); !strings.Contains(got, `"x": true`) {
		t.Fatalf("HEAD script-output.json = %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "no-such-file.txt")); !isNoSuch(err) {
		t.Fatalf("missing guard path materialized: %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "status", "--porcelain")); got != "" {
		t.Fatalf("worktree dirty after missing guard materialization:\n%s", got)
	}
}

func TestE2ECreateThenDeleteDoesNotBreakMaterialization(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	writeFile(t, dir, "ops.etch", "create transient.txt temporary\ndelete transient.txt\nset state.json status complete\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "run", "ops.etch")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:state.json"); !strings.Contains(got, `"status":"complete"`) {
		t.Fatalf("HEAD state.json = %s", got)
	}
	if err := testGitMayFail(t, dir, "show", "HEAD:transient.txt"); err == nil {
		t.Fatal("transient path exists in HEAD")
	}
	if _, err := os.Stat(filepath.Join(dir, "transient.txt")); !isNoSuch(err) {
		t.Fatalf("transient path materialized: %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "status", "--porcelain")); got != "" {
		t.Fatalf("worktree dirty after create-delete materialization:\n%s", got)
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
	if !strings.Contains(headBytes, `"x": "1"`) {
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
	code, err := runCLIAt(dir, []string{"append", "a.json", "items", "--json", "1"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("append code=%d err=%v stderr=%s", code, err, errb.String())
	}
	code, err = runCLIAt(dir, []string{"add", "b.json", "items", "--json", "1"}, &out, &errb)
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

func TestE2EJSONRemoveRootArray(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "root.json", `["x","y","z","y"]`+"\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "remove", "root.json", "$", "y")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:root.json"); got != `["x","z"]`+"\n" {
		t.Fatalf("HEAD root.json = %s", got)
	}
}

func TestE2EValueSyntaxAndAssignmentItems(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"events":[]}`+"\n")
	writeFile(t, dir, "config.yaml", "{}\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "set", "state.json", "positional", "12")
	runOK(t, dir, "set", "state.json", "typed", "--json", "12")
	runOK(t, dir, "append", "state.json", "events", "--json", `{"kind":"prompt"}`)
	runOK(t, dir, "set", "state.json", "literal=12", "multi:=12", "empty=", "nil:=null", `$["a=b"]=value`)
	runOK(t, dir, "set", "config.yaml", "enabled=true", "native:=true")

	jsonOut := testGit(t, dir, "show", "HEAD:state.json")
	for _, want := range []string{
		`"positional":"12"`,
		`"typed":12`,
		`"kind":"prompt"`,
		`"literal":"12"`,
		`"multi":12`,
		`"empty":""`,
		`"nil":null`,
		`"a=b":"value"`,
	} {
		if !strings.Contains(jsonOut, want) {
			t.Fatalf("JSON output missing %q:\n%s", want, jsonOut)
		}
	}

	yamlOut := testGit(t, dir, "show", "HEAD:config.yaml")
	for _, want := range []string{`enabled: "true"`, "native: true"} {
		if !strings.Contains(yamlOut, want) {
			t.Fatalf("YAML output missing %q:\n%s", want, yamlOut)
		}
	}
}

func TestE2EAssignmentItemErrorsDoNotCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"events":[]}`+"\n")
	head := commitAll(t, dir, "initial")

	cases := [][]string{
		{"set", "state.json", "a=1", "$.a=2"},
		{"set", "state.json", "a", "1", "b=2"},
		{"append", "state.json", "events:={\"kind\":\"prompt\"}"},
	}
	for _, args := range cases {
		var out, errb bytes.Buffer
		code, err := runCLIAt(dir, args, &out, &errb)
		if err == nil || code == exitOK {
			t.Fatalf("etch %v unexpectedly succeeded stdout=%s stderr=%s", args, out.String(), errb.String())
		}
		if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
			t.Fatalf("failed %v moved HEAD to %s", args, got)
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

func TestE2ECreateNoopsWhenExistingContentMatches(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	head := commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "create", "README.md", "# hi\n")
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(errb.String(), "nothing to do") {
		t.Fatalf("stderr = %s", errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("idempotent create moved HEAD to %s", got)
	}
}

func TestE2ECreateErrorsWhenExistingContentDiffers(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	head := commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "create", "README.md", "# bye\n")
	if err == nil || code != exitFailure {
		t.Fatalf("create unexpectedly succeeded code=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(err.Error(), "already exists with different content") {
		t.Fatalf("error = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed create moved HEAD to %s", got)
	}
}

func TestE2EReplaceFileContent(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.txt", "old\n")
	commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "replace", "note.txt", "new\n")
	if err != nil || code != exitOK {
		t.Fatalf("replace code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := testGit(t, dir, "show", "HEAD:note.txt"); got != "new\n" {
		t.Fatalf("HEAD note.txt = %q", got)
	}
	wt, readErr := os.ReadFile(filepath.Join(dir, "note.txt"))
	if readErr != nil || string(wt) != "new\n" {
		t.Fatalf("worktree note.txt = %q err=%v", wt, readErr)
	}
}

func TestE2EReplaceNoopsWhenContentMatches(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.txt", "same\n")
	head := commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "replace", "note.txt", "same\n")
	if err != nil || code != exitOK {
		t.Fatalf("replace code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if !strings.Contains(errb.String(), "nothing to do") {
		t.Fatalf("stderr = %s", errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("idempotent replace moved HEAD to %s", got)
	}
}

func TestE2EReplaceMissingFileErrors(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	head := commitAll(t, dir, "initial")

	_, errb, code, err := runCLIInDir(t, dir, "replace", "missing.txt", "new\n")
	if err == nil || code != exitFailure {
		t.Fatalf("replace unexpectedly succeeded code=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(err.Error(), "missing.txt is missing") {
		t.Fatalf("error = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed replace moved HEAD to %s", got)
	}
}

func TestE2ECreateExtensionDefaults(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")

	cases := map[string]string{
		"config.yaml":  "{}\n",
		"events.jsonl": "",
		"note.md":      "",
		"data.csv":     "",
		"plain.txt":    "{}",
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
		return []byte("set stdin.json x 1\nset stdin.json y --json 2\n"), nil
	}
	t.Cleanup(func() { readStdin = oldReadStdin })

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"run"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	headBytes := testGit(t, dir, "show", "HEAD:stdin.json")
	if !strings.Contains(headBytes, `"x": "1"`) || !strings.Contains(headBytes, `"y": 2`) {
		t.Fatalf("HEAD stdin.json = %s", headBytes)
	}
}
