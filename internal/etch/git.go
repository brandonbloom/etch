package etch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Workspace struct {
	CWD       string
	Root      string
	Head      string
	Ref       string
	Unborn    bool
	Untracked bool
}

// gitObjectStore names where object-writing Git plumbing should put new
// objects. Preview paths use a temporary store so they can still ask Git for
// exact blob/tree/commit OIDs without leaving dangling objects in the repo;
// execution paths use the repository store at the explicit persistence boundary.
type gitObjectStore struct {
	env     []string
	cleanup func()
}

func (s gitObjectStore) close() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

// OpenWorkspace opens the process working directory. Prefer OpenWorkspaceAt
// when the caller already has an explicit directory.
func OpenWorkspace(untracked bool) (*Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return OpenWorkspaceAt(cwd, untracked)
}

func OpenWorkspaceAt(cwd string, untracked bool) (*Workspace, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	cwd = abs
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	} else {
		return nil, failf("cannot resolve working directory %s: %v", cwd, err)
	}
	rootBytes, err := gitOutput(cwd, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, failf("not inside a git worktree")
	}
	root := strings.TrimSpace(string(rootBytes))
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	headBytes, err := gitOutput(cwd, nil, "rev-parse", "--verify", "HEAD")
	unborn := false
	head := ""
	if err != nil {
		unborn = true
	} else {
		head = strings.TrimSpace(string(headBytes))
	}
	refBytes, err := gitOutput(cwd, nil, "symbolic-ref", "-q", "HEAD")
	ref := "HEAD"
	if err == nil {
		ref = strings.TrimSpace(string(refBytes))
	}
	return &Workspace{CWD: cwd, Root: root, Head: head, Ref: ref, Unborn: unborn, Untracked: untracked}, nil
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
	if finalSymlink && !mayBeMissing {
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			checkAbs = real
			abs = real
		} else if !isNoSuch(err) {
			return ResolvedPath{}, failf("%s: %v", input, err)
		}
	} else if real, err := filepath.EvalSymlinks(checkAbs); err == nil {
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

func (w *Workspace) ReadBase(path ResolvedPath) ([]byte, string, bool) {
	if w.Unborn {
		return nil, "100644", true
	}
	out, err := gitOutput(w.CWD, nil, "show", w.Head+":"+path.Repo)
	if err != nil {
		if w.Untracked {
			if b, ok, _ := readUntrackedFile(path.Abs); ok {
				return b, "100644", false
			}
		}
		return nil, "100644", true
	}
	mode := "100644"
	if m, err := gitOutput(w.CWD, nil, "ls-tree", "--format=%(objectmode)", w.Head, "--", path.Repo); err == nil {
		mode = strings.TrimSpace(string(m))
		if mode == "" {
			mode = "100644"
		}
	}
	return out, mode, false
}

func (w *Workspace) ExistsInAdmittedView(path ResolvedPath) (bool, []byte, string, error) {
	b, mode, absent := w.ReadBase(path)
	if !absent {
		return true, b, mode, nil
	}
	if w.Untracked {
		if _, _, err := readUntrackedFile(path.Abs); err != nil {
			return false, nil, "100644", err
		}
	}
	return false, nil, mode, nil
}

func readUntrackedFile(path string) ([]byte, bool, error) {
	b, err := os.ReadFile(path)
	if err == nil {
		return b, true, nil
	}
	if isNoSuch(err) {
		return nil, false, nil
	}
	return nil, false, err
}

func gitOutput(dir string, env []string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return out, fmt.Errorf("%s", msg)
		}
		return out, err
	}
	return out, nil
}

func gitRun(dir string, env []string, stdin []byte, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

func gitOutputStdin(dir string, env []string, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return out, fmt.Errorf("%s", msg)
		}
		return out, err
	}
	return out, nil
}

func (w *Workspace) repositoryObjectStore() gitObjectStore {
	return gitObjectStore{}
}

// ephemeralObjectStore lets Git read existing repository objects through
// alternates while writing all new objects into a throwaway directory. Git OIDs
// are content-addressed, so the resulting planned tree ID is the same one the
// repository store will produce later if the invocation is committed.
func (w *Workspace) ephemeralObjectStore() (gitObjectStore, error) {
	objectDir, err := w.objectDir()
	if err != nil {
		return gitObjectStore{}, err
	}
	tmp, err := os.MkdirTemp("", "etch-objects-*")
	if err != nil {
		return gitObjectStore{}, err
	}
	return gitObjectStore{
		env: []string{
			"GIT_OBJECT_DIRECTORY=" + tmp,
			"GIT_ALTERNATE_OBJECT_DIRECTORIES=" + objectDir,
		},
		cleanup: func() { _ = os.RemoveAll(tmp) },
	}, nil
}

func (w *Workspace) objectDir() (string, error) {
	out, err := gitOutput(w.CWD, nil, "rev-parse", "--git-path", "objects")
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(w.CWD, path)
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	return path, nil
}

// computePlannedTreeOID returns the tree object ID for a plan without
// populating the repository object database.
func (w *Workspace) computePlannedTreeOID(changes map[string]fileChange) (string, error) {
	objects, err := w.ephemeralObjectStore()
	if err != nil {
		return "", err
	}
	defer objects.close()
	return w.buildTreeInObjectStore(changes, objects)
}

