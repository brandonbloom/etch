---
status: implemented
depends_on: []
---

# Markdown Addressing

## Summary

Define shared addressing and placement rules for Markdown body operations.
Inline fields, section insertion, task/list operations, and future section
deletion should use the same vocabulary for locating parts of a Markdown file.

Implementation note: this proposal landed as shared internal helpers and
documentation. Existing section commands use the shared heading resolver; item,
task, and placement helpers are ready for dependent Markdown commands.

## Candidate Addressing Flags

```sh
--body
--section <heading>
--item <literal>
--item-type <task|plain|numbered|bullet>
--task <literal>
--after <literal>
--before <literal>
--head
--tail
```

## Scopes And Ranges

- `--body` selects the whole Markdown body after frontmatter.
- `--section <heading>` selects one heading-delimited section.
- `--item <literal>` selects one Markdown list item, including task, plain,
  numbered, and bullet-list items.
- `--item-type` narrows an `--item` selection. It is repeatable, and repeated
  item-type constraints are AND-ed across independent axes.
- `--task <literal>` is shorthand for `--item <literal> --item-type task`.
- `--item-type` without `--item` or `--task` is an error.
- `--after` and `--before` identify a location relative to an existing anchor
  inside the selected body, section, or item scope.
- `--head` and `--tail` identify the start or end of the selected body,
  section, list, or item scope.
- Missing selectors are errors for mutating commands that need an existing
  location.
- Ambiguous selectors are errors. Etch reports the candidate locations rather
  than choosing one.

## Placement

Markdown addressing can select a range, a placement point, or both. Commands
decide which kind of address they accept, but they should use these shared
meanings.

- Commands whose subcommand encodes direction, such as `section append` and
  `section prepend`, use that direction.
- Commands whose verb creates a new item without encoding direction, such as
  `list add`, should default to `--tail` unless the command specifies a more
  specific default.
- `--head` places new content at the beginning of the selected scope.
- `--tail` places new content at the end of the selected scope.
- `--after <literal>` places new content immediately after the unique matching
  anchor inside the selected scope.
- `--before <literal>` places new content immediately before the unique matching
  anchor inside the selected scope.
- `--head`, `--tail`, `--after`, and `--before` are mutually exclusive for
  commands that create one insertion point.
- If a command has an operation-specific placement default, explicitly passing
  one placement flag overrides that default.
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
- Item types have two axes: task state (`task` or `plain`) and marker shape
  (`numbered` or `bullet`). Repeating `--item-type` can combine one constraint
  from each axis, such as `--item-type task --item-type numbered`.
- Contradictory item-type combinations within the same axis, such as `task`
  with `plain` or `numbered` with `bullet`, are errors.
- Numbered tasks, such as `1. [ ] Do thing`, are supported. They match
  `--item-type task --item-type numbered`. If a valid item-type combination has
  no matching source item, that is an ordinary no-match error, not a selector
  syntax error.
- Item matching is rendered-text-normalized. Etch normalizes away the list
  marker, checkbox marker, surrounding whitespace, Dataview inline field
  annotations, and numeric trailing reference-annotation links, then compares
  the rendered inline Markdown text.
- Inline Markdown syntax is not part of the item identity. `**Buy milk**`
  matches `Buy milk`; `[Buy milk](url)` matches `Buy milk`.
- The normalized item text must match exactly.
- Repeated matching items are ambiguous and cause an error. If the same text
  appears as both a numbered task and a bullet-list task, callers can add
  `--item-type numbered` or `--item-type bullet`.
- Complex cases such as nested items, multiline items, or items whose normalized
  item text is unstable should fail rather than guess.
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
  task shorthand, anchor resolution, and placement resolution.
- Add fixtures for title and ATX heading matching, repeated headings, closing
  heading markers, item source normalization, item-type filters, numbered
  tasks, ambiguous items, nested items, multiline items, head/tail placement,
  before/after placement, and conflicting placement flags.

Verification:

- Unit-test item-type classification independently from item text matching.
- Include examples for bullet tasks, numbered tasks, plain bullet items, and
  plain numbered items.
- Assert that `task` with `plain` and `numbered` with `bullet` fail as
  contradictory filters.
- Assert that a valid but absent combination, such as a numbered task in a file
  with only bullet tasks, fails as a no-match.

## Rationale

These rules are cross-cutting. Keeping them in one proposal prevents each
Markdown feature from inventing slightly different section, item, and anchor
matching behavior. The general philosophy is exact matching with ergonomic
syntax normalization, followed by clear failure when the target is missing,
ambiguous, or structurally complex.
