---
status: draft
depends_on: []
---

# Query Commands

## Summary

Add a read-side command for printing selected values without creating commits.
The first version should mirror Etch's existing selectors and input-admission
rules, while staying clearly separate from mutating scripts.

## Candidate Commands

```sh
etch get <path> <selector>
etch get <path.md> <selector>
etch get <path.md> <field> --body|--section <heading>|--item <text>|--task <text>
```

Examples:

```sh
etch get state.json status
etch get config.yaml agents.assistant.last_run
etch get note.md status
etch get note.md done --task "Send follow-up"
```

## Semantics

- `get` reads from the same admitted input view as mutations: tracked files are
  read from `HEAD`; `--untracked` admits checkout-only files under CWD.
- Porcelain format inference matches mutating commands.
- Structured JSON, YAML, and frontmatter values print compact JSON to stdout.
- String values print as JSON strings by default so scripts receive one
  unambiguous representation.
- A future `--raw` flag may print scalar strings without JSON quoting.
- Missing values exit 1 with a user-facing error rather than printing `null`.
- `get` creates no commit and does not materialize files into the checkout.
- `get` is not accepted inside mutating `run` scripts until mixed read/write
  script semantics are designed.
- `--plan`, `--dry-run`, commit-message flags, and `--allow-empty` do not apply
  to `get` in the first version.

## Rationale

Several adopter scripts want to branch in the host language based on repository
state before deciding which mutation to run. Today those scripts must use
general file reads and format-specific parsing outside Etch. A narrow query
surface keeps the same selector and format rules as mutation commands without
turning guards into a reporting API.

## Impact

Spec:

- Add a non-mutating command class or explicitly document query commands as
  outside the mutating/guard classes.
- Define stdout encoding, missing-value behavior, and unsupported global flags.
- Decide whether query commands are excluded from `run`.

Docs:

- Add `etch help queries` or extend the model/help topic.
- Add examples for JSON/YAML/frontmatter and Markdown inline fields.

Code:

- Add query planning/evaluation path that reads admitted input without creating
  a commit.
- Reuse selector normalization and Markdown address decoding.
- Add tests for stdout, missing values, untracked input, dirty checkout
  behavior, unsupported flags, and no side effects.

## Open Questions

- Should the command be named `get`, `read`, or should one alias the other?
- Should query commands read from `HEAD` only, or should an explicit
  `--worktree` mode exist?
- Should Markdown section and table reads be part of the first query surface?
- Should stdout always be newline-terminated, including for compact JSON?
- Should missing values have a distinct exit code from parse/usage errors?
