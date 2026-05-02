---
status: implemented
depends_on:
  - markdown-addressing
---

# Markdown Task and List Operations

## Summary

Add a narrow Markdown task/list layer for checkbox toggles and list-item adds.
This proposal is deliberately narrower than generic Markdown list editing.

## Dataview Background

Dataview indexes tasks and list items. It treats `TASK` query results at task
level, with fields such as `status`, `checked`, `completed`, `fullyCompleted`,
`text`, `line`, `section`, `children`, `task`, and `blockId`.

Dataview is mostly display/index oriented. The documented write exception is
interactive task checking from a `TASK` query, which can update the original
file and can set a completion metadata field when configured. That makes task
checkbox changes the Dataview write behavior Etch should intentionally mirror.

## Candidate Commands

```sh
etch task close <path> <text> [--section <heading>] [--before <literal>] [--after <literal>]
etch task open <path> <text> [--section <heading>] [--before <literal>] [--after <literal>]
etch list add <path> <text> [--section <heading>] [--task] [--before <literal>] [--after <literal>]
etch task add <path> <text> [--section <heading>] [--before <literal>] [--after <literal>]
```

Examples:

```sh
etch task close memory/2026-04-29.md "Send follow-up" --section "## Action Items"
etch task open memory/2026-04-29.md "Send follow-up" --section "## Action Items"
etch list add memory/2026-04-29.md "Send follow-up" --section "## Action Items" --task
etch task add memory/2026-04-29.md "Send follow-up" --section "## Action Items"
```

## Semantics

- Task selectors should avoid matching arbitrary prose. The first version can
  require exact task text and uniqueness within an optional section or exact
  anchor window.
- `task close` changes only the checkbox marker from `[ ]` to `[x]`.
- `task close` does not create missing tasks.
- `task open` is an "ensure open task" operation:
  - If a matching open task exists, it is a no-op.
  - If a matching closed task exists, it changes `[x]` or `[X]` to `[ ]`.
  - If no matching task exists, it creates `- [ ] <text>` using the same
    placement and marker inference rules as `task add`.
  - If multiple matching tasks exist, it fails as ambiguous.
- Missing-task creation through `task open` requires a destination address such
  as `--section`, `--before`, or `--after`; bare `task open <path> <text>`
  should fail rather than append to the document body by accident.
- Custom task statuses are out of scope for the first version. A task with a
  checkbox marker other than `[ ]`, `[x]`, or `[X]` fails rather than guessing
  how that status maps to open or closed.
- `list add` adds one list item in the selected Markdown address. `<text>` is
  plain item text, not Markdown list item source.
- `list add` defaults to tail placement within the selected compatible list.
- `--before` and `--after` use the placement rules from
  [Markdown Addressing](markdown-addressing.md) to choose an insertion point
  relative to an existing item in the selected section or body. For task/list
  commands, these anchors match list items, not arbitrary prose.
- `task add` is shorthand for `list add ... --task`.
- `list add ... --task` constructs an unchecked task item.
- If the selected section already contains a compatible list, Etch follows that
  list's marker style. Bullet lists reuse the existing bullet marker. Numbered
  lists continue numbering.
- If the selected section has no compatible list, plain `list add` defaults
  to `- <text>` and `--task` defaults to `- [ ] <text>`.
- `list add` rejects multiline text in the first version.
- `list add` rejects text that already starts with a Markdown list marker;
  callers pass item text, not full list item source.
- Command-local Markdown address flags are order-insensitive. These forms are
  equivalent:

```sh
etch task add note.md "Send follow-up" --section Actions
etch task add note.md --section Actions "Send follow-up"
```

- List-item moves are deferred. They are likely a filtered case of a broader
  Markdown block move operation, where the source range is restricted to one
  list item and the destination is a section, list, or neighboring block.

## Deferred Dataview Metadata

Dataview recognizes task/list metadata fields such as `due`, `completion`,
`created`, `start`, and `scheduled`. Task completion metadata is useful, but it
depends on Markdown inline field mutation and should not be part of the first
task/list operation.

If Etch later adds an option that creates completion metadata while closing a
task, it should write an ordinary inline field such as
`[completion:: 2026-04-29]` because that form is explicit and easy to update
deterministically. If Etch updates an existing task/list date field and the
source already uses Dataview shorthand, it should preserve that source form
rather than add a duplicate inline field for the same meaning.

## Impact

Spec:

- Add task/list selector rules.
- Define list add placement, marker inference, and validation.
- Define task status preservation for non-`x` custom statuses before supporting
  custom status mutation.

Docs:

- Add examples for opening, closing, adding plain list items, and adding task
  items.
- Explain deferred interaction with Dataview completion metadata.

Code:

- Add `task close`, `task open`, `list add`, and `task add` to the verb
  catalog.
- Add fixtures for exact task matching, ambiguous task text, section scoping,
  nested tasks, list add marker inference, default tail placement, before/after
  placement, task add shorthand, multiline add refusal, full-source add refusal,
  custom task status refusal, and no-op behavior.

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

## Deferred

- Should `task close` later set `[completion:: <date>]` in the same
  operation, or should that stay a separate Markdown field operation?
- Is `list move` useful enough as a filtered command, or should all moves wait
  for a general Markdown block move proposal?
