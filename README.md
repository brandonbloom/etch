# etch

`etch` turns small structured file changes into one command and one Git commit.

It is built for agentic coding workflows and knowledge repos where mechanical
edits are too slow and token-hungry as tool choreography: set this JSON field,
close that Markdown task, append this JSONL event, replace that section, update
this table.

Instead of asking an agent to inspect a file, construct a patch, re-check the
result, inspect Git state, stage, and commit, `etch` makes the edit itself the
transaction.

## Features

- Applies structural operations to JSON, JSONL/NDJSON, YAML, Markdown
  frontmatter, Markdown sections, Markdown tasks and lists, CSV tables,
  Markdown pipe tables, and plain files.
- Records each successful mutation as a Git commit, making repository history
  the transaction log.
- Supports concurrent humans, agents, and subagents in one repo: invocations
  plan from `HEAD`, commit atomically, and retry when another writer moves the
  branch.
- Treats multi-operation scripts as one transaction: all operations commit
  together, or none of them do.
- Fits live human and agent worktrees by materializing committed changes back
  into the index and working tree, merging around local checkout edits when
  possible.
- Supports review before execution: `--plan` emits the canonical transaction
  plan, and `--dry-run` emits the patch preview without changing files.
- Stays bounded to the caller's current working directory, which makes it
  practical to allow-list for agent use.

## Install

```sh
GOEXPERIMENT=jsonv2 go install github.com/brandonbloom/etch/cmd/etch@latest
```

Requires Go 1.26.2+ with `GOEXPERIMENT=jsonv2`, plus Git.

## Try it

Run `etch` from inside a Git worktree.

```sh
git init

etch --dry-run set state.json status open
etch set state.json status open
git show --stat --oneline HEAD
```

The `set` command creates `state.json`, creates a commit, and materializes the
new file into the checkout.

## Documentation

- [Overview](https://brandonbloom.github.io/etch/)
- [Quickstart](https://brandonbloom.github.io/etch/quickstart/)
- [Reference](https://brandonbloom.github.io/etch/reference/)

The CLI also includes reference help:

```sh
etch help
etch help <topic>
etch help --all
```

Useful starting topics include `model`, `invocation`, `values`, `formats`,
`addressing`, `fields`, `scripts`, `plans`, and `conflicts`.

For the design record, see [spec.md](spec.md). For proposal history, see
[docs/proposals](docs/proposals/).

## Development

See [DEVELOPING.md](DEVELOPING.md) for contributor workflow notes, including
local build, test, and website commands.

## License

`etch` is open-source software released under the [MIT License](LICENSE).
