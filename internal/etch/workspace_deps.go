package etch

// workspaceDeps is the private bundle of injectable Workspace boundaries. Tests
// can replace a specific boundary, while production constructors leave the
// bundle empty and receive OS/Git backed defaults.
type workspaceDeps struct {
	git      gitRunner
	worktree workingTreeFS
	temp     workspaceTempStore
	paths    workspacePaths
}

func (d workspaceDeps) withDefaults() workspaceDeps {
	if d.git == nil {
		d.git = realGitRunner{}
	}
	if d.worktree == nil {
		d.worktree = osWorkingTreeFS{}
	}
	if d.temp == nil {
		d.temp = osWorkspaceTempStore{}
	}
	if d.paths == nil {
		d.paths = osWorkspacePaths{}
	}
	return d
}

func (w *Workspace) workspaceDeps() workspaceDeps {
	return w.deps.withDefaults()
}
