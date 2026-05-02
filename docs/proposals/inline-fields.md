---
status: draft
depends_on:
  - markdown-addressing
  - value-syntax-and-assignment-items
---

# Markdown Fields

## Summary

Extend Markdown `set` and `delete` so a Markdown file behaves like a structured
data document:

- With no address flags, `set` targets YAML frontmatter.
- With address flags, `set` targets Dataview inline fields in the
  Markdown body.
- Dataview implicit fields, such as `file.name` and task `status`, are not
  mutable through this proposal.

This is a breaking change from the MVP Markdown selector model, which required
`frontmatter.*` to mutate frontmatter through porcelain `set`. The new model
treats a Markdown file as a structured document whose default writable field
location is frontmatter, while address flags opt into body-local inline fields.

## Dataview Model

Dataview calls queryable metadata "fields". Page fields can come from
frontmatter, inline fields, or implicit `file.*` fields. Tasks and list items
inherit page fields, can carry item-local inline fields, and expose their own
implicit fields such as `status`, `checked`, `completed`, `text`, `line`,
`section`, `task`, `parent`, and `blockId`.

Inline field source forms include:

- Full-line fields: `Key:: Value`
- Bracketed inline fields: `[key:: value]`
- Parenthesized inline fields: `(key:: value)`

Dataview also recognizes task/list date shorthand conventions for `due`,
`completion`, `created`, `start`, and `scheduled`. Those shorthands are useful
context, but this proposal writes ordinary Dataview inline fields.

References:

- [Adding Metadata](https://blacksmithgu.github.io/obsidian-dataview/annotation/add-metadata/)
- [Metadata on Pages](https://blacksmithgu.github.io/obsidian-dataview/annotation/metadata-pages/)
- [Metadata on Tasks and Lists](https://blacksmithgu.github.io/obsidian-dataview/annotation/metadata-tasks/)

## Candidate Commands

Use the existing `set` and `delete` primitives. Address flags imply inline-field
mutation.

```sh
etch set <path.md> <field> <value>
etch delete <path.md> <field>

etch set <path.md> <field> <value> [address flags] [--hidden]
etch delete <path.md> <field> [address flags]
```

Address flags are the optional Markdown addressing flags defined in
[Markdown Addressing](markdown-addressing.md), such as `--body`, `--section`,
`--item`, `--item-type`, `--task`, `--after`, and `--before`.

Examples:

```sh
etch set memory/programs/spender-agent.md attention '"Driving"'
etch set memory/tasks/follow-up.md last "2026-05-01" --body
etch set memory/tasks/follow-up.md snooze "2026-05-06" --section Status
etch set memory/tasks/follow-up.md done "2026-05-01" --task "Send follow-up"
etch set memory/tasks/follow-up.md trace-id "abc123" --task "Send follow-up" --hidden
```

## Semantics

- For Markdown paths, `set <path> <field> <value>` without address flags
  mutates YAML frontmatter.
- Porcelain Markdown `frontmatter.*` selectors are replaced by the bare
  frontmatter-default form. `set note.md frontmatter.title ...` should fail
  rather than address frontmatter or create a field named `frontmatter.title`.
  Frontmatter-specific plumbing can remain available as
  `frontmatter set <path> <selector> <value>`.
- The Dataview page implicit namespace `file.*` is reserved. This proposal does
  not add read/query commands, but `set note.md file.name ...` should fail
  rather than create frontmatter at `file.name`.
- Address flags switch the target from frontmatter to Dataview inline fields in
  the Markdown body. Illegal or contradictory address flag combinations are
  handled by the shared Markdown addressing rules.
- Etch parses full-line, bracketed, and parenthesized inline fields outside
  fenced code blocks, indented code blocks, inline code spans, and raw HTML
  blocks.
- Updating an existing inline field preserves its source form, field-name
  spelling, whitespace around `::`, and surrounding Markdown.
- Updating or deleting an inline field resolves field names in two steps:
  first exact raw source field-name match, then Dataview-normalized field-name
  match if no raw match exists.
- Dataview normalization lowercases the field name, replaces whitespace with
  `-`, removes built-in Markdown formatting from field keys, and removes
  punctuation that Dataview excludes from simplified names. `_` is not a
  whitespace alias, so `last-run` can match `[last run:: ...]` but `last_run`
  cannot.
- A Dataview-normalized match must resolve to exactly one existing inline
  field. If multiple source field names normalize to the requested name, Etch
  fails and reports candidate locations rather than choosing one.
- Creating an inline field uses the `<field>` operand literally as the source
  field name.
- Creating a page-level or section-level inline field uses full-line syntax.
  Placement uses the shared Markdown addressing rules. If no placement flag is
  supplied, the command should use an operation-specific default.
- Creating an item-local inline field through `--item` or `--task` uses
  bracketed syntax.
- `--hidden` applies only when creating an inline field. It uses parenthesized
  syntax, which hides the field name in Obsidian reading mode while still
  rendering the value. The source text remains ordinary Markdown text, so other
  frontends may render the parentheses literally.
- Existing task/list date shorthands are conventions Etch should avoid
  duplicating accidentally, but shorthand mutation is out of scope here.
- With `--item` or `--task`, implicit fields such as `status`,
  `checked`, `completed`, `text`, and `blockId` are not writable through `set`;
  task/list structure changes belong to task/list operations.
- Repeated-key list editing is out of scope for this proposal.

## Rationale

This keeps the command surface simple: `set` remains the primitive, the field
operand is the Dataview field name, and address flags decide whether Etch
writes frontmatter or inline Markdown. The tradeoff is terminology precision:
internally, Dataview fields are key/value pairs, but `field` is the word users
see in Dataview and is clearer than asking them to set a "key" to a value.

Another tradeoff is that this changes the MVP Markdown selector model. The new
porcelain behavior is intentionally simpler: bare Markdown fields target
frontmatter, and body-local inline fields require address flags.

## Deferred

- Repeated-key list editing, likely as `append` or `add` with address
  flags.
- Page-level and section-level inline field creation placement defaults.
- Task/list date shorthand mutation.
- Typed inline value rendering.
- `--near <literal>`, unless it is deterministic and ambiguity-safe.

## Impact

Spec:

- Change Markdown porcelain `set` and `delete` so bare field names target
  frontmatter by default and `frontmatter.*` is no longer a porcelain selector
  namespace.
- Define address flags that switch Markdown `set` and `delete` to Dataview
  inline fields.
- Define Dataview-compatible parsing for full-line, bracketed, parenthesized,
  repeated-field, raw-field-name, Dataview-normalized field-name, and
  shorthand-detection behavior.
- Reserve Dataview implicit fields from mutation through this surface.

Docs:

- Add examples for frontmatter default, page-level inline fields, section-local
  fields, item-local fields, and hidden fields.
- Cross-link to Markdown addressing and `etch help fields`.

Code:

- Extend Markdown `set` and `delete` decoding for field operands plus
  address flags.
- Implement Dataview field-name normalization as an independently unit-tested
  helper, separate from inline-field parsing and mutation.
- Add fixtures for default frontmatter mutation, inline body mutation,
  item-local fields, hidden fields, shorthand detection, anchor windows,
  exact raw field-name matching, Dataview-normalized field-name matching,
  normalized-name collisions, repeated fields, code-block exclusion, and no-op
  deletion.

## Open Questions

- Should `--body` be the flag for page-level inline fields, or should that spell
  as `--inline`?
