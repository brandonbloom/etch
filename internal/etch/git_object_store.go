package etch

import (
	"fmt"
	"path/filepath"
	"strings"
)

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
	temp := w.tempStore()
	tmp, err := temp.mkdir("etch-objects-*")
	if err != nil {
		return gitObjectStore{}, err
	}
	return gitObjectStore{
		env: []string{
			"GIT_OBJECT_DIRECTORY=" + tmp,
			"GIT_ALTERNATE_OBJECT_DIRECTORIES=" + objectDir,
		},
		cleanup: func() { _ = temp.removeAll(tmp) },
	}, nil
}

func (w *Workspace) objectDir() (string, error) {
	out, err := w.gitRunner().output(w.CWD, nil, "rev-parse", "--git-path", "objects")
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(w.CWD, path)
	}
	if real, err := w.pathResolver().evalSymlinks(path); err == nil {
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
	git := w.gitRunner()
	indexPath, cleanup, err := w.tempIndexPath()
	if err != nil {
		return "", err
	}
	defer cleanup()

	env := append([]string{}, objects.env...)
	env = append(env, "GIT_INDEX_FILE="+indexPath)
	if !w.Unborn {
		if err := git.run(w.CWD, env, nil, "read-tree", w.Head); err != nil {
			return "", err
		}
	}
	for _, key := range sortedKeys(changes) {
		ch := changes[key]
		if ch.AbsentAfter {
			if err := git.run(w.CWD, env, nil, "update-index", "--force-remove", "--", ch.RepoPath); err != nil {
				return "", err
			}
			continue
		}
		blob, err := git.outputStdin(w.CWD, env, ch.After, "hash-object", "-w", "--stdin")
		if err != nil {
			return "", err
		}
		mode := ch.Mode
		if mode == "" {
			mode = "100644"
		}
		spec := fmt.Sprintf("%s,%s,%s", mode, strings.TrimSpace(string(blob)), ch.RepoPath)
		if err := git.run(w.CWD, env, nil, "update-index", "--add", "--cacheinfo", spec); err != nil {
			return "", err
		}
	}
	tree, err := git.output(w.CWD, env, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(tree)), nil
}

func (w *Workspace) tempIndexPath() (string, func(), error) {
	temp := w.tempStore()
	tmp, err := temp.create("etch-index-*")
	if err != nil {
		return "", nil, err
	}
	indexPath := tmp.Name()
	_ = tmp.Close()
	_ = temp.remove(indexPath)
	return indexPath, func() { _ = temp.remove(indexPath) }, nil
}

func (w *Workspace) writeCommitObject(tree, message string) (string, error) {
	return w.createCommitInObjectStore(tree, message, w.repositoryObjectStore())
}

func (w *Workspace) createCommitInObjectStore(tree, message string, objects gitObjectStore) (string, error) {
	args := []string{"commit-tree", tree}
	if !w.Unborn {
		args = append(args, "-p", w.Head)
	}
	out, err := w.gitRunner().outputStdin(w.CWD, objects.env, []byte(message), args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
