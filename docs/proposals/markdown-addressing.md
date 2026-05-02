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
--item-type <task|plain|numbered|bullet>
--task <literal>
--after <literal>
--before <literal>
```

## Scopes And Ranges

- `--body` selects the whole Markdown body after frontmatter.
- `--section <heading>` selects one heading-delimited section.
- `--item <literal>` selects one Markdown list item, including task, plain,
  numbered, and bullet-list items.
- `--item-type` narrows an `--item` selection. It is repeatable, and repeated
  item-type constraints are AND-ed.
- `--task <literal>` is shorthand for `--item <literal> --item-type task`.
- `--item-type` without `--item` or `--task` is an error.
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

- `--item` accepts either the source text after the list marker and checkbox, or
  the item source including the list marker and checkbox.
- `--item-type task` matches list items with a Markdown task checkbox marker,
  such as `[ ]` or `[x]`.
- `--item-type plain` matches list items without a Markdown task checkbox
  marker.
- `--item-type numbered` matches list items with a numbered/ordered marker,
  such as `1.` or `1)`.
- `--item-type bullet` matches list items with a bullet-list marker: `-`, `+`,
  or `*`.
- Contradictory item-type combinations, such as `task` with `plain` or
  `numbered` with `bullet`, are errors.
- Item matching is source-normalized, not rendered-text-normalized. Etch
  normalizes away the list marker, checkbox marker, surrounding whitespace, and
  Dataview inline field annotations.
- Inline Markdown syntax remains meaningful source text. `**Buy milk**` matches
  `**Buy milk**`, not `Buy milk`; `[Buy milk](url)` matches the link source,
  not just the rendered label.
- The normalized source text must match exactly.
- Repeated matching items are ambiguous and cause an error. If the same text
  appears as both a numbered task and a bullet-list task, callers can add
  `--item-type numbered` or `--item-type bullet`.
- Complex cases such as nested items, multiline items, or items whose normalized
  source text is unstable should fail rather than guess.
- Obsidian-compatible block IDs are deferred to
  [Markdown Block IDs](block-ids.md).

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

- Add reusable Markdown addressing helpers for body, section, item, item-type,
  task shorthand, and anchor resolution.
- Add fixtures for title and ATX heading matching, repeated headings, closing
  heading markers, item source normalization, item-type filters, ambiguous
  items, nested items, and multiline items.

## Rationale

These rules are cross-cutting. Keeping them in one proposal prevents each
Markdown feature from inventing slightly different section, item, and anchor
matching behavior. The general philosophy is exact matching with ergonomic
syntax normalization, followed by clear failure when the target is missing,
ambiguous, or structurally complex.
