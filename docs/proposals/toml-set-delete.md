---
status: draft
depends_on: []
---

# TOML Set/Delete

## Summary

Add TOML scalar/key updates for hand-authored configuration files such as
`mise.toml`, with source-preservation requirements strong enough that Etch does
not become a TOML formatter by accident.

## Candidate Commands

```sh
etch set <path.toml> <selector> <value>
etch delete <path.toml> <selector>

etch toml set <path> <selector> <value>
etch toml delete <path> <selector>
```

Example:

```sh
etch set mise.toml tasks.test.description '"Run tests"'
etch toml set mise.toml tasks.test.description '"Run tests"'
```

## Semantics

- Porcelain `set` and `delete` infer TOML for `.toml` paths.
- `toml set` and `toml delete` are format-explicit plumbing commands.
- Selectors use the same singular path model as JSON/YAML: relative dotted
  shorthand is normalized to rooted JSONPath-style selectors.
- Selectors address TOML keys, dotted tables, and array-of-table elements, but
  the first version should only update or delete existing scalar leaves.
- Values are parsed as TOML values. JSON string syntax can be accepted for
  ordinary strings only if the parser/rewriter can round-trip the resulting
  TOML string without ambiguity.
- `set` creates a missing final scalar key only when every parent table already
  exists and has an unambiguous insertion point.
- `set` does not create missing parent tables in the first version.
- `delete` removes an existing scalar key and is a no-op when the final key is
  absent.
- Deleting tables, arrays of tables, comments, or mixed inline structures is
  out of scope for the first version.

## Preservation Contract

TOML support must preserve:

- comments and blank lines
- table order
- key order within each table
- table header spelling, including dotted table spelling
- quoted key spelling
- indentation around values and inline comments
- scalar spelling for untouched values
- multiline strings and literal strings outside the changed value
- array and inline table formatting outside the changed value

Whole-document reformatting is a feature failure. The implementation should
rewrite the smallest source range the parser can identify safely.

## Parser Acceptance Gate

Before TOML support lands, candidate parser/rewriter integration must pass
fixtures for:

- updating an existing scalar in a table
- creating a missing final scalar in an existing table
- deleting an existing scalar
- no-op deletion of a missing scalar
- comments before, after, and inline with keys
- blank lines between tables
- dotted tables and dotted keys
- quoted keys
- arrays, arrays of tables, and inline tables
- multiline basic strings and literal strings
- malformed TOML refusal without rewriting

## Rationale

TOML matters for adopter settings and project tooling, and it fits Etch's
structured mutation model better than generic text replacement. The risk is
source preservation: configuration files often carry comments, intentional
ordering, and compact hand formatting. A TOML adapter is worthwhile only if it
can make targeted scalar changes without erasing that human structure.

## Impact

Spec:

- Add TOML as a format adapter for scalar `set` and `delete`.
- Define porcelain `.toml` inference for `set` and `delete`.
- Define scalar-only creation/deletion limits and preservation requirements.

Docs:

- Add TOML examples for porcelain `set`/`delete` and plumbing `toml set` /
  `toml delete`.
- Document unsupported table and array-of-table structural edits.

Code:

- Add TOML parser/rewriter integration after the parser acceptance gate passes.
- Add TOML adapter tests for selector normalization, scalar rendering,
  preservation fixtures, deletion, no-op deletion, malformed TOML, and
  porcelain/plumbing dispatch.

## Open Questions

- Which TOML parser gives enough source-preservation control?
- Should JSON string syntax be accepted as a convenience for TOML strings, or
  should values be TOML-only?
- What is the insertion policy for creating a missing final key in an existing
  table?
- Should array element updates be allowed in the first version, or are scalar
  table keys enough?
