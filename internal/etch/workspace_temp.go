package etch

import "os"

// workspaceTempStore is the boundary for short-lived OS allocations needed by
// Git plumbing. Planning and dry-run paths need temporary object directories
// and index files, but those allocations should be injectable and auditable
// rather than hidden inside object-store logic.
type workspaceTempStore interface {
	mkdir(pattern string) (string, error)
	create(pattern string) (workspaceTempFile, error)
	remove(path string) error
	removeAll(path string) error
}

type workspaceTempFile interface {
	Name() string
	Close() error
}

type osWorkspaceTempStore struct{}

func (w *Workspace) tempStore() workspaceTempStore {
	return w.workspaceDeps().temp
}

func (osWorkspaceTempStore) mkdir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

func (osWorkspaceTempStore) create(pattern string) (workspaceTempFile, error) {
	return os.CreateTemp("", pattern)
}

func (osWorkspaceTempStore) remove(path string) error {
	return os.Remove(path)
}

func (osWorkspaceTempStore) removeAll(path string) error {
	return os.RemoveAll(path)
}
