---
status: draft
depends_on: []
---

# Field Assignment Items

## Summary

Add HTTPie-style assignment items for setting multiple structured-data fields
or selectors in one invocation.

The first version can land for raw JSON and YAML files because those formats
already have singular selector semantics. Markdown body fields can adopt the
same assignment-item syntax later, after Markdown addressing and inline-field
resolution are settled.

## Candidate Commands

```sh
etch set <path.json> <selector=value>...
etch set <path.json> <selector:=json>...

etch set <path.yaml> <selector=value>...
etch set <path.yaml> <selector:=json>...
```

Examples:

```sh
etch set cache/state.json last_ingestion=2026-05-02 count:=12
etch set cache/state.json stats.daily.prompts:=4 tags:='["agent","active"]'
etch set cache/state.yaml owner=Brandon enabled:=true
```

## Semantics

- Assignment items are accepted only by field-setting surfaces.
- `selector=value` sets the selector to the literal string `value`.
- `selector:=json` sets the selector to the parsed strict JSON value.
- The selector side uses the same singular selector syntax as ordinary `set`.
  Bare member names remain shorthand for top-level fields.
- Multiple assignment items in one invocation are applied to the same file.
- The existing `set <path> <selector> <value>` form can remain available for
  single-field setting.
- If Markdown address flags later compose with assignment items, those flags
  must apply to the whole invocation. Argument order must not change which
  address applies to which field. To mutate fields in multiple sections, tasks,
  list items, or blocks, callers should use multiple script lines and `etch
  run`.
- Duplicate assignment targets in the same invocation are errors rather than
  last-write-wins.
- If any assignment item fails validation or mutation, the whole invocation
  fails and produces no commit.

## Rationale

Single-line multi-field updates are common when updating JSON watermarks, YAML
state files, frontmatter, and Dataview inline fields. A script already provides
batching, but assignment items make the common "set several fields in this one
file" shape compact while keeping Etch's one-invocation, one-plan, one-commit
model.

Raw JSON and YAML can use this syntax before Markdown fields because they only
need the existing structured selector engine. Keeping future Markdown address
flags global avoids order-sensitive command lines such as "these two fields in
this task, then that field in another task." Those belong in `etch run` where
each line has one address.

## Impact

Spec:

- Add assignment-item parsing for JSON and YAML `set`.
- Define `=` literal-string and `:=` strict-JSON value semantics.
- Define duplicate-target and partial-failure behavior.
- Reserve invocation-wide address flag semantics for later Markdown field
  composition.

Docs:

- Add examples for multi-field JSON and YAML object updates.
- Explain when to use assignment items versus `etch run`.

Code:

- Add assignment-item token parsing before command-specific decoding.
- Extend JSON and YAML `set` decoders to emit multiple normalized operations
  from one invocation.
- Add fixtures for literal values, JSON values, values containing `=`, duplicate
  targets, mixed valid/invalid assignments, and compatibility with single-field
  `set`.

## Open Questions

- Should assignment items be accepted by format-explicit plumbing commands such
  as `etch json set`, or only by porcelain `etch set`?
- Should `selector=` set an empty string?
- Should `selector:=null` be allowed for formats where `null` is meaningful?
- Should there be assignment syntax for deletion, or should delete stay
  one-field-per-command?
- When Markdown Fields lands, should Markdown frontmatter accept assignment
  items before body-addressed inline fields do?
