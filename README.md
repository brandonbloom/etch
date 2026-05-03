# etch

`etch` is a small CLI for precise, mechanical edits to structured files in a
Git worktree. Each successful mutating invocation is planned from `HEAD` and
recorded as a Git commit, so the repository history becomes the transaction log
for file changes such as "set this JSON field", "replace this Markdown section",
or "append this JSONL event".

The project is aimed at agentic coding workflows and wiki-style repositories
where many edits are simple but easy to get subtly wrong with hand-authored
patches. `etch` trades generality for a narrow set of format-aware operations
that are atomic, reviewable, and replayable.

## What etch does

- Mutates JSON, JSONL/NDJSON, YAML, Markdown frontmatter, Markdown sections,
  CSV tables, Markdown pipe tables, and plain files.
- Commits each successful mutating invocation as one Git commit.
- Treats multi-operation scripts as one transaction: all operations commit
  together, or none of them do.
- Reads tracked inputs from `HEAD`, not from dirty checkout files.
- Materializes committed changes back into the index and working tree, merging
  around local checkout edits when possible.
- Emits a canonical JSON plan with `--plan`.
- Emits a `git am` compatible patch preview with `--dry-run` or `-n`.
- Restricts path operands to relative paths under the process CWD.

See [spec.md](spec.md) for the full design record.

## Install

```sh
go install github.com/brandonbloom/etch/cmd/etch@latest
```

Requires Go 1.25.3+ and Git.

## Build

Requirements:

- Go 1.25.3
- Git
- Mise, optional but recommended for the project environment

Build both binaries into `./bin`:

```sh
mise run build
```

The equivalent Go command is:

```sh
mkdir -p ./bin
go build -o ./bin/ ./cmd/...
```

This produces:

- `./bin/etch`: the CLI
- `./bin/etch-validate`: a validation harness for representative workflows

## Quick Start

Run `etch` from inside a Git worktree. The CWD is the capability boundary for
path operands.

```sh
printf '{"status":"open"}\n' > state.json
git add state.json
git commit -m 'add state'

etch set state.json status complete
git show --stat --oneline HEAD
```

The `set` command rewrites `state.json`, creates a commit, and materializes the
new bytes into the checkout.

Preview the same operation without side effects:

```sh
etch --plan set state.json status complete
etch --dry-run set state.json status complete
```

## Core Commands

Porcelain commands infer the format from the file extension:

| Command | Description |
| --- | --- |
| `set <path> <selector> <value>` | Set one JSON, YAML, or Markdown frontmatter value. |
| `set <path> <selector=value>...` | Set multiple structured values in one file. |
| `delete <path> [<selector>]` | Delete a file or selected structured value. |
| `append <path> <selector> <value>` | Append a value to an array. |
| `append <path.jsonl> <json-value>` | Append one compact JSON record to a JSONL or NDJSON log. |
| `add <path> <selector> <value>` | Ensure an array contains a value. |
| `remove <path> <selector> <value>` | Ensure an array does not contain a value. |
| `section replace <path> <heading> <content>` | Replace the body under one Markdown heading. |
| `section append <path> <heading> <content>` | Append a block fragment under one Markdown heading. |
| `section prepend <path> <heading> <content>` | Prepend a block fragment under one Markdown heading. |
| `task close <path> <text>` | Ensure a Markdown task is closed. |
| `task open <path> <text>` | Ensure a Markdown task is open, creating it when a destination is addressed. |
| `list add <path> <text>` | Add one Markdown list item. |
| `task add <path> <text>` | Add one open Markdown task. |
| `create <path> [<content>]` | Create a file with explicit or extension-aware default content. |
| `replace <path> <content>` | Replace an existing file's entire content. |
| `move <src> <dst>` | Move a file path. |
| `copy <src> <dst>` | Copy a file path. |
| `exists <path>` | Guard that a path exists in the admitted input view. |
| `missing <path>` | Guard that a path is missing in the admitted input view. |
| `contains <path> <literal>` | Guard that file bytes contain a literal. |

Table commands infer CSV or Markdown table behavior:

| Command | Description |
| --- | --- |
| `table set <path> ... <range> <value>` | Set one or more table cells. |
| `table row append <path> ... <row-json>` | Append a row. |
| `table row insert <path> ... (--before <row>\|--after <row>) <row-json>` | Insert a row. |
| `table row delete <path> ... <row>` | Delete rows. |
| `table column add <path> ... <column> [--after <column>] [--default <value>]` | Add a column. |
| `table column rename <path> ... <old-column> <new-column>` | Rename a column. |
| `table column delete <path> ... <column>` | Delete a column. |

Use `etch help --all` to see format-explicit plumbing commands such as
`json set`, `jsonl append`, `yaml set`, `frontmatter set`,
`md section replace`, and `csv row append`.

For machine-readable command metadata:

```sh
etch verbs --json
```

## Examples

Set JSON:

```sh
etch set state.json status complete
etch set state.json priority --json 1
etch set state.json labels --json '["agent","docs"]'
etch set state.json status=complete priority:=1
```

Append JSONL/NDJSON:

```sh
etch append events.jsonl '{"kind":"prompt","at":"2026-05-02T09:00:00-07:00"}'
etch jsonl append events.log '{"kind":"heartbeat"}'
```

Mutate Markdown frontmatter:

```sh
etch set posts/hello.md title "Hello, world"
etch add posts/hello.md tags draft
etch delete posts/hello.md draft
```

Mutate Dataview-style Markdown inline fields:

