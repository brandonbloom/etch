---
title: Getting started
description: Install etch, run your first commands, understand the mental model.
weight: 2
---

## Install

```sh
go install github.com/brandonbloom/etch/cmd/etch@latest
```

Requires Go 1.25.3+ and Git.

## First commands

Run etch from inside a Git worktree. Create a file, commit it, then use etch to mutate it:

```sh
printf '{"status":"open"}\n' > state.json
git add state.json
git commit -m 'add state'

etch set state.json status complete
```

That `set` command rewrites `state.json`, creates a commit, and materializes the new bytes into your checkout. Check the result:

```sh
git show --stat --oneline HEAD
cat state.json
```

## Preview without side effects

```sh
# Canonical JSON plan
etch --plan set state.json status complete

# git-am compatible patch
etch --dry-run set state.json status complete
```

## Scripts

Multiple operations can be grouped into an atomic transaction. All operations commit together, or none of them do:

```sh
etch run - <<'SCRIPT'
set tasks/review.md snooze "2026-05-10"
set tasks/review.md status deferred
append people/alice.md tags '"pending-review"'
SCRIPT
```

Script lines use the same syntax as CLI arguments. Blank lines and `#` comments are ignored. There are no shell expansions: `$FOO` is literal text.

## Mental model

Every mutating etch invocation follows five stages:

1. **Parse** --- Tokenize input using shell quoting rules. No expansion of any kind.
2. **Plan** --- Read target files from HEAD (never the working tree). Compute before/after state.
3. **Build** --- Write to an isolated temporary git index. Never touches your staging area.
4. **Commit** --- Compare-and-swap on the branch ref. If another writer moved HEAD, retry automatically (up to `--retries`, default 3).
5. **Materialize** --- Write touched paths into the working tree and index.

Read operations (`exists`, `missing`, `contains`) never commit.

## Built-in help

etch has extensive built-in documentation:

```sh
etch help              # Porcelain commands
etch help --all        # All commands including plumbing
etch help model        # Mental model and transaction semantics
etch help selectors    # JSONPath selector syntax
etch help values       # String vs JSON value modes
etch help fields       # Markdown fields (frontmatter and inline)
etch help addressing   # Markdown section/task/item addressing
etch help scripts      # Batch script documentation
etch help plans        # --plan and --dry-run modes
etch help security     # Safety guarantees
etch help conflicts    # Merge conflict handling
```