// writePlannedTree is the execution boundary where planned file bytes become
// persistent repository objects.
func (w *Workspace) writePlannedTree(plan *Plan) (string, error) {
	tree, err := w.buildTreeInObjectStore(plan.Touched, w.repositoryObjectStore())
	if err != nil {
		return "", err
	}
	if plan.Tree != "" && tree != plan.Tree {
		return "", failf("planned tree changed while writing objects: got %s, want %s", tree, plan.Tree)
	}
	return tree, nil
}

func (w *Workspace) buildTreeInObjectStore(changes map[string]fileChange, objects gitObjectStore) (string, error) {
	tmp, err := os.CreateTemp("", "etch-index-*")
	if err != nil {
		return "", err
	}
	indexPath := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(indexPath)
	defer os.Remove(indexPath)
	env := append([]string{}, objects.env...)
	env = append(env, "GIT_INDEX_FILE="+indexPath)
	if !w.Unborn {
		if err := gitRun(w.CWD, env, nil, "read-tree", w.Head); err != nil {
			return "", err
		}
	}
	for _, key := range sortedKeys(changes) {
		ch := changes[key]
		if ch.AbsentAfter {
			if err := gitRun(w.CWD, env, nil, "update-index", "--force-remove", "--", ch.RepoPath); err != nil {
				return "", err
			}
			continue
		}
		blob, err := gitOutputStdin(w.CWD, env, ch.After, "hash-object", "-w", "--stdin")
		if err != nil {
			return "", err
		}
		mode := ch.Mode
		if mode == "" {
			mode = "100644"
		}
		spec := fmt.Sprintf("%s,%s,%s", mode, strings.TrimSpace(string(blob)), ch.RepoPath)
		if err := gitRun(w.CWD, env, nil, "update-index", "--add", "--cacheinfo", spec); err != nil {
			return "", err
		}
	}
	tree, err := gitOutput(w.CWD, env, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(tree)), nil
}

func (w *Workspace) writeCommitObject(tree, message string) (string, error) {
	return w.createCommitInObjectStore(tree, message, w.repositoryObjectStore())
}

func (w *Workspace) createCommitInObjectStore(tree, message string, objects gitObjectStore) (string, error) {
	args := []string{"commit-tree", tree}
	if !w.Unborn {
		args = append(args, "-p", w.Head)
	}
	out, err := gitOutputStdin(w.CWD, objects.env, []byte(message), args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (w *Workspace) updateRef(newCommit string) error {
	if w.Unborn {
		return gitRun(w.CWD, nil, nil, "update-ref", w.Ref, newCommit)
	}
	return gitRun(w.CWD, nil, nil, "update-ref", w.Ref, newCommit, w.Head)
}

func (w *Workspace) pathClean(repoPath string) (bool, error) {
	out, err := gitOutput(w.CWD, nil, "status", "--porcelain=v1", "-z", "--", repoPath)
	if err != nil {
		return false, err
	}
	return len(out) == 0, nil
}

func (w *Workspace) restorePathFromCommit(commit, repoPath string) error {
	return gitRun(w.CWD, nil, nil, "restore", "--source="+commit, "--staged", "--worktree", "--", repoPath)
}

func (w *Workspace) checkoutConversionRisk(repoPath string) (string, error) {
	var reasons []string
	out, err := gitOutput(w.CWD, nil, "check-attr", "-z", "filter", "working-tree-encoding", "ident", "eol", "--", repoPath)
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
	if autocrlf, err := gitOutput(w.CWD, nil, "config", "--bool", "--get", "core.autocrlf"); err == nil && strings.EqualFold(strings.TrimSpace(string(autocrlf)), "true") {
		reasons = append(reasons, "core.autocrlf=true")
	}
	return strings.Join(reasons, ", "), nil
}

func checkoutAttrSet(value string) bool {
	return value != "" && value != "unspecified" && value != "unset"
}

func (w *Workspace) indexBytes(repoPath string) ([]byte, bool, error) {
	out, err := gitOutput(w.CWD, nil, "show", ":"+repoPath)
	if err != nil {
		return nil, true, nil
	}
	return out, false, nil
}

func (w *Workspace) headBytes(repoPath string) ([]byte, bool, error) {
	out, err := gitOutput(w.CWD, nil, "show", "HEAD:"+repoPath)
	if err != nil {
		return nil, true, nil
	}
	return out, false, nil
}

func (w *Workspace) updateIndexBytes(repoPath, mode string, b []byte, absent bool) error {
	if absent {
		return gitRun(w.CWD, nil, nil, "update-index", "--force-remove", "--", repoPath)
	}
	blob, err := gitOutputStdin(w.CWD, nil, b, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	if mode == "" {
		mode = "100644"
	}
	spec := fmt.Sprintf("%s,%s,%s", mode, strings.TrimSpace(string(blob)), repoPath)
	return gitRun(w.CWD, nil, nil, "update-index", "--add", "--cacheinfo", spec)
}
