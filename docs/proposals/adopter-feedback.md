---
status: draft
depends_on: []
---

# Adopter Feedback Proposals

This index collects beyond-MVP proposals derived from adopter feedback. Each
child proposal is meant to be approved, rejected, or revised on its own before
any `spec.md` or code changes land.

## Context

The adopter is a git-backed second-brain application with many mechanical file
mutations that should not require LLM-authored read-modify-write edits:

- JSON watermark and cache files.
- Daily Markdown logs with stable headings.
- Dataview-style inline fields in Markdown notes, tasks, and list items.
- Markdown sections that accumulate ingested bullets or action items.
- JSON stats files and JSONL event streams.
- TOML configuration files.
- Session-shaped workflows that make many tiny mutations.

MVP Etch already covers several adopter needs:

- JSON cache and stats object updates via JSON `set`, `append`, `add`,
  `delete`, and `remove`.
- Whole-section daily log replacement, renamed here to `section replace`.
- Calendar Markdown tables through table row and cell commands.
- Committed-state guards through `exists`, `missing`, and `contains`.
- Note-global Markdown metadata when the adopter stores it in YAML frontmatter.

## Review Set

Review these in priority order:

1. [Markdown fields](inline-fields.md)
2. [Markdown addressing](markdown-addressing.md)
3. [Value syntax and assignment items](value-syntax-and-assignment-items.md)
4. [Markdown block IDs](block-ids.md)
5. [Markdown section insertion and block whitespace](section-insertion.md)
6. [Transaction batching](transactions.md)
7. [Markdown task and list operations](task-list-ops.md)
8. [JSONL append](jsonl-append.md)
9. [TOML set/delete](toml-set-delete.md)
10. [Template-based creation](template-creation.md)
11. [Query commands](query-commands.md)
12. [Negative array indexes](negative-array-indexes.md)
13. [JSONC support](jsonc-support.md)
14. [JSON formatting options](json-formatting.md)

The first two proposals are the highest-leverage adopter-driven additions.
Transaction batching is listed before task/list operations because commit
volume and HEAD-sourced virtual state affect how comfortable adopters can be
with frequent script-driven mutations.

Related guidance: `etch help fields` and `etch help scripts`.

## 0.1.0 Release-Readiness Feedback

The `Etch 0.1.0 -- Feedback Report` raised a separate set of release-readiness
issues. These are not new adopter mutation families. Safety and help behavior
changes remain proposals to review before implementation:

1. [Format prefix safety](format-prefix-safety.md)
2. [Help and usage polish](help-and-usage-polish.md)

Website, README, Reference/help, and release-readiness review tasks live in
`PLAN.md` because they are execution work, not Etch behavior proposals.

## Shared Goals

- Replace LLM-authored mechanical edits with deterministic Etch commands.
- Preserve Etch's HEAD-sourced transaction model.
- Keep each new command structural and explainable in `etch help`.
- Prefer exact selectors and ambiguity errors over fuzzy matching.
- Make boundary whitespace deterministic for Markdown insertion commands.
- Add format support only where Etch can provide reviewable behavior without
  becoming a generic text editor.

## Shared Non-Goals

- No regex replace command.
- No generic "edit arbitrary Markdown span" command.
- No checkout-sourced mutation mode for tracked files.
- No mutation of gitignored signal files in the default model.
- No adopter-domain commands such as `dossier set-dri` or `program touch`.

## Deferred or Rejected

- **Checkout-sourced mutation mode:** reject from this proposal set. It
  undermines Etch's guarantee that commits contain only the requested
  structural mutation from `HEAD`.
- **Gitignored signal files:** keep out of scope. Scripts can use ordinary shell
  tools for ephemeral files.
- **Generic Markdown span editing:** reject. Inline fields, sections, tables,
  tasks, and list items are enough structure to keep operations reviewable.
- **JSONL updates other than append:** defer unless a concrete repair workflow
  appears.

## Integration Success

Adopter integration is successful when these mutation families no longer need
LLM-authored file patches:

- Set JSON watermark/cache fields.
- Update Dataview inline fields such as `[last:: ...]`, `[done:: ...]`, and
  `[snooze:: ...]`.
- Append ingestion bullets under stable Markdown headings.
- Replace or append the daily `## Heartbeat` section.
- Toggle action-item checkboxes without rewriting surrounding prose.
- Append newline-delimited JSON event records.
