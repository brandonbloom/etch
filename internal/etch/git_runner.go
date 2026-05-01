package etch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitRunner is the narrow boundary for invoking Git plumbing. Workspace logic
// depends on this interface so tests can inject Git behavior without creating a
// repository, while production keeps the real shell-out implementation here.
type gitRunner interface {
	output(dir string, env []string, args ...string) ([]byte, error)
	run(dir string, env []string, stdin []byte, args ...string) error
	outputStdin(dir string, env []string, stdin []byte, args ...string) ([]byte, error)
}

type realGitRunner struct{}

func (w *Workspace) gitRunner() gitRunner {
	if w.git != nil {
		return w.git
	}
	return realGitRunner{}
}

func (realGitRunner) output(dir string, env []string, args ...string) ([]byte, error) {
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

func (realGitRunner) run(dir string, env []string, stdin []byte, args ...string) error {
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

func (realGitRunner) outputStdin(dir string, env []string, stdin []byte, args ...string) ([]byte, error) {
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
