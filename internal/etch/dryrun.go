package etch

import (
	"fmt"
	"strings"
)

func RenderDryRun(w *Workspace, plan *Plan) (string, error) {
	commit, err := w.createCommit(plan.Tree, plan.Commit.Message)
	if err != nil {
		return "", err
	}
	author, err := gitOutput(w.CWD, nil, "show", "-s", "--format=%aN <%aE>", commit)
	if err != nil {
		return "", err
	}
	date, err := gitOutput(w.CWD, nil, "show", "-s", "--format=%aD", commit)
	if err != nil {
		return "", err
	}
	diff, err := gitOutput(w.CWD, nil, "show", "--format=", "--stat", "--patch", "--binary", commit)
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
