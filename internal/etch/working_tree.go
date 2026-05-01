package etch

import (
	"os"
	"path/filepath"
)

// workingTreeFS is the narrow boundary for live working-tree bytes. Base
// snapshots and object writes belong to Git, but materialization has to inspect
// and update the user's live files; keeping those operations behind Workspace
// makes CWD assumptions explicit and testable.
type workingTreeFS interface {
	readFile(path string) ([]byte, error)
	writeFile(path string, b []byte, perm os.FileMode) error
	mkdirAll(path string, perm os.FileMode) error
	remove(path string) error
}

type osWorkingTreeFS struct{}

func (w *Workspace) workingTreeFS() workingTreeFS {
	if w.worktree != nil {
		return w.worktree
	}
	return osWorkingTreeFS{}
}

func (w *Workspace) readWorktreeFile(ch fileChange) ([]byte, bool, error) {
	b, err := w.workingTreeFS().readFile(w.worktreePath(ch))
	if err == nil {
		return b, false, nil
	}
	if isNoSuch(err) {
		return nil, true, nil
	}
	return nil, true, err
}

func (w *Workspace) worktreePath(ch fileChange) string {
	if ch.AbsPath != "" {
		return ch.AbsPath
	}
	return filepath.Join(w.CWD, filepath.FromSlash(ch.Path))
}

func (w *Workspace) writeWorktree(ch fileChange, b []byte, absent bool) error {
	path := w.worktreePath(ch)
	fs := w.workingTreeFS()
	if absent {
		if err := fs.remove(path); err != nil && !isNoSuch(err) {
			return err
		}
		return nil
	}
	if err := fs.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fs.writeFile(path, b, 0o644)
}

func (osWorkingTreeFS) readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (osWorkingTreeFS) writeFile(path string, b []byte, perm os.FileMode) error {
	return os.WriteFile(path, b, perm)
}

func (osWorkingTreeFS) mkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osWorkingTreeFS) remove(path string) error {
	return os.Remove(path)
}
