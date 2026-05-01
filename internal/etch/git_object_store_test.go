package etch

import (
	"os"
	"strings"
	"testing"
)

func TestPlanAndDryRunUseInjectedTempStore(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	commitAll(t, dir, "initial")
	tempRoot := t.TempDir()
	temp := &recordingTempStore{root: tempRoot}

	w, err := openWorkspaceAtWithDeps(dir, false, workspaceDeps{
		git:      realGitRunner{},
		worktree: osWorkingTreeFS{},
		temp:     temp,
	})
	if err != nil {
		t.Fatal(err)
	}
	op, err := DecodeOperation(Statement{Tokens: []string{"create", "b.txt", "b\n"}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanOperations(w, GlobalOptions{}, []Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Tree == "" {
		t.Fatal("plan tree is empty")
	}
	patch, err := RenderDryRun(w, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "diff --git") {
		t.Fatalf("dry-run patch missing diff:\n%s", patch)
	}
	if len(temp.mkdirs) != 2 {
		t.Fatalf("temp object stores = %d, want 2: %#v", len(temp.mkdirs), temp.mkdirs)
	}
	if len(temp.creates) != 2 {
		t.Fatalf("temp index files = %d, want 2: %#v", len(temp.creates), temp.creates)
	}
	for _, path := range append(append([]string{}, temp.mkdirs...), temp.creates...) {
		if !strings.HasPrefix(path, tempRoot+string(os.PathSeparator)) {
			t.Fatalf("temp path %q escaped temp root %q", path, tempRoot)
		}
	}
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp root not cleaned up: %v", entries)
	}
}

type recordingTempStore struct {
	root       string
	mkdirs     []string
	creates    []string
	removes    []string
	removeAlls []string
}

func (s *recordingTempStore) mkdir(pattern string) (string, error) {
	path, err := os.MkdirTemp(s.root, pattern)
	if err == nil {
		s.mkdirs = append(s.mkdirs, path)
	}
	return path, err
}

func (s *recordingTempStore) create(pattern string) (workspaceTempFile, error) {
	file, err := os.CreateTemp(s.root, pattern)
	if err == nil {
		s.creates = append(s.creates, file.Name())
	}
	return file, err
}

func (s *recordingTempStore) remove(path string) error {
	s.removes = append(s.removes, path)
	return os.Remove(path)
}

func (s *recordingTempStore) removeAll(path string) error {
	s.removeAlls = append(s.removeAlls, path)
	return os.RemoveAll(path)
}
