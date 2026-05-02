package etch

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

// Workspace captures the repository snapshot and working directory that an etch
// invocation reasons about. All process, Git, and filesystem effects are routed
// through its private dependency bundle.
type Workspace struct {
	CWD       string
	Root      string
	Head      string
	Ref       string
	Unborn    bool
	Untracked bool
	deps      workspaceDeps
}

// OpenWorkspace opens the process working directory. Prefer OpenWorkspaceAt
// when the caller already has an explicit directory.
func OpenWorkspace(untracked bool) (*Workspace, error) {
	return openWorkspace(untracked, workspaceDeps{})
}

func openWorkspace(untracked bool, deps workspaceDeps) (*Workspace, error) {
	deps = deps.withDefaults()
	cwd, err := deps.paths.getwd()
	if err != nil {
		return nil, err
	}
	return openWorkspaceAt(cwd, untracked, deps)
}

// OpenWorkspaceAt opens the Git worktree containing cwd.
func OpenWorkspaceAt(cwd string, untracked bool) (*Workspace, error) {
	return openWorkspaceAt(cwd, untracked, workspaceDeps{})
}

func openWorkspaceAt(cwd string, untracked bool, deps workspaceDeps) (*Workspace, error) {
	deps = deps.withDefaults()
	abs, err := deps.paths.abs(cwd)
	if err != nil {
		return nil, err
	}
	cwd = abs
	if real, err := deps.paths.evalSymlinks(cwd); err == nil {
		cwd = real
	} else {
		return nil, failf("cannot resolve working directory %s: %v", cwd, err)
	}
	rootBytes, err := deps.git.output(cwd, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, failf("not inside a git worktree")
	}
	root := strings.TrimSpace(string(rootBytes))
	if real, err := deps.paths.evalSymlinks(root); err == nil {
		root = real
	}
	headBytes, err := deps.git.output(cwd, nil, "rev-parse", "--verify", "HEAD")
	unborn := false
	head := ""
	if err != nil {
		unborn = true
	} else {
		head = strings.TrimSpace(string(headBytes))
	}
	refBytes, err := deps.git.output(cwd, nil, "symbolic-ref", "-q", "HEAD")
	ref := "HEAD"
	if err == nil {
		ref = strings.TrimSpace(string(refBytes))
	}
	return &Workspace{CWD: cwd, Root: root, Head: head, Ref: ref, Unborn: unborn, Untracked: untracked, deps: deps}, nil
}

type ResolvedPath struct {
	Input string
	Clean string
	Abs   string
	Repo  string
}

func (w *Workspace) Resolve(input string, mayBeMissing bool, finalSymlink bool) (ResolvedPath, error) {
	if input == "" {
		return ResolvedPath{}, usagef("empty path")
	}
	if filepath.IsAbs(input) {
		return ResolvedPath{}, usagef("path %s is absolute; paths must be relative to CWD", input)
	}
	parts := strings.FieldsFunc(filepath.ToSlash(input), func(r rune) bool { return r == '/' })
	for _, p := range parts {
		if p == ".." {
			return ResolvedPath{}, usagef("path %s contains ..", input)
		}
		if p == ".git" {
			return ResolvedPath{}, usagef("path %s contains .git", input)
		}
	}
	clean := filepath.Clean(input)
	if clean == "." {
		return ResolvedPath{}, usagef("path %s resolves to CWD", input)
	}
	abs := filepath.Join(w.CWD, clean)
	checkAbs := abs
	if mayBeMissing {
		checkAbs = filepath.Dir(abs)
	}
	paths := w.pathResolver()
	if finalSymlink && !mayBeMissing {
		if real, err := paths.evalSymlinks(abs); err == nil {
			checkAbs = real
			abs = real
		} else if !isNoSuch(err) {
			return ResolvedPath{}, failf("%s: %v", input, err)
		}
	} else if real, err := paths.evalSymlinks(checkAbs); err == nil {
		checkAbs = real
	} else if !isNoSuch(err) {
		return ResolvedPath{}, failf("%s: %v", input, err)
	}
	relToCWD, err := filepath.Rel(w.CWD, checkAbs)
	if err != nil || relToCWD == ".." || strings.HasPrefix(relToCWD, ".."+string(filepath.Separator)) {
		return ResolvedPath{}, usagef("path %s escapes CWD", input)
	}
	repo, err := filepath.Rel(w.Root, abs)
	if err != nil || repo == ".." || strings.HasPrefix(repo, ".."+string(filepath.Separator)) {
		return ResolvedPath{}, usagef("path %s escapes git worktree", input)
	}
	return ResolvedPath{
		Input: input,
		Clean: cleanSlash(clean),
		Abs:   abs,
		Repo:  filepath.ToSlash(repo),
	}, nil
}

