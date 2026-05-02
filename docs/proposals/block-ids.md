---
status: draft
depends_on:
  - markdown-addressing
---

# Markdown Block IDs

## Summary

Add Obsidian-compatible block IDs as a first-class Markdown addressing form.
Block IDs are source anchors such as `^launch-note` attached to a paragraph,
list item, task item, or other block.

This proposal is separate from the base Markdown addressing proposal so the
core selector model can land without committing to Obsidian-specific anchor
semantics.

## Candidate Addressing Flag

```sh
--block <id>
```

Example:

```sh
etch set memory/tasks/follow-up.md done "2026-05-01" --block follow-up-task
```

## Semantics

- Callers pass the ID value without the caret. `--block follow-up-task` matches
  source `^follow-up-task`.
- Passing a caret-prefixed value, such as `--block ^follow-up-task`, is an
  error. The caret is source syntax, not part of the ID.
- `--block` targets existing block IDs only. Etch does not create block IDs or
  generate new ID values.
- Duplicate block IDs are ambiguity errors, consistent with duplicate matching
  headings.
- `--block` is composable with `--section`; the section narrows the search
  scope before the block ID is resolved.
- `--block` is mutually exclusive with `--item` and `--task` because a block ID
  already identifies one block or item.
- `--block` can be used by commands that need a block or item range. Commands
  that need an insertion point should use placement flags such as `--before`,
  `--after`, `--head`, or `--tail` according to the shared Markdown addressing
  rules.
- Etch ignores block-ID-looking text inside fenced code blocks, indented code
  blocks, inline code spans, and raw HTML blocks.

## Addressable Blocks

The first version should recognize Obsidian-compatible block IDs attached to:

- paragraphs
- list items
- task items
- blockquotes or callouts

Other block shapes, such as tables, code blocks, thematic breaks, and headings,
should fail until there is a concrete command that can safely mutate them by
block ID.

## Impact

Spec:

- Add block ID parsing and selector semantics.
- Define how block IDs compose with existing Markdown addressing flags.

Docs:

- Add examples for addressing paragraphs, list items, and task items by stable
  block ID.

Code:

- Add block ID detection to the Markdown addressing helper.
- Add fixtures for paragraph IDs, list-item IDs, task-item IDs, section-scoped
  lookup, duplicate IDs, missing IDs, caret-prefixed argument rejection,
  mutual exclusion with item selectors, and IDs inside code blocks.

Verification:

- Assert `--block launch-note` matches source `^launch-note`.
- Assert `--block ^launch-note` fails as invalid input.
- Assert duplicate block IDs fail as ambiguous and report candidate locations.
- Assert `--section Status --block launch-note` searches within the `Status`
  section only.
- Assert `--block launch-note --task "Follow up"` fails before planning.
