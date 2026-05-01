package etch

import (
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