func (w *Workspace) ReadBase(path ResolvedPath) ([]byte, string, bool, error) {
	if w.Unborn {
		return nil, "100644", true, nil
	}
	git := w.gitRunner()
	out, err := git.output(w.CWD, nil, "show", w.Head+":"+path.Repo)
	if err != nil {
		if w.Untracked {
			b, readErr := w.workingTreeFS().readFile(path.Abs)
			if readErr == nil {
				return b, "100644", false, nil
			}
			if !isNoSuch(readErr) {
				return nil, "100644", true, readErr
			}
		}
		return nil, "100644", true, nil
	}
	mode := "100644"
	if m, err := git.output(w.CWD, nil, "ls-tree", "--format=%(objectmode)", w.Head, "--", path.Repo); err == nil {
		mode = strings.TrimSpace(string(m))
		if mode == "" {
			mode = "100644"
		}
	}
	return out, mode, false, nil
}

func (w *Workspace) ExistsInAdmittedView(path ResolvedPath) (bool, []byte, string, error) {
	b, mode, absent, err := w.ReadBase(path)
	if err != nil {
		return false, nil, mode, err
	}
	if !absent {
		return true, b, mode, nil
	}
	if w.Untracked {
		b, err := w.workingTreeFS().readFile(path.Abs)
		if err == nil {
			return true, b, "100644", nil
		}
		if !isNoSuch(err) {
			return false, nil, "100644", err
		}
	}
	return false, nil, mode, nil
}

func (w *Workspace) updateRef(newCommit string) error {
	if w.Unborn {
		return w.gitRunner().run(w.CWD, nil, nil, "update-ref", w.Ref, newCommit)
	}
	return w.gitRunner().run(w.CWD, nil, nil, "update-ref", w.Ref, newCommit, w.Head)
}

func (w *Workspace) pathClean(repoPath string) (bool, error) {
	out, err := w.gitRunner().output(w.CWD, nil, "status", "--porcelain=v1", "-z", "--", repoPath)
	if err != nil {
		return false, err
	}
	return len(out) == 0, nil
}

func (w *Workspace) restorePathFromCommit(commit, repoPath string) error {
	return w.gitRunner().run(w.CWD, nil, nil, "restore", "--source="+commit, "--staged", "--worktree", "--", repoPath)
}

func (w *Workspace) checkoutConversionRisk(repoPath string) (string, error) {
	var reasons []string
	git := w.gitRunner()
	out, err := git.output(w.CWD, nil, "check-attr", "-z", "filter", "working-tree-encoding", "ident", "eol", "--", repoPath)
	if err != nil {
		return "", err
	}
	fields := bytes.Split(out, []byte{0})
	for i := 0; i+2 < len(fields); i += 3 {
		attr := string(fields[i+1])
		value := string(fields[i+2])
		switch attr {
		case "filter":
			if checkoutAttrSet(value) {
				reasons = append(reasons, "filter="+value)
			}
		case "working-tree-encoding":
			if checkoutAttrSet(value) {
				reasons = append(reasons, "working-tree-encoding="+value)
			}
		case "ident":
			if checkoutAttrSet(value) {
				reasons = append(reasons, "ident="+value)
			}
		case "eol":
			if strings.EqualFold(value, "crlf") {
				reasons = append(reasons, "eol=crlf")
			}
		}
	}
	if autocrlf, err := git.output(w.CWD, nil, "config", "--bool", "--get", "core.autocrlf"); err == nil && strings.EqualFold(strings.TrimSpace(string(autocrlf)), "true") {
		reasons = append(reasons, "core.autocrlf=true")
	}
	return strings.Join(reasons, ", "), nil
}

func checkoutAttrSet(value string) bool {
	return value != "" && value != "unspecified" && value != "unset"
}

func (w *Workspace) indexBytes(repoPath string) ([]byte, bool, error) {
	out, err := w.gitRunner().output(w.CWD, nil, "show", ":"+repoPath)
	if err != nil {
		return nil, true, nil
	}
	return out, false, nil
}

func (w *Workspace) headBytes(repoPath string) ([]byte, bool, error) {
	out, err := w.gitRunner().output(w.CWD, nil, "show", "HEAD:"+repoPath)
	if err != nil {
		return nil, true, nil
	}
	return out, false, nil
}

func (w *Workspace) updateIndexBytes(repoPath, mode string, b []byte, absent bool) error {
	git := w.gitRunner()
	if absent {
		return git.run(w.CWD, nil, nil, "update-index", "--force-remove", "--", repoPath)
	}
	blob, err := git.outputStdin(w.CWD, nil, b, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	if mode == "" {
		mode = "100644"
	}
	spec := fmt.Sprintf("%s,%s,%s", mode, strings.TrimSpace(string(blob)), repoPath)
	return git.run(w.CWD, nil, nil, "update-index", "--add", "--cacheinfo", spec)
}
