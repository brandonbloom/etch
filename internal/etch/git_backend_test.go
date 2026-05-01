package etch

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	tree, err := w.writePlannedTree(plan)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := w.writeCommitObject(tree, plan.Commit.Message)
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

func TestDryRunShorthandDoesNotCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	head := commitAll(t, dir, "initial")
	objectsBefore := gitObjectFiles(t, dir)
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"-n", "set", "state.json", "status", "complete"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("-n advanced HEAD to %s, want %s", got, head)
	}
	if objectsAfter := gitObjectFiles(t, dir); !reflect.DeepEqual(objectsAfter, objectsBefore) {
		t.Fatalf("-n wrote repository objects\nbefore=%v\nafter=%v", objectsBefore, objectsAfter)
	}
	patch := out.String()
	if !strings.Contains(patch, "Etch-Plan-Hash:") || !strings.Contains(patch, "diff --git") {
		t.Fatalf("-n output is not a dry-run patch:\n%s", patch)
	}
}

func TestPlanDoesNotWriteRepositoryObjects(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	head := commitAll(t, dir, "initial")
	objectsBefore := gitObjectFiles(t, dir)
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"--plan", "set", "state.json", "status", "complete"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("--plan advanced HEAD to %s, want %s", got, head)
	}
	if objectsAfter := gitObjectFiles(t, dir); !reflect.DeepEqual(objectsAfter, objectsBefore) {
		t.Fatalf("--plan wrote repository objects\nbefore=%v\nafter=%v", objectsBefore, objectsAfter)
	}
	planJSON := out.String()
	if !strings.Contains(planJSON, `"tree":`) || !strings.Contains(planJSON, `"commit":`) {
		t.Fatalf("--plan output missing tree or commit:\n%s", planJSON)
	}
}

func gitObjectFiles(t *testing.T, dir string) []string {
	t.Helper()
	objectDir := stringsTrim(testGit(t, dir, "rev-parse", "--git-path", "objects"))
	if !filepath.IsAbs(objectDir) {
		objectDir = filepath.Join(dir, objectDir)
	}
	var files []string
	if err := filepath.WalkDir(objectDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(objectDir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}
