package etch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRejectsEscapingPaths(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "a.txt", "base\n")
	commitAll(t, dir, "initial")

	w, err := OpenWorkspaceAt(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../x", "/tmp/x", ".git/config", "sub/../x"} {
		if _, err := w.Resolve(path, false, true); err == nil {
			t.Fatalf("Resolve(%q) succeeded", path)
		}
	}
}

func TestWorkspaceReadsTrackedBytesFromHEAD(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"status":"base"}`+"\n")
	commitAll(t, dir, "initial")
	writeFile(t, dir, "state.json", `{"status":"dirty"}`+"\n")

	w, err := OpenWorkspaceAt(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Resolve("state.json", false, true)
	if err != nil {
		t.Fatal(err)
	}
	got, _, absent := w.ReadBase(res)
	if absent {
		t.Fatal("state.json reported absent")
	}
	if strings.Contains(string(got), "dirty") || !strings.Contains(string(got), "base") {
		t.Fatalf("ReadBase = %q", got)
	}
}

func TestOpenWorkspaceAtUsesExplicitCWD(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "nested/state.json", `{"status":"base"}`+"\n")
	commitAll(t, dir, "initial")

	w, err := OpenWorkspaceAt(filepath.Join(dir, "nested"), false)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Resolve("state.json", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Clean != "state.json" || res.Repo != "nested/state.json" {
		t.Fatalf("Resolve from explicit CWD = clean %q repo %q", res.Clean, res.Repo)
	}
}

func TestOpenWorkspaceAtUsesInjectedGitRunner(t *testing.T) {
	dir := t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeGitRunner{
		outputs: map[string]fakeGitResponse{
			fakeGitKey("output", "rev-parse", "--show-toplevel"): {
				out: []byte(realDir + "\n"),
			},
			fakeGitKey("output", "rev-parse", "--verify", "HEAD"): {
				out: []byte("abc123\n"),
			},
			fakeGitKey("output", "symbolic-ref", "-q", "HEAD"): {
				out: []byte("refs/heads/main\n"),
			},
			fakeGitKey("output", "status", "--porcelain=v1", "-z", "--", "state.json"): {},
		},
	}

	w, err := openWorkspaceAt(dir, false, runner)
	if err != nil {
		t.Fatal(err)
	}
	if w.CWD != realDir || w.Root != realDir || w.Head != "abc123" || w.Ref != "refs/heads/main" || w.Unborn {
		t.Fatalf("workspace = %#v", w)
	}
	clean, err := w.pathClean("state.json")
	if err != nil {
		t.Fatal(err)
	}
	if !clean {
		t.Fatal("pathClean reported injected clean path dirty")
	}

	wantCalls := strings.Join([]string{
		"output rev-parse --show-toplevel",
		"output rev-parse --verify HEAD",
		"output symbolic-ref -q HEAD",
		"output status --porcelain=v1 -z -- state.json",
	}, "\n")
	if gotCalls := strings.Join(runner.calls, "\n"); gotCalls != wantCalls {
		t.Fatalf("git calls:\n%s\nwant:\n%s", gotCalls, wantCalls)
	}
}

func TestWorkspaceUntrackedAdmission(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "tracked.txt", "base\n")
	commitAll(t, dir, "initial")
	writeFile(t, dir, "local.txt", "local\n")

	w, err := OpenWorkspaceAt(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Resolve("local.txt", false, true)
	if err != nil {
		t.Fatal(err)
	}
	exists, _, _, err := w.ExistsInAdmittedView(res)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("untracked file admitted without --untracked")
	}

	w, err = OpenWorkspaceAt(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	exists, b, _, err := w.ExistsInAdmittedView(res)
	if err != nil {
		t.Fatal(err)
	}
	if !exists || string(b) != "local\n" {
		t.Fatalf("untracked admitted = %v %q", exists, b)
	}
}

func TestWorkspaceUntrackedAdmissionUsesInjectedWorkingTree(t *testing.T) {
	dir := t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "local.txt")
	runner := &fakeGitRunner{
		outputs: map[string]fakeGitResponse{
			fakeGitKey("output", "rev-parse", "--show-toplevel"): {
				out: []byte(realDir + "\n"),
			},
			fakeGitKey("output", "rev-parse", "--verify", "HEAD"): {
				out: []byte("abc123\n"),
			},
			fakeGitKey("output", "symbolic-ref", "-q", "HEAD"): {
				out: []byte("refs/heads/main\n"),
			},
			fakeGitKey("output", "show", "abc123:local.txt"): {
				err: fmt.Errorf("not tracked"),
			},
		},
	}
	worktree := &fakeWorkingTreeFS{files: map[string][]byte{
		path: []byte("local\n"),
	}}

	w, err := openWorkspaceAtWithDeps(dir, true, workspaceDeps{git: runner, worktree: worktree})
	if err != nil {
		t.Fatal(err)
	}
	exists, b, mode, err := w.ExistsInAdmittedView(ResolvedPath{Clean: "local.txt", Abs: path, Repo: "local.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !exists || string(b) != "local\n" || mode != "100644" {
		t.Fatalf("ExistsInAdmittedView = exists %v bytes %q mode %q", exists, b, mode)
	}
	if got, want := strings.Join(worktree.reads, "\n"), path; got != want {
		t.Fatalf("working tree reads = %q, want %q", got, want)
	}
}

func TestWorkspaceContainedSymlinkAllowedEscapeRejected(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "real/data.json", `{"a":1}`+"\n")
	if err := os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Dir(dir), filepath.Join(dir, "escape")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	testGit(t, dir, "add", ".")
	testGit(t, dir, "commit", "-m", "symlinks")
	w, err := OpenWorkspaceAt(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Resolve("link/data.json", false, true); err != nil {
		t.Fatalf("contained symlink rejected: %v", err)
	}
	if _, err := w.Resolve("escape/outside", true, true); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

type fakeGitResponse struct {
	out []byte
	err error
}

type fakeGitRunner struct {
	outputs map[string]fakeGitResponse
	calls   []string
}

func (f *fakeGitRunner) output(dir string, env []string, args ...string) ([]byte, error) {
	return f.respond("output", args...)
}

func (f *fakeGitRunner) run(dir string, env []string, stdin []byte, args ...string) error {
	_, err := f.respond("run", args...)
	return err
}

func (f *fakeGitRunner) outputStdin(dir string, env []string, stdin []byte, args ...string) ([]byte, error) {
	return f.respond("outputStdin", args...)
}

func (f *fakeGitRunner) respond(kind string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, kind+" "+strings.Join(args, " "))
	resp, ok := f.outputs[fakeGitKey(kind, args...)]
	if !ok {
		return nil, fmt.Errorf("unexpected fake git call: %s %s", kind, strings.Join(args, " "))
	}
	return append([]byte(nil), resp.out...), resp.err
}

func fakeGitKey(kind string, args ...string) string {
	return kind + "\x00" + strings.Join(args, "\x00")
}

type fakeWorkingTreeFS struct {
	files  map[string][]byte
	reads  []string
	writes []string
}

func (f *fakeWorkingTreeFS) readFile(path string) ([]byte, error) {
	f.reads = append(f.reads, path)
	b, ok := f.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), b...), nil
}

func (f *fakeWorkingTreeFS) writeFile(path string, b []byte, perm os.FileMode) error {
	f.writes = append(f.writes, path)
	f.files[path] = append([]byte(nil), b...)
	return nil
}

func (f *fakeWorkingTreeFS) mkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (f *fakeWorkingTreeFS) remove(path string) error {
	delete(f.files, path)
	return nil
}
