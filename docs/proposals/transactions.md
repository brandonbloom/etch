---
status: draft
depends_on: []
---

# Transaction Batching

## Summary

Design a multi-execution transaction model before recommending Etch for
high-frequency adopter events such as heartbeats, stats, ingestion steps, and
note touches.

MVP `run` already batches operations when a script can be generated up front.
Adopters also have session-shaped workflows where mutations happen over time
and where commit-per-invocation could be too noisy.

## Candidate Command Family

```sh
etch tx begin
etch tx <id> -- <ordinary etch command...>
etch tx <id> -- run <script>
etch tx plan <id>
etch tx commit <id> --message <message>
etch tx abort <id>
```

## Semantics

- A transaction stores an accumulated virtual tree derived from `HEAD`.
- Ordinary Etch commands can be applied to that virtual tree through the
  transaction handle.
- Later operations in the transaction see earlier Etch mutations without reading
  dirty checkout files.
- `tx plan` renders the pending final plan.
- `tx commit` creates one git commit for the accumulated tree.
- `tx abort` discards transaction state.
- The ref update remains the durability boundary.
- Materialization happens after `tx commit`, not after each staged operation.

## Rationale

Batching reduces commit noise while preserving Etch's plan/commit model. It
also gives scripts a clean virtual base state, which is preferable to a
checkout-sourced mutation mode.

Reading dirty tracked files as inputs would weaken Etch's central safety
property and could accidentally commit unfinished LLM prose. Transaction
batching gives adopters the ergonomic benefit without changing the input model.

## Impact

Spec:

- Move the multi-execution transaction sketch from `spec.md` into concrete
  command semantics.
- Define transaction IDs, storage, locking, crash cleanup, plan hashing,
  retries, materialization, and top-level flag behavior.

Docs:

- Extend `verbs --json` and help surfaces for transaction commands.
- Add recovery notes for stale and abandoned transactions.

Code:

- Add transaction state management, virtual-tree planning, and commit execution.
- Add fixtures for begin/apply/plan/commit/abort, stale transaction bases,
  concurrent ref updates, crash cleanup, dirty checkout materialization, and
  nested or overlapping transactions if supported.

## Open Questions

- Is `etch run -` enough batching for the first adopter integration, or is
  `etch tx` required before adoption?
- Where should transaction state live?
- Should there be a porcelain current-transaction context, or only explicit
  transaction IDs?
- How should approval caches interact with transaction plans?
