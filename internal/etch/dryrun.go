package etch

import (
	"fmt"
	"strings"
)

func RenderDryRun(w *Workspace, plan *Plan) (string, error) {
	git := w.gitRunner()
	// Native Git already knows how to render the mailbox patch shape we want,
	// but it needs tree and commit objects to diff. Keep those objects
	// ephemeral so --dry-run remains a repository-no-write preview.
	objects, err := w.ephemeralObjectStore()
	if err != nil {
		return "", err
	}
	defer objects.close()
	tree, err := w.buildTreeInObjectStore(plan.Touched, objects)
	if err != nil {
		return "", err
	}
	if tree != plan.Tree {
		return "", failf("planned tree changed while rendering dry-run: got %s, want %s", tree, plan.Tree)
	}
	commit, err := w.createCommitInObjectStore(tree, plan.Commit.Message, objects)
	if err != nil {
		return "", err
	}
	author, err := git.output(w.CWD, objects.env, "show", "-s", "--format=%aN <%aE>", commit)
	if err != nil {
		return "", err
	}
	date, err := git.output(w.CWD, objects.env, "show", "-s", "--format=%aD", commit)
	if err != nil {
		return "", err
	}
	diff, err := git.output(w.CWD, objects.env, "show", "--format=", "--stat", "--patch", "--binary", commit)
	if err != nil {
		return "", err
	}
	subject, body, _ := strings.Cut(plan.Commit.Message, "\n\n")
	var b strings.Builder
	b.WriteString("From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001\n")
	fmt.Fprintf(&b, "From: %s", strings.TrimSpace(string(author)))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "Date: %s", strings.TrimSpace(string(date)))
	b.WriteByte('\n')
	fmt.Fprintf(&b, "Subject: %s\n", subject)
	fmt.Fprintf(&b, "Etch-Plan-Hash: %s\n", plan.Hash)
	fmt.Fprintf(&b, "Etch-Base-Commit: %s\n", plan.BaseCommit)
	fmt.Fprintf(&b, "Etch-Tree: %s\n", plan.Tree)
	b.WriteByte('\n')
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	b.WriteString("---\n")
	b.WriteString(strings.TrimLeft(string(diff), "\n"))
	if !strings.HasSuffix(b.String(), "\n") {
		b.WriteByte('\n')
	}
	return b.String(), nil
}
