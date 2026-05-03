---
status: implemented
depends_on:
  - markdown-addressing
---

# Markdown Section Insertion and Block Whitespace

## Summary

Add a coherent section command family. `section append` and `section prepend`
insert block fragments under an existing ATX heading without requiring callers
to read, concatenate, and replace the whole section body. `section replace`
replaces the whole section body.

This proposal includes the block-fragment whitespace model because that is what
makes insertion safe for generated scripts.

## Candidate Commands

The command shape should group operations by Markdown object. This makes
section operations one family and leaves room for later commands such as
`section delete` or `section move`.

```sh
etch section replace <path> <heading> <content>
etch section append <path> <heading> <content>
etch section prepend <path> <heading> <content>

etch md section replace <path> <heading> <content>
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

- `<heading>` uses the shared Markdown heading selector rules from
  [Markdown Addressing](markdown-addressing.md).
- Missing or ambiguous headings are errors.
- `section replace` replaces the whole selected section body.
- `section append` inserts content at the end of the selected section body,
  inside the section boundary.
- `section prepend` inserts content at the beginning of the selected section
  body, inside the section boundary.
- The caller owns the interior bytes of `<content>`.
- For append and prepend, Etch owns the boundary whitespace where the fragment
  meets the existing section body.
- `replace-section` is removed in favor of `section replace`; this is an
  intentional breaking change from the MVP spelling.

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
2. Obsidian-compatible block ID, if that addressing proposal is adopted and the
   note already has one.
3. Exact list item or task text for list/task commands.
4. Exact `--after`/`--before` anchor windows.

Avoid ordinal selectors such as "third paragraph" or "second block" unless a
future command exposes them as an explicit last-resort escape hatch.

## Block-Fragment Spacing

- Section commands treat `<content>` as a Markdown block fragment.
- Etch normalizes line endings according to the target file's existing newline
  style.
- Etch trims leading and trailing blank lines from the fragment.
- `section append` and `section prepend` reject a blank-only fragment;
  `section replace` uses it to clear the section body.
- Etch preserves interior bytes, including interior blank lines, indentation,
  list markers, code fences, and inline markup.
- If the section body is non-empty, Etch trims trailing blank lines from the
  existing body for append and leading blank lines from the existing body for
  prepend.
- For `section replace`, Etch preserves existing blank-line boundaries between
  the heading and body, and between the body and following heading, when those
  boundaries are present in the replaced section.
- When both the existing section body and inserted fragment are non-empty, Etch
  separates them with exactly one blank line. In source terms, that means two
  newline sequences between the last nonblank line on one side and the first
  line on the other side, using the file's newline style.
- When the section body is empty, the fragment starts immediately after the
  heading line with one newline.
- This empty-section behavior is intentionally different from non-empty
  insertion: no blank separator is needed until there is existing content to
  separate from the inserted block fragment.
- When appending to an existing list as the same list, callers should use
  `list add` rather than `section append`; section insertion always creates
  a block boundary.
- When appending rows to an existing table, callers should use table row
  operations rather than `section append`.

## Rationale

This keeps "append to an ingestion section" atomic from `HEAD`. Without this
operation, a script must read the section, concatenate text, and call
`section replace`, which reintroduces stale-read and dirty-checkout risk.

Blank lines in Markdown are structural. Rather than infer a block-type-specific
spacing matrix, `section append` and `section prepend` use one deterministic
block boundary for every non-empty insertion. More specific operations can
handle list adjacency, table rows, and other structures that intentionally need
tighter spacing.

## Impact

Spec:

- Replace the `replace-section` spelling with `section replace`.
- Add `section append` and `section prepend` to the Markdown verb surface.
- Define Markdown block-fragment insertion and boundary-whitespace rules.

Docs:

- Extend `md` plumbing help and `verbs --json` documentation for
  `section replace`, `section append`, and `section prepend`.
- Add examples for section replace/append/prepend around paragraphs, lists, and
  empty sections.

Code:

- Add plan and commit-message descriptors for section replace and insertion.
- Add fixtures for empty sections, non-empty append, non-empty prepend,
  deterministic one-blank-line boundaries, list adjacency, table adjacency,
  leading and trailing blank lines in payloads, existing body boundary trimming,
  and files with no trailing newline.

## Open Questions

- Should a later `--create` flag create the section when absent?
