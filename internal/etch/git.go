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

func OpenWorkspace(untracked bool) (*Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
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
	out, err := gitOutput(w.CWD, nil, "show", "HEAD:"+path.Repo)
	if err != nil {
		if w.Untracked {
			if b, ok, _ := readUntrackedFile(path.Abs); ok {
				return b, "100644", false
			}
		}
		return nil, "100644", true
	}
	mode := "100644"
	if m, err := gitOutput(w.CWD, nil, "ls-tree", "--format=%(objectmode)", "HEAD", "--", path.Repo); err == nil {
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

func (w *Workspace) buildTree(changes map[string]fileChange) (string, error) {
	tmp, err := os.CreateTemp("", "etch-index-*")
	if err != nil {
		return "", err
	}
	indexPath := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(indexPath)
	defer os.Remove(indexPath)
	env := []string{"GIT_INDEX_FILE=" + indexPath}
	if !w.Unborn {
		if err := gitRun(w.CWD, env, nil, "read-tree", "HEAD"); err != nil {
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
		blob, err := gitOutputStdin(w.CWD, nil, ch.After, "hash-object", "-w", "--stdin")
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

func (w *Workspace) createCommit(tree, message string) (string, error) {
	args := []string{"commit-tree", tree}
	if !w.Unborn {
		args = append(args, "-p", w.Head)
	}
	out, err := gitOutputStdin(w.CWD, nil, []byte(message), args...)
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
