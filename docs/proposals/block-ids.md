---
status: draft
depends_on:
  - markdown-addressing
---

# Markdown Block IDs

## Summary

Consider Obsidian-compatible block IDs as a first-class Markdown addressing
form. Block IDs are source anchors such as `^launch-note` attached to a
paragraph, list item, task item, or other block.

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

## Questions

- Which Markdown blocks can carry block IDs for Etch purposes?
- Does `--block follow-up-task` match source `^follow-up-task`, or should
  callers include the caret?
- Should Etch ever create block IDs, or only target existing ones?
- How should block IDs interact with `--section`, `--item`, and `--after` or
  `--before`?
- Are duplicate block IDs an ambiguity error or a malformed-document error?

## Impact

Spec:

- Add block ID parsing and selector semantics if adopted.
- Define how block IDs compose with existing Markdown addressing flags.

Docs:

- Add examples for addressing paragraphs, list items, and task items by stable
  block ID.

Code:

- Add block ID detection to the Markdown addressing helper.
- Add fixtures for paragraph IDs, list-item IDs, task-item IDs, duplicate IDs,
  missing IDs, and IDs inside code blocks.
