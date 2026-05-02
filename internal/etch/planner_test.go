package etch

import (
	"bytes"
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

func TestPlanGuardsDoNotAddTouchedFiles(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")

	var ops []Operation
	for _, tokens := range [][]string{
		{"missing", "no-such-file.txt"},
		{"set", "state.json", "status", "complete"},
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
	if _, ok := plan.Files["no-such-file.txt"]; ok {
		t.Fatalf("guard path included in plan files: %#v", plan.Files)
	}
	if _, ok := plan.Touched["no-such-file.txt"]; ok {
		t.Fatalf("guard path included in touched files: %#v", plan.Touched)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("plan files = %#v, want only state.json", plan.Files)
	}
}

func TestPlanPrunesCreateThenDeleteNetNoop(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")

	var ops []Operation
	for _, tokens := range [][]string{
		{"create", "transient.txt", "temporary\n"},
		{"delete", "transient.txt"},
		{"set", "state.json", "status", "complete"},
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
	if _, ok := plan.Files["transient.txt"]; ok {
		t.Fatalf("net no-op path included in plan files: %#v", plan.Files)
	}
	if _, ok := plan.Touched["transient.txt"]; ok {
		t.Fatalf("net no-op path included in touched files: %#v", plan.Touched)
	}
	if len(plan.Files) != 1 {
		t.Fatalf("plan files = %#v, want only state.json", plan.Files)
	}
}

func TestPlanMarkdownSectionAppendDescriptorAndCommitMessage(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Notes\nold\n")
	commitAll(t, dir, "initial")

	op, err := DecodeOperation(Statement{Tokens: []string{"section", "append", "note.md", "Notes", "new"}})
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
	if len(plan.Operations) != 1 {
		t.Fatalf("operations = %d, want 1", len(plan.Operations))
	}
	got := plan.Operations[0]
	if got.Class != ClassNonIdempotent {
		t.Fatalf("class = %v, want %v", got.Class, ClassNonIdempotent)
	}
	if got.Target != (PlanTarget{Path: "note.md", Part: "body", Section: "Notes"}) {
		t.Fatalf("target = %#v", got.Target)
	}
	if got.Descriptor != `section append note.md Notes "new"` {
		t.Fatalf("descriptor = %q", got.Descriptor)
	}
	if got.ValueHash != shaHex([]byte("new")) {
		t.Fatalf("value hash = %q", got.ValueHash)
	}
	if plan.Commit.Message != `etch section append note.md Notes "new"` {
		t.Fatalf("commit message = %q", plan.Commit.Message)
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

func TestPlanHashStableAcrossPlanningRuns(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.json", `{"status":"open"}`+"\n")
	writeFile(t, dir, "b.json", `{"status":"open"}`+"\n")
	commitAll(t, dir, "initial")

	planOnce := func(t *testing.T) (*Workspace, *Plan, []byte) {
		t.Helper()
		var ops []Operation
		for _, tokens := range [][]string{
			{"set", "b.json", "status", "complete"},
			{"set", "a.json", "status", "complete"},
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
		return w, plan, canonicalPlanBytes(plan)
	}

	w, first, firstBytes := planOnce(t)
	_, second, secondBytes := planOnce(t)
	if first.Hash == "" || !strings.HasPrefix(first.Hash, "sha256:") {
		t.Fatalf("plan hash missing: %#v", first)
	}
	if first.Hash != second.Hash {
		t.Fatalf("plan hash changed across runs: %s != %s", first.Hash, second.Hash)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("canonical plan bytes changed across runs:\n%s\n---\n%s", firstBytes, secondBytes)
	}

	patch, err := RenderDryRun(w, first)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "Etch-Plan-Hash: "+first.Hash+"\n") {
		t.Fatalf("dry-run missing plan hash %s:\n%s", first.Hash, patch)
	}
}

func TestBuildCommitMessageAppliesSubjectAndBodyModifiers(t *testing.T) {
	ops := []Operation{{
		Verb:       "set",
		Class:      ClassIdempotent,
		Descriptor: `set state.json $.status "done"`,
		Value:      `"done"`,
	}}

	got := buildCommitMessage(GlobalOptions{
		SubjectPrefix: "feat: ",
		SubjectSuffix: " [skip ci]",
	}, ops)
	want := `feat: etch set state.json $.status "done" [skip ci]`
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	got = buildCommitMessage(GlobalOptions{
		BodySuffix: "Refs: #1",
	}, ops)
	want = `etch set state.json $.status "done"` + "\n\nRefs: #1"
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestBuildCommitMessageJoinsBodyModifiersWithGeneratedBody(t *testing.T) {
	ops := []Operation{
		{
			Verb:       "set",
			Class:      ClassIdempotent,
			Target:     PlanTarget{Path: "state.json"},
			Descriptor: `set state.json $.status "done"`,
			Value:      `"done"`,
		},
		{
			Verb:       "delete",
			Class:      ClassIdempotent,
			Target:     PlanTarget{Path: "state.json"},
			Descriptor: `delete state.json $.old`,
		},
	}

	got := buildCommitMessage(GlobalOptions{
		SubjectPrefix: "feat: ",
		BodyPrefix:    "Context: generated",
		BodySuffix:    "\n\nRefs: #1",
	}, ops)
	want := strings.Join([]string{
		"feat: etch: 2 changes in state.json",
		"",
		"Context: generated",
		"",
		"Changes:",
		`- set state.json $.status "done"`,
		"- delete state.json $.old",
		"",
		"Refs: #1",
	}, "\n")
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestBuildCommitMessageSubjectModifiersAffectValueFallback(t *testing.T) {
	ops := []Operation{{
		Verb:       "set",
		Class:      ClassIdempotent,
		Descriptor: `set state.json $.status "1234567890123456789012345678901234567890"`,
		Value:      `"1234567890123456789012345678901234567890"`,
	}}

	got := buildCommitMessage(GlobalOptions{SubjectPrefix: "feat: "}, ops)
	want := strings.Join([]string{
		"feat: etch set state.json $.status",
		"",
		`Value: "1234567890123456789012345678901234567890"`,
	}, "\n")
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func filepathJoin(elem ...string) string {
	return filepath.Join(elem...)
}

func osStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}
