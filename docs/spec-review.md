# Spec Review Burn-Down

This file is temporary. Keep only feedback that has not yet been absorbed into
the spec, implementation, tests, or an explicit drop decision. Delete this file
when this disposition record is no longer useful.

## Disposition

- Plan hashing, YAML preservation, dependency choices, dry-run object isolation,
  help surfaces, preview-mode checkout behavior, retry windows, numeric parsing,
  checkout conversion detection, workspace dependency injection, and error
  classification have been folded into `spec.md`, code, and regression tests.
- JSON duplicate object names are an AST-editing decision, not a strict-data
  parsing bug. `spec.md` now documents that source edits target the first
  matching object member while value materialization uses last-name-wins map
  semantics.
- YAML multi-document behavior and touched-node formatting are product
  decisions. `spec.md` now documents first-document MVP behavior and generated
  formatting for new or replaced YAML nodes.
- Whole-message prefix/suffix feedback was resolved by replacing those flags
  with `--subject-prefix`, `--subject-suffix`, `--body-prefix`, and
  `--body-suffix`. Subject modifiers are literal first-line affixes; body
  modifiers are body blocks separated from generated body text by blank lines.
- `create` remains classified as `idempotent`; `CommandClass` and `spec.md` now
  clarify that classes describe content-change behavior within a transaction
  base, not post-commit rerun behavior.
- The limited text merge implementation is accepted for MVP. `spec.md` now says
  the merge engine may conservatively report conflicts for complex text edits it
  cannot confidently merge.
- The table ordinal parser concern is dropped. In Markdown table commands, the
  optional table ordinal is a positional `@N` immediately after scope; row
  selectors after `--before` or `--after` are unambiguous.
- The coverage inventory from the review is not tracked here. `spec.md` section
  18 is the verification backlog and remains the source of truth for missing or
  shallow fixture coverage.

## Remaining Feedback

None.
