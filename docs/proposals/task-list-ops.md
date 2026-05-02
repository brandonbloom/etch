---
status: draft
depends_on:
  - markdown-addressing
  - section-insertion
  - inline-fields
---

# Markdown Task and List Operations

## Summary

Add a narrow Markdown task/list layer for checkbox toggles and bullet appends.
This proposal is deliberately narrower than generic Markdown list editing.

## Dataview Background

Dataview indexes tasks and list items. It treats `TASK` query results at task
level, with fields such as `status`, `checked`, `completed`, `fullyCompleted`,
`text`, `line`, `section`, `children`, `task`, and `blockId`.

Dataview is mostly display/index oriented. The documented write exception is
interactive task checking from a `TASK` query, which can update the original
file and can set a completion metadata field when configured. That makes task
completion the Dataview write behavior Etch should intentionally mirror.

## Candidate Commands

```sh
etch task complete <path> [--section <heading>] [--after <literal>] [--before <literal>] <text>
etch task reopen <path> [--section <heading>] [--after <literal>] [--before <literal>] <text>
etch list append <path> <heading> <item>
```

Examples:

```sh
etch task complete memory/2026-04-29.md --section "## Action Items" "Send follow-up"
etch list append memory/2026-04-29.md "## Action Items" "- [ ] Send follow-up"
```

## Semantics

- Task selectors should avoid matching arbitrary prose. The first version can
  require exact task text and uniqueness within an optional section or exact
  anchor window.
- `task complete` changes only the checkbox marker from `[ ]` to `[x]`.
- `task reopen` changes only `[x]` to `[ ]`.
- Dataview recognizes task/list metadata fields such as `due`, `completion`,
  `created`, `start`, and `scheduled`.
- Dataview can read those fields from ordinary inline fields and from
  task/list date shorthand conventions. If Etch adds an option that creates
  completion metadata while completing a task, it should write an ordinary
  inline field such as `[completion:: 2026-04-29]` because that form is explicit
  and easy to update deterministically.
- If Etch later updates an existing task/list date field and the source already
  uses Dataview shorthand, it should preserve that source form rather than add
  a duplicate inline field for the same meaning.
- `list append` appends one complete list item under a section and preserves the
  caller's marker spelling.
- List-item moves are deferred. They are likely a filtered case of a broader
  Markdown block move operation, where the source range is restricted to one
  list item and the destination is a section, list, or neighboring block.

## Impact

Spec:

- Add task/list selector rules.
- Define task status preservation for non-`x` custom statuses before supporting
  custom status mutation.

Docs:

- Add examples for completing, reopening, and appending list items.
- Explain interaction with Dataview completion metadata.

Code:

- Add `task complete`, `task reopen`, and `list append` to the verb catalog if
  approved.
- Add fixtures for exact task matching, ambiguous task text, section scoping,
  nested tasks, multiline list items, optional Dataview completion metadata,
  and no-op behavior.

## Deferred Move Design

`list move` should not land as a standalone one-off until Markdown move
addressing is clearer. A future move family could look like:

```sh
etch block move <path> --item <literal> --to-section <heading>
etch list move <path> --item <literal> --to-section <heading>
```

In that model, `list move` is a convenience wrapper or filtered form of a
general block move. The hard part is not the spelling; it is preserving the
source item's nested children, continuation lines, surrounding blank lines, and
destination list structure without guessing.

## Open Questions

- Should `task complete` optionally set `[completion:: <date>]` in the same
  operation?
- Should Etch preserve custom task statuses, or should the first version only
  handle `[ ]` and `[x]`?
- Is `list move` useful enough as a filtered command, or should all moves wait
  for a general Markdown block move proposal?
- Should `list append` accept only complete Markdown list item source, or should
  it infer the marker from the surrounding list?
