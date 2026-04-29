package etch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitBackendCommitAndRefCAS(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	head := commitAll(t, dir, "initial")
	chdir(t, dir)

	op, err := DecodeOperation(Statement{Tokens: []string{"create", "b.txt", "b\n"}})
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
	commit, err := w.createCommit(plan.Tree, plan.Commit.Message)
	if err != nil {
		t.Fatal(err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("commit-tree moved HEAD to %s", got)
	}
	if err := w.updateRef(commit); err != nil {
		t.Fatal(err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != commit {
		t.Fatalf("HEAD = %s, want %s", got, commit)
	}
	if err := w.updateRef(commit); err == nil {
		t.Fatal("stale CAS update succeeded")
	}
}

func TestDryRunAppliesWithGitAm(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	op, err := DecodeOperation(Statement{Tokens: []string{"create", "b.txt", "b\n"}})
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
	patch, err := RenderDryRun(w, plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(patch, "[PATCH") {
		t.Fatalf("dry-run subject contains PATCH prefix:\n%s", patch)
	}
	patchPath := filepath.Join(dir, "plan.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "am", patchPath)
	gotTree := stringsTrim(testGit(t, dir, "rev-parse", "HEAD^{tree}"))
	if gotTree != plan.Tree {
		t.Fatalf("git am tree = %s, want %s\npatch:\n%s", gotTree, plan.Tree, patch)
	}
	log := testGit(t, dir, "log", "-1", "--format=%B")
	if strings.Contains(log, "Etch-Plan-Hash") {
		t.Fatalf("Etch headers leaked into commit message:\n%s", log)
	}
}
