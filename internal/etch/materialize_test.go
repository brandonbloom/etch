package etch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializationDirtyWorktreeConflictDoesNotRollbackCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open","note":"base"}`+"\n")
	base := commitAll(t, dir, "initial")
	writeFile(t, dir, "state.json", `{"status":"open","note":"local"}`+"\n")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"set", "state.json", "status", "complete"}, &out, &errb)
	if err == nil || code != exitFailure {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	head := stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
	if head == base {
		t.Fatal("materialization conflict rolled back commit")
	}
	headBytes := testGit(t, dir, "show", "HEAD:state.json")
	if !strings.Contains(headBytes, `"status":"complete"`) {
		t.Fatalf("commit missing mutation: %s", headBytes)
	}
	wt, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if !bytes.Contains(wt, []byte("<<<<<<<")) {
		t.Fatalf("worktree lacks conflict markers:\n%s\nstderr:\n%s", wt, errb.String())
	}
	if !strings.Contains(errb.String(), "HEAD was updated") {
		t.Fatalf("stderr missing recovery prompt:\n%s", errb.String())
	}
}

func TestMaterializationAddAddConflict(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	base := commitAll(t, dir, "initial")
	writeFile(t, dir, "new.txt", "local\n")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"create", "new.txt", "etch\n"}, &out, &errb)
	if err == nil || code != exitFailure {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got == base {
		t.Fatal("add/add conflict rolled back commit")
	}
	wt, _ := os.ReadFile(filepath.Join(dir, "new.txt"))
	if !bytes.Contains(wt, []byte("<<<<<<<")) || !bytes.Contains(wt, []byte("local")) || !bytes.Contains(wt, []byte("etch")) {
		t.Fatalf("add/add conflict markers wrong:\n%s", wt)
	}
}

func TestMaterializationDeleteModifyConflict(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "old.txt", "base\n")
	base := commitAll(t, dir, "initial")
	writeFile(t, dir, "old.txt", "local\n")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"delete", "old.txt"}, &out, &errb)
	if err == nil || code != exitFailure {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got == base {
		t.Fatal("delete/modify conflict rolled back commit")
	}
	if _, err := gitOutput(dir, nil, "show", "HEAD:old.txt"); err == nil {
		t.Fatal("deleted path still exists in HEAD")
	}
	wt, _ := os.ReadFile(filepath.Join(dir, "old.txt"))
	if !bytes.Contains(wt, []byte("<<<<<<<")) || !bytes.Contains(wt, []byte("local")) {
		t.Fatalf("delete/modify conflict markers wrong:\n%s", wt)
	}
}

func TestMaterializationBinaryRefusalDoesNotOverwrite(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "old.bin", "base\n")
	base := commitAll(t, dir, "initial")
	if err := os.WriteFile(filepath.Join(dir, "old.bin"), []byte{'l', 0, 'x'}, 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"delete", "old.bin"}, &out, &errb)
	if err == nil || code != exitFailure {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got == base {
		t.Fatal("binary refusal rolled back commit")
	}
	wt, _ := os.ReadFile(filepath.Join(dir, "old.bin"))
	if !bytes.Equal(wt, []byte{'l', 0, 'x'}) {
		t.Fatalf("binary worktree overwritten: %v", wt)
	}
	if !strings.Contains(errb.String(), "binary local change could not be merged") {
		t.Fatalf("stderr = %s", errb.String())
	}
}

func TestMergeStateAllPresentTextConflict(t *testing.T) {
	got, absent, conflict, err := mergeState(
		[]byte("base\n"), false,
		[]byte("ours\n"), false,
		[]byte("theirs\n"), false,
		"ours", "theirs",
	)
	if err != nil {
		t.Fatal(err)
	}
	if absent || !conflict {
		t.Fatalf("mergeState absent=%v conflict=%v, want present conflict", absent, conflict)
	}
	for _, want := range []string{"<<<<<<< ours", "||||||| base", "=======", ">>>>>>> theirs"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("conflict output missing %q:\n%s", want, got)
		}
	}
}
