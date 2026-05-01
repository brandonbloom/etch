package etch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPlanFileCreateAndGuardNoSideEffects(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	head := commitAll(t, dir, "initial")
	objectsBefore := gitObjectFiles(t, dir)

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
	w, err := OpenWorkspaceAt(dir, false)
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
	if objectsAfter := gitObjectFiles(t, dir); !reflect.DeepEqual(objectsAfter, objectsBefore) {
		t.Fatalf("planning wrote repository objects\nbefore=%v\nafter=%v", objectsBefore, objectsAfter)
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
	w, err := OpenWorkspaceAt(dir, false)
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

	op, err := DecodeOperation(Statement{Tokens: []string{"delete", "missing.txt"}})
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
	if plan.Changed {
		t.Fatal("delete of missing file marked changed")
	}
	if !plan.Operations[0].Noop {
		t.Fatal("delete of missing file not marked noop")
	}
}

func TestPlanHashUsesJCSCanonicalBytes(t *testing.T) {
	plan := &Plan{
		Schema:     "schema",
		Ref:        "refs/heads/main",
		BaseCommit: "abc",
		Operations: []Operation{{
			Verb:      "set",
			Target:    PlanTarget{Path: "z.json", Selector: "$.a"},
			ValueHash: "sha256:value",
		}},
		Files: map[string]PlanFile{
			"z.json": {BeforeSHA256: "before-z", AfterSHA256: "after-z"},
			"a.json": {BeforeSHA256: "before-a", AfterSHA256: "after-a"},
		},
		Tree:     "tree",
		Commit:   PlanCommit{Message: "msg"},
		Hash:     "ignored",
		Touched:  map[string]fileChange{"ignored": {}},
		Mutating: true,
		Changed:  true,
	}

	want := `{"$schema":"schema","base_commit":"abc","commit":{"message":"msg"},"files":{"a.json":{"after_sha256":"after-a","before_sha256":"before-a"},"z.json":{"after_sha256":"after-z","before_sha256":"before-z"}},"operations":[{"target":{"path":"z.json","selector":"$.a"},"value_sha256":"sha256:value","verb":"set"}],"ref":"refs/heads/main","tree":"tree"}`
	got := canonicalPlanBytes(plan)
	if string(got) != want {
		t.Fatalf("canonical bytes = %s, want %s", got, want)
	}
	hash := planHash(plan)
	if hash != "sha256:"+shaHex([]byte(want)) {
		t.Fatalf("hash = %s, want sha256:%s", hash, shaHex([]byte(want)))
	}
}

func filepathJoin(elem ...string) string {
	return filepath.Join(elem...)
}

func osStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
