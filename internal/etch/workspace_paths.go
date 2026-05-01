package etch

import (
	"os"
	"path/filepath"
)

// workspacePaths is the boundary for process-dependent path resolution. The
// rest of Workspace can still use filepath's pure lexical operations directly,
// but current-directory and symlink resolution go through this interface.
type workspacePaths interface {
	getwd() (string, error)
	abs(path string) (string, error)
	evalSymlinks(path string) (string, error)
}

type osWorkspacePaths struct{}

func (w *Workspace) pathResolver() workspacePaths {
	return w.workspaceDeps().paths
}

func (osWorkspacePaths) getwd() (string, error) {
	return os.Getwd()
}

func (osWorkspacePaths) abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (osWorkspacePaths) evalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
