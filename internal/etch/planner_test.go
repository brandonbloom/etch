package etch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanFileCreateAndGuardNoSideEffects(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	head := commitAll(t, dir, "initial")
	chdir(t, dir)

	ops := []Operation{}
	for _, tokens := range [][]string{
		{"exists", "README.md"},
		{"create", "notes/today.md", "hello\n"},
	} {
		op, err := DecodeOperation(Statement{Tokens: tokens})
		if err != nil {
			t.Fatal(err)
		}
		ops = append(ops, op)
	}
	w, err := OpenWorkspace(false)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanOperations(w, GlobalOptions{}, ops)
	if err != nil {
		t.Fatal(err)
	}
	if plan.BaseCommit != head {
		t.Fatalf("base = %s, want %s", plan.BaseCommit, head)
	}
	if plan.Tree == "" || !strings.HasPrefix(plan.Hash, "sha256:") {
		t.Fatalf("plan tree/hash missing: %#v", plan)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("planning moved HEAD to %s", got)
	}
	if _, err := osStat(filepathJoin(dir, "notes/today.md")); err == nil {
		t.Fatal("planning wrote working-tree file")
	}
	if pf, ok := plan.Files["notes/today.md"]; !ok || pf.BeforeSHA256 != shaHex(nil) || pf.AfterSHA256 == shaHex(nil) {
		t.Fatalf("plan files = %#v", plan.Files)
	}
}

func TestPlanGuardFailurePreventsLaterOperations(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	head := commitAll(t, dir, "initial")
	chdir(t, dir)

	var ops []Operation
	for _, tokens := range [][]string{
		{"missing", "README.md"},
		{"create", "created.txt", "nope\n"},
	} {
		op, err := DecodeOperation(Statement{Tokens: tokens})
		if err != nil {
			t.Fatal(err)
		}
		ops = append(ops, op)
	}
	w, err := OpenWorkspace(false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PlanOperations(w, GlobalOptions{}, ops); err == nil {
		t.Fatal("expected guard failure")
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failure moved HEAD to %s", got)
	}
	if _, err := osStat(filepathJoin(dir, "created.txt")); err == nil {
		t.Fatal("failure wrote later operation")
	}
}

func TestPlanFileDeleteNoop(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	op, err := DecodeOperation(Statement{Tokens: []string{"delete", "missing.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	w, err := OpenWorkspace(false)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanOperations(w, GlobalOptions{}, []Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Changed {
		t.Fatal("delete of missing file marked changed")
	}
	if !plan.Operations[0].Noop {
		t.Fatal("delete of missing file not marked noop")
	}
}

func filepathJoin(elem ...string) string {
	return filepath.Join(elem...)
}

func osStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
