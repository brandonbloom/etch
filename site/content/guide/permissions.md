---
title: Setting up permissions
description: How to allow-list etch in Claude Code and other agent frameworks.
weight: 4
---

etch is designed to be a safe, narrow permission target. When you allow-list etch, you're granting access to a tool that:

- Makes **no network calls** --- no HTTP, no DNS, no sockets
- Spawns **no processes** except git (for reading/writing objects and refs)
- Stays **under CWD** --- rejects absolute paths, `..` segments, `.git` paths, and symlink escapes
- Has **no scripting escape hatches** --- no variables, no expansion, no subshells

This means allowing etch is not equivalent to "always allow shell." The blast radius is statically bounded by the command text.

## Claude Code

Add etch to your project's `.claude/settings.json`:

```json
{
  "permissions": {
    "allow": [
      "Bash(etch *)"
    ]
  }
}
```

This allows any etch command without prompting. The agent can freely mutate files in the repo via etch while still being prompted for other shell commands.

## Other agent frameworks

The general pattern is the same: add `etch` to your tool allow-list or permission grants. Since etch's only side effects are git commits to the local repo, the permission surface is well-defined.

For frameworks that support plan-based authorization:

```sh
# Compute a plan (no side effects)
etch --plan set tasks/review.md status complete

# The plan includes SHA-256 hashes of all inputs and outputs.
# A host runtime can present this to a human for approval
# and cache that approval by plan hash.
```

If anything changes after approval --- file contents, branch state, even etch's own behavior across a version upgrade --- the recomputed hash changes, invalidating the cached approval.

## Concurrency

Multiple agents (or humans) can work on the same repo simultaneously. etch handles this with optimistic concurrency:

- Each invocation reads from committed HEAD state, not the working tree
- Commits use compare-and-swap on the branch ref
- On conflict, etch automatically re-plans and retries (up to `--retries`, default 3)
- The caller never sees the retry unless the budget is exhausted

This means you don't need locking or coordination between agents. They can work in parallel on the same repo, and etch handles the serialization.
