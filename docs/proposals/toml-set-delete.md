---
status: draft
depends_on: []
---

# TOML Set/Delete

## Summary

Add TOML scalar/key updates only if Etch can preserve comments, ordering, and
table layout well enough for configuration files such as `mise.toml`.

## Candidate Commands

```sh
etch toml set <path> <selector> <value>
etch toml delete <path> <selector>
```

Example:

```sh
etch toml set mise.toml tasks.test.description '"Run tests"'
```

## Semantics

- Selectors use the same singular path model as JSON/YAML.
- Values are parsed as TOML values, with JSON string syntax accepted only if the
  selected TOML parser supports unambiguous conversion.
- Table and array-of-table mutation beyond scalar/key updates is deferred.
- Whole-document reformatting is not acceptable for common config files.

## Rationale

TOML matters for adopter settings and project tooling, but its value is lower
than Markdown body operations. It should not land before a preservation fixture
suite proves that ordinary hand-authored config files stay readable.

## Impact

Spec:

- Add TOML to format-adapter decisions if fixture-gated preservation succeeds.
- Decide whether porcelain `set` should infer TOML for `.toml` paths.

Docs:

- Add `toml set` and `toml delete` to plumbing help and `verbs --json`
  documentation.

Code:

- Add TOML parser/rewriter integration only after preservation fixtures pass.
- Add fixtures for comments, ordering, dotted tables, arrays, inline tables,
  quoted keys, multiline strings, deletion, no-op deletion, and malformed TOML.

## Open Questions

- Is TOML support worth implementing before Markdown task/list operations?
- Which TOML parser gives enough source-preservation control?
- Should the first version support creating missing tables, or only updating
  existing scalar keys?
