---
status: draft
depends_on: []
---

# Markdown Addressing

## Summary

Define shared addressing rules for Markdown body operations. Inline fields,
section insertion, task/list operations, and future section deletion should use
the same vocabulary for locating parts of a Markdown file.

## Candidate Addressing Flags

```sh
--body
--section <heading>
--item <literal>
--task <literal>
--bullet <literal>
--after <literal>
--before <literal>
```

## Scopes And Ranges

- `--body` selects the whole Markdown body after frontmatter.
- `--section <heading>` selects one heading-delimited section.
- `--item <literal>` selects one list item or task.
- `--task <literal>` selects one task item.
- `--bullet <literal>` selects one non-task list item.
- `--after` and `--before` narrow the selected body, section, or item scope.
- Missing selectors are errors for mutating commands that need an existing
  location.
- Ambiguous selectors are errors. Etch reports the candidate locations rather
  than choosing one.

## Insertions

Addressing identifies ranges. Insertion direction belongs to the command using
the address.

- Commands whose subcommand encodes direction, such as `section append` and
  `section prepend`, use that direction.
- Commands that create content inside a selected range should define their own
  placement policy, such as field-specific `--head`/`--tail`, operation-specific
  defaults, or requiring `--after`/`--before`.
- `--after <literal>` and `--before <literal>` narrow the range and can also
  provide a concrete insertion point for commands that create content near
  existing text.
- If a command cannot determine one insertion point, it should fail. Etch is
  mostly driven by agents, and a failed structural mutation can fall back to an
  ordinary manual edit.

## Headings

- `--section` accepts either a title, such as `Status`, or an ATX heading, such
  as `## Status`.
- Passing a title searches all ATX heading levels for that exact normalized
  title.
- Passing an ATX heading includes the level in the match, so `## Status` matches
  level-2 `Status` headings only.
- Closing `#` markers are ignored for matching.
- Heading whitespace is normalized around the title.
- Repeated matching headings are ambiguous and cause an error.

## Items

- `--item` accepts either the item text or the item source including the list
  marker and checkbox.
- `--task` and `--bullet` use the same matching rules as `--item`, but filter
  the candidate set first. `--task` only considers task list items.
  `--bullet` only considers non-task list items, including unordered and ordered
  list items.
- Item matching normalizes away the list marker, checkbox marker, surrounding
  whitespace, and Dataview inline field annotations.
- The normalized text must match exactly.
- Repeated matching items are ambiguous and cause an error.
- Complex cases such as nested items, multiline items, or items whose normalized
  text is unstable should fail rather than guess.
- `--task` is useful when the same text appears as both a plain list item and a
  task. `--bullet` is the inverse convenience for prose lists.
- Block IDs should become a first-class addressing form if adopters use them as
  stable anchors.

## Impact

Spec:

- Define shared Markdown body addressing flags and matching rules.
- Specify missing and ambiguous selector behavior for Markdown body operations.
- Decide heading and item normalization rules before dependent features land.

Docs:

- Add one shared addressing help topic instead of repeating rules in every
  Markdown command.
- Cross-link section insertion, Markdown fields, and task/list docs to this
  topic.

Code:

- Add reusable Markdown addressing helpers for body, section, item, task,
  bullet, and anchor resolution.
- Add fixtures for title and ATX heading matching, repeated headings, closing
  heading markers, item text normalization, ambiguous items, nested items,
  multiline items, and block ID behavior if adopted.

## Rationale

These rules are cross-cutting. Keeping them in one proposal prevents each
Markdown feature from inventing slightly different section, item, and anchor
matching behavior. The general philosophy is exact matching with ergonomic
syntax normalization, followed by clear failure when the target is missing,
ambiguous, or structurally complex.