```sh
etch set tasks/follow-up.md done "2026-05-02" --task "Send follow-up"
etch set journal/today.md heartbeat "ok" --section "## Status" --tail
etch delete tasks/follow-up.md snooze --task "Send follow-up"
```

Mutate Markdown tasks and lists:

```sh
etch task close tasks/follow-up.md "Send follow-up" --section "## Actions"
etch task open tasks/follow-up.md "Review draft" --section "## Actions"
etch list add tasks/follow-up.md "Capture launch note" --section "## Notes"
etch task add tasks/follow-up.md "Send update" --section "## Actions"
```

Mutate a Markdown section:

```sh
etch section replace posts/hello.md "## Summary" <<'EOF'
This post introduces the project and its goals.
EOF

etch section append posts/hello.md Summary <<'EOF'
Follow-up note.
EOF
```

Mutate a CSV table:

```sh
etch table set data.csv all,status done
etch table row append data.csv '{"id":"1","status":"open"}'
etch table column add data.csv owner --default Brandon
```

Mutate a Markdown pipe table:

```sh
etch table set notes.md doc @0 all,status done
etch table row append notes.md "## Inventory" '{"sku":"A1","status":"open"}'
```

Guard a transaction:

```sh
etch run ops.etch
```

```text
contains state.json open
set state.json status complete
section replace README.md "## Status" <<EOF
Complete.
EOF
```

If any guard or mutation fails, the script produces no commit.

## Scripts

`etch run <script>` executes a file containing one command per line. `etch run`
without a script path reads from stdin.

Script lines use the same token sequence as CLI arguments after the `etch`
binary name. Blank lines and `#` comments are ignored. Quoting supports single
quotes, double quotes, and backslash escaping. There are no shell expansions:
`$FOO` is literal text.

Multi-line values use heredocs:

```text
set posts/hello.md title "Hello, world"
section replace posts/hello.md "## Summary" <<EOF
This is literal text.
$FOO is not expanded.
EOF
```

Parse errors include script path and line number.

## Selectors and Values

Structured selectors are singular JSONPath-style paths:

```text
$.agents.assistant.last_run
agents.assistant.last_run
$.items[0].title
$["key.with.dots"]
```

Unsupported selector forms include wildcards, recursive descent, slices,
filters, unions, functions, and negative indexes.

Structured values are strings by default. Use `--json` to parse one following
token as a strict JSON value.

```text
etch set state.json status complete
etch set state.json count --json 12
etch append state.json events --json '{"status":"done"}'
etch append events.jsonl '{"status":"done"}'
```

`set` also accepts assignment items: `selector=value` writes a string and
`selector:=json` writes strict JSON. Assignment items are not accepted by
`append`, `add`, `remove`, or `delete`.

JSONL and NDJSON append values are always strict JSON and do not use `--json`;
missing JSONL targets are created as empty logs before appending.

For Markdown paths, structured selectors target YAML frontmatter by default.
Markdown address flags such as `--body`, `--section`, and `--task` switch
`set` and `delete` to Dataview-style inline fields in the Markdown body.
Task/list commands use `--section`, `--before`, and `--after` to address where
task and list mutations happen; `--before` and `--after` match list items, not
arbitrary prose.

## Transaction Model

Mutating invocations plan changes from the base commit at `HEAD`. Dirty
working-tree files are not swept into the commit. After the commit lands, `etch`
updates the index and working tree for the touched paths.

If local checkout edits conflict with the committed change during
materialization, `etch` leaves conflict markers in the affected paths and reports
the recovery steps on stderr. The commit remains durable after the ref update
succeeds.

Useful flags:

| Flag | Effect |
| --- | --- |
| `--plan` | Print the canonical JSON plan and skip execution. |
| `-n`, `--dry-run` | Print a `git am` compatible patch preview and skip execution. |
| `--no-checkout` | Commit without materializing touched paths into the checkout. |
| `--untracked` | Admit untracked source paths under CWD. |
| `--message <m>` | Override the generated commit message. |
| `--subject-prefix <s>` | Prepend literal text to the generated commit subject. |
| `--subject-suffix <s>` | Append literal text to the generated commit subject. |
| `--body-prefix <s>` | Prepend a body block before the generated commit body. |
| `--body-suffix <s>` | Append a body block after the generated commit body. |
| `--retries <n>` | Retry optimistic ref-update conflicts. The default is `3`. |
| `--allow-empty` | Permit an empty commit for mutating invocations. |

## Safety Model

`etch` accepts only relative paths under CWD. It rejects absolute paths, `..`
segments, `.git` path segments, and symlink escapes. Mutating invocations require
a Git worktree. The implementation invokes Git for repository, object, and ref
operations.

Supported structured formats are UTF-8 text. JSON, JSONL/NDJSON, YAML,
Markdown, and CSV inputs may include a UTF-8 BOM; `etch` preserves it when
writing the file back.

## Development

Run tests:

```sh
go test ./...
```

Run the validation harness:

```sh
go run ./cmd/etch-validate
```

Project layout:

| Path | Purpose |
| --- | --- |
| `cmd/etch` | CLI entrypoint. |
| `cmd/etch-validate` | Validation harness over fixture repositories. |
| `internal/etch` | Parser, catalog, planner, Git backend, materializer, and format evaluators. |
| `spec.md` | Design and behavior specification. |
| `docs/` | Explainer site, deployed to GitHub Pages. |
| `mise.toml` | Project environment and build task. |

Useful local commands:

```sh
./bin/etch help
./bin/etch help selectors
./bin/etch help values
./bin/etch help plans
./bin/etch help conflicts
./bin/etch help table
```
