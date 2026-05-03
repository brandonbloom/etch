---
status: draft
depends_on: []
---

# Help And Usage Polish

## Summary

Bring terse usage errors and the `help addressing` topic back into alignment
with implemented behavior, so the surfaces users see while debugging commands
match the reference help and tests.

## Scope

This proposal covers two adopter-reported rough edges:

- `add`, `append`, and `remove` usage strings do not mention that structured
  values may be passed with `--json`.
- `help addressing` describes inline Markdown matching too broadly. The topic
  should distinguish annotation-like inline constructs that Etch strips during
  item matching from inline formatting source that remains part of the literal
  text, if that is the intended behavior.

## Proposed Usage Text

Structured command usage should make JSON value typing discoverable at the
point where an invocation fails. The exact strings can vary by command shape,
but they should communicate both valid positions:

```text
usage: etch add <path> <selector> [--json] <value>
usage: etch append <path> <selector> [--json] <value>
usage: etch remove <path> <selector> [--json] <value>
```

If the value-specific help topic stays the complete reference, the usage error
can still be compact, but it should not imply string-only values.

## Addressing Text

The help topic should match the chosen item-normalization contract:

- List markers, task checkboxes, surrounding whitespace, Dataview inline
  fields, wiki links, and Markdown links used as annotations are stripped when
  matching list and task item selectors.
- Bold, italic, and inline-code behavior should be verified against tests and
  documented precisely.
- The topic should include one example where a plain selector matches an item
  with trailing annotation links or fields.

## Rationale

Usage errors are often the first documentation users see. If the terse usage
string omits `--json`, users can conclude the command does not support typed
values even when the full help topic says it does.

Addressing help needs extra precision because selector matching is a trust
surface: users must know whether they are matching source Markdown, rendered
text, or Etch-normalized text.

## Impact

Docs:

- Update `help addressing`.
- Ensure generated Reference text follows from CLI help.

Code:

- Update command usage errors for structured `add`, `append`, and `remove`.
- Add or update tests asserting usage output mentions `--json`.
- Add or update tests that pin item matching examples used by help text.

## Open Questions

- Should JSONL `append` keep separate usage because its value is always strict
  JSON and does not use `--json`?
- Should the help topic call annotation stripping "rendered text",
  "normalized text", or another term?
