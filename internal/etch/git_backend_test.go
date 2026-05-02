package etch

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/brandonbloom/etch/internal/jsonx"
)

func TestGitBackendCommitAndRefCAS(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	head := commitAll(t, dir, "initial")

	op, err := DecodeOperation(Statement{Tokens: []string{"create", "b.txt", "b\n"}})
	if err != nil {
		t.Fatal(err)
	}
	w, err := OpenWorkspaceAt(dir, false)
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

	op, err := DecodeOperation(Statement{Tokens: []string{"create", "b.txt", "b\n"}})
	if err != nil {
		t.Fatal(err)
	}
	w, err := OpenWorkspaceAt(dir, false)
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

func TestDryRunAppliesMultiOperationPatchWithGitAm(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	writeFile(t, dir, "old.txt", "old\n")
	writeFile(t, dir, "src.txt", "source\n")
	writeFile(t, dir, "move.txt", "move\n")
	commitAll(t, dir, "initial")
	writeFile(t, dir, "ops.etch", strings.Join([]string{
		"set state.json status complete",
		"delete old.txt",
		"copy src.txt copied.txt",
		"move move.txt moved.txt",
		"",
	}, "\n"))

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"--dry-run", "run", "ops.etch"}, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("dry-run code=%d err=%v stderr=%s", code, err, errb.String())
	}
	patch := out.String()
	for _, want := range []string{"Etch-Plan-Hash:", "diff --git", "deleted file mode", "copied.txt", "moved.txt"} {
		if !strings.Contains(patch, want) {
			t.Fatalf("dry-run patch missing %q:\n%s", want, patch)
		}
	}
	patchPath := filepath.Join(dir, "multi.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, dir, "am", patchPath)

	if got := testGit(t, dir, "show", "HEAD:state.json"); !strings.Contains(got, `"status":"complete"`) {
		t.Fatalf("HEAD state.json = %s", got)
	}
	if err := testGitMayFail(t, dir, "show", "HEAD:old.txt"); err == nil {
		t.Fatal("deleted file still exists after git am")
	}
	if got := testGit(t, dir, "show", "HEAD:copied.txt"); got != "source\n" {
		t.Fatalf("HEAD copied.txt = %q", got)
	}
	if err := testGitMayFail(t, dir, "show", "HEAD:move.txt"); err == nil {
		t.Fatal("moved source still exists after git am")
	}
	if got := testGit(t, dir, "show", "HEAD:moved.txt"); got != "move\n" {
		t.Fatalf("HEAD moved.txt = %q", got)
	}
}

func TestDryRunShorthandDoesNotCommit(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	head := commitAll(t, dir, "initial")
	objectsBefore := gitObjectFiles(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"-n", "set", "state.json", "status", "complete"}, &out, &errb)
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

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"--plan", "set", "state.json", "status", "complete"}, &out, &errb)
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
	var plan Plan
	if err := jsonx.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("--plan output is not plan JSON: %v\n%s", err, planJSON)
	}
	if plan.Schema != planSchema {
		t.Fatalf("plan schema = %q, want %q", plan.Schema, planSchema)
	}
	if plan.Ref != "refs/heads/main" || plan.BaseCommit != head {
		t.Fatalf("plan ref/base = %q/%q, want refs/heads/main/%s", plan.Ref, plan.BaseCommit, head)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("plan operations = %#v", plan.Operations)
	}
	op := plan.Operations[0]
	if op.Verb != "set" || op.Target.Path != "state.json" || op.Target.Selector != "$.status" || op.ValueHash != shaHex([]byte("complete")) {
		t.Fatalf("plan operation = %#v", op)
	}
	wantFiles := map[string]PlanFile{
		"state.json": {
			BeforeSHA256: shaHex([]byte(`{"status":"open"}` + "\n")),
			AfterSHA256:  shaHex([]byte(`{"status":"complete"}` + "\n")),
		},
	}
	if !reflect.DeepEqual(plan.Files, wantFiles) {
		t.Fatalf("plan files = %#v, want %#v", plan.Files, wantFiles)
	}
	if plan.Tree == "" || plan.Tree == stringsTrim(testGit(t, dir, "rev-parse", "HEAD^{tree}")) {
		t.Fatalf("plan tree = %q", plan.Tree)
	}
	if plan.Commit.Message != `etch set state.json $.status "complete"` {
		t.Fatalf("plan commit message = %q", plan.Commit.Message)
	}
	if plan.Hash != "" || plan.Touched != nil || plan.Mutating || plan.Changed {
		t.Fatalf("plan output populated internal fields: %#v", plan)
	}

	var raw map[string]any
	if err := jsonx.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"hash", "touched", "mutating", "changed"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("--plan output includes internal field %q:\n%s", key, planJSON)
		}
	}
}

func TestNoCheckoutIgnoredForPreviewModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "plan",
			args: []string{"--no-checkout", "--plan", "set", "state.json", "status", "complete"},
			want: `"tree":`,
		},
		{
			name: "dry-run",
			args: []string{"--no-checkout", "--dry-run", "set", "state.json", "status", "complete"},
			want: "diff --git",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
			head := commitAll(t, dir, "initial")
			objectsBefore := gitObjectFiles(t, dir)

			var out, errb bytes.Buffer
			code, err := runCLIAt(dir, tc.args, &out, &errb)
			if err != nil || code != exitOK {
				t.Fatalf("runCLI code=%d err=%v stderr=%s", code, err, errb.String())
			}
			if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
				t.Fatalf("%s advanced HEAD to %s, want %s", tc.name, got, head)
			}
			if objectsAfter := gitObjectFiles(t, dir); !reflect.DeepEqual(objectsAfter, objectsBefore) {
				t.Fatalf("%s wrote repository objects\nbefore=%v\nafter=%v", tc.name, objectsBefore, objectsAfter)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("%s output missing %q:\n%s", tc.name, tc.want, out.String())
			}
			if strings.Contains(errb.String(), "checkout skipped") {
				t.Fatalf("%s reported checkout skip despite preview mode:\n%s", tc.name, errb.String())
			}
		})
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
