---
status: draft
depends_on:
  - markdown-addressing
---

# Markdown Section Insertion and Block Whitespace

## Summary

Add section insertion commands that append or prepend block fragments under an
existing ATX heading without requiring callers to read, concatenate, and replace
the whole section body.

This proposal includes the block-fragment whitespace model because that is what
makes insertion safe for generated scripts.

## Candidate Commands

The command shape should group operations by Markdown object: `section append`
and `section prepend`. This keeps section operations together and leaves room
for later commands such as `section delete` or `section move`.

```sh
etch section append <path> <heading> <content>
etch section prepend <path> <heading> <content>

etch md section append <path> <heading> <content>
etch md section prepend <path> <heading> <content>
```

Examples:

```sh
etch section append memory/2026-04-29.md "## Heartbeat" "$content"

etch section append memory/programs/spender-agent.md "## Key Interactions" <<'EOF'
- Met with finance partner about launch readiness.
EOF
```

## Section Semantics

- `<heading>` uses the same exact ATX heading selector as `replace-section`.
- Missing or ambiguous headings are errors.
- Content is inserted at the end or beginning of the selected section body,
  inside the section boundary.
- The caller owns the interior bytes of `<content>`.
- Etch owns the boundary whitespace where the fragment meets the existing
  section body.

## Block Model

Etch should reason about Markdown body mutations in terms of source ranges for
recognized blocks, not by re-rendering a Markdown AST. A block is a parser-known
unit inside a selected scope:

- paragraph
- list
- list item
- task item
- table
- blockquote or callout
- fenced or indented code block
- HTML block
- thematic break
- heading

Sections are heading-delimited block ranges. This block model is a safety
model, not a mandate to expose generic paragraph editing commands.

Prefer stable scopes and anchors in this order:

1. Heading scope.
2. Obsidian-style block ID, if the note already has one.
3. Exact list item or task text for list/task commands.
4. Exact `--after`/`--before` anchor windows.

Avoid ordinal selectors such as "third paragraph" or "second block" unless a
future command exposes them as an explicit last-resort escape hatch.

## Block-Fragment Spacing

- `section append` and `section prepend` treat `<content>` as a Markdown block
  fragment.
- Etch normalizes line endings according to the target file's existing newline
  style.
- Etch trims leading and trailing blank lines from the fragment and rejects a
  blank-only fragment.
- Etch preserves interior bytes, including interior blank lines, indentation,
  list markers, code fences, and inline markup.
- Etch inserts the minimum boundary whitespace needed to keep the old content
  and inserted fragment from merging into one paragraph, code block, table, or
  other accidental structure.
- When the section body is empty, the fragment starts immediately after the
  heading line with one newline.
- When inserting next to paragraph-like or table-like content, Etch uses one
  blank line as the separator.
- When appending to an existing list as the same list, callers should use
  `list append` rather than relying on `section append` spacing heuristics.
- `replace-section` remains the raw whole-body operation for callers that need
  exact surrounding whitespace control.

## Rationale

This keeps "append to an ingestion section" atomic from `HEAD`. Without this
operation, a script must read the section, concatenate text, and call
`replace-section`, which reintroduces stale-read and dirty-checkout risk.

Blank lines in Markdown are structural. They can separate paragraphs, loosen
lists, end blockquotes, and prevent accidental table or code-block extension.
Etch should therefore make boundary whitespace deterministic for insertion
commands.

## Impact

Spec:

- Add `section append` and `section prepend` to the Markdown verb surface.
- Define Markdown block-fragment insertion and boundary-whitespace rules.

Docs:

- Extend `md` plumbing help and `verbs --json` documentation.
- Add examples for section append/prepend around paragraphs, lists, and empty
  sections.

Code:

- Add plan and commit-message descriptors for section insertion.
- Add fixtures for empty sections, paragraph boundaries, table boundaries,
  fenced code boundaries, list adjacency, leading and trailing blank lines in
  payloads, and files with no trailing newline.

## Open Questions

- Should a later `--create` flag create the section when absent?
- Are the proposed block-fragment spacing rules acceptable, or should callers
  have a command-local flag for exact raw insertion?
