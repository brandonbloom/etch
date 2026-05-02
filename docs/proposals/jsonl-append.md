---
status: draft
depends_on: []
---

# JSONL Append

## Summary

Add a JSONL adapter for append-only event streams. The concrete adopter need is
an event log where scripts record one prompt, heartbeat, ingestion step, or
counter observation per line without reading and rewriting the whole file.

## Candidate Commands

```sh
etch append <path.jsonl> <json-value>
etch append <path.ndjson> <json-value>

etch jsonl append <path> <json-value>
```

Example:

```sh
etch append cache/stats-events.jsonl '{"kind":"prompt","at":"2026-04-29T10:00:00-07:00"}'
etch jsonl append cache/stats-events.jsonl '{"kind":"prompt","at":"2026-04-29T10:00:00-07:00"}'
```

The top-level `append` form is porcelain: `.jsonl` and `.ndjson` paths infer the
newline-delimited JSON adapter and take no selector. The `jsonl append` form is
monomorphic plumbing for callers that want explicit format selection.

## Semantics

- `<json-value>` must be strict JSON.
- JSON Lines and NDJSON both allow any valid JSON value per line. Etch should
  accept any JSON value, though event-log conventions should use objects.
- Etch validates that every existing non-empty line is valid JSON before
  appending.
- The appended value is rendered as one compact JSON line plus `\n`.
- The command is non-idempotent.
- `.jsonl` and `.ndjson` are handled by the same adapter. There is no separate
  NDJSON command family unless users need spelling aliases.
- Blank lines are rejected by default because JSON Lines disallows them. A
  future compatibility flag could ignore blank lines if needed for NDJSON files
  produced by permissive tools.
- There is no JSONL `set`, `delete`, `remove`, or `move` in the first version.

## Other Operations

- `append` is the core JSONL operation because it preserves the event-log model
  and does not need line ordinals or selectors.
- `validate` could be useful as a guard-like operation, but it does not mutate
  and should be considered with other validation commands.
- `add` is not a natural event-log operation because duplicate events can be
  meaningful. If a future queue/set use case needs idempotent append-by-value,
  it should be designed separately.
- `set`, `delete`, `remove`, and `move` imply line-addressed rewriting. Those
  operations should stay out of scope unless a concrete repair workflow needs
  them.

## Rationale

Event streams are naturally append-only. Treating JSONL as a JSON array would
require whole-file rewriting and would blur the event-log model. Requiring
objects would match many event-log conventions, but it would make Etch narrower
than JSON Lines and NDJSON themselves.

The porcelain `append <path.jsonl> <json-value>` form is worth adding because
scripts should not need to know whether append means "append to an array inside
a JSON document" or "append a record to a newline-delimited JSON file"; the
format inferred from the path can choose the operand schema.

References:

- [JSON Lines](https://jsonlines.org/)
- [NDJSON specification](https://github.com/ndjson/ndjson-spec)

## Impact

Spec:

- Add JSONL as a supported format for append-only mutation.
- Define JSONL validation, rendering, no-op, and malformed-line behavior.
- Define porcelain `append` arity for `.jsonl` and `.ndjson` paths.

Docs:

- Add `append` and `jsonl append` to help surfaces with append-only examples.
- Explain that `.jsonl` and `.ndjson` use the same adapter.

Code:

- Add `append` dispatch for `.jsonl` and `.ndjson` paths.
- Add `jsonl append` to the verb catalog.
- Add a JSONL adapter that validates existing lines and appends compact JSON.
- Add fixtures for empty files, files with and without trailing newlines,
  malformed existing lines, blank lines, non-object JSON values, `.ndjson`
  paths, porcelain arity, and compact rendering.

## Open Questions

- Should `ndjson append` be a spelling alias for `jsonl append`, or are
  extension inference and docs enough?
- Should Etch add an object-only mode for event logs, or leave schema
  conventions outside the command?
