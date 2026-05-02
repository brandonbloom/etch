---
status: draft
depends_on:
  - markdown-addressing
---

# Markdown Fields

## Summary

Extend Markdown `set` and `delete` so a Markdown file behaves like a structured
data document:

- With no body-location flags, `set` targets YAML frontmatter.
- With body-location flags, `set` targets Dataview inline fields in the
  Markdown body.
- Dataview implicit fields, such as `file.name` and task `status`, are not
  mutable through this proposal.

This avoids new `field.*` or `frontmatter.*` syntax for the adopter-facing
surface while still allowing Etch to disambiguate where the field lives.

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

Use the existing `set` and `delete` primitives. Body-location flags imply
inline-field mutation.

```sh
etch set <path.md> <field> <value>
etch delete <path.md> <field>

etch set <path.md> <field> <value> [--body] [--section <heading>] [--item <literal>] [--task <literal>] [--bullet <literal>] [--after <literal>] [--before <literal>] [--hidden]
etch delete <path.md> <field> [--body] [--section <heading>] [--item <literal>] [--task <literal>] [--bullet <literal>] [--after <literal>] [--before <literal>]
```

Examples:

```sh
etch set memory/programs/spender-agent.md attention '"Driving"'
etch set memory/tasks/follow-up.md last "2026-05-01" --body
etch set memory/tasks/follow-up.md snooze "2026-05-06" --section Status
etch set memory/tasks/follow-up.md done "2026-05-01" --task "Send follow-up"
etch set memory/tasks/follow-up.md trace-id "abc123" --task "Send follow-up" --hidden
```

## Semantics

- For Markdown paths, `set <path> <field> <value>` without body-location flags
  mutates YAML frontmatter.
- The Dataview page implicit namespace `file.*` is reserved. This proposal does
  not add read/query commands, but `set note.md file.name ...` should fail
  rather than create frontmatter at `file.name`.
- `--body`, `--section`, `--item`, `--task`, `--bullet`, `--after`, and
  `--before` are body-location flags. Supplying any of them switches the target
  from frontmatter to Dataview inline fields.
- The body-location flags use the shared Markdown addressing rules in
  [Markdown Addressing](markdown-addressing.md).
- Etch parses full-line, bracketed, and parenthesized inline fields outside
  fenced code blocks, indented code blocks, inline code spans, and raw HTML
  blocks.
- Updating an existing inline field preserves its source form, field-name
  spelling, whitespace around `::`, and surrounding Markdown.
- Creating a page-level or section-level inline field uses full-line syntax.
  The placement flag for field creation is still open; candidate spellings are
  `--head` and `--tail` on `set`, rather than a shared `--at` flag.
- Creating an item-local inline field through `--item`, `--task`, or `--bullet`
  uses bracketed syntax.
- `--hidden` applies only when creating an inline field. It uses parenthesized
  syntax, which hides the field name in Obsidian reading mode while still
  rendering the value. The source text remains ordinary Markdown text, so other
  frontends may render the parentheses literally.
- Existing task/list date shorthands are conventions Etch should avoid
  duplicating accidentally, but shorthand mutation is out of scope here.
- With `--item`, `--task`, or `--bullet`, implicit fields such as `status`,
  `checked`, `completed`, `text`, and `blockId` are not writable through `set`;
  task/list structure changes belong to task/list operations.
- Repeated-key list editing is out of scope for this proposal.

## Rationale

This keeps the command surface simple: `set` remains the primitive, the field
operand is the Dataview field name, and location flags decide whether Etch
writes frontmatter or inline Markdown. The tradeoff is terminology precision:
internally, Dataview fields are key/value pairs, but `field` is the word users
see in Dataview and is clearer than asking them to set a "key" to a value.

Another tradeoff is that page-level inline fields need an explicit body-location
flag. Frontmatter remains the default because it is the least surprising
location for structured Markdown data, while body mutation should be opt-in.

## Deferred

- Repeated-key list editing, likely as `append` or `add` with body-location
  flags.
- Page-level and section-level inline field creation placement, including
  whether to use `--head`/`--tail`, operation-specific defaults, or only
  `--after`/`--before`.
- Task/list date shorthand mutation.
- Typed inline value rendering.
- `--near <literal>`, unless it is deterministic and ambiguity-safe.

## Impact

Spec:

- Change Markdown porcelain `set` so bare field names target frontmatter by
  default.
- Define body-location flags that switch Markdown `set` and `delete` to
  Dataview inline fields.
- Define Dataview-compatible parsing for full-line, bracketed, parenthesized,
  repeated-field, raw-field-name, canonical-field-name, and shorthand-detection
  behavior.
- Reserve Dataview implicit fields from mutation through this surface.

Docs:

- Add examples for frontmatter default, page-level inline fields, section-local
  fields, item-local fields, and hidden fields.
- Cross-link to Markdown addressing and frontmatter migration guidance.

Code:

- Extend Markdown `set` and `delete` decoding for field operands plus
  body-location flags.
- Add fixtures for default frontmatter mutation, inline body mutation,
  item-local fields, hidden fields, shorthand detection, anchor windows,
  ambiguous field names, repeated fields, code-block exclusion, and no-op
  deletion.

## Open Questions

- Should `--body` be the flag for page-level inline fields, or should that spell
  as `--inline`, `--head`, or `--tail` depending on insertion placement?
- Should Etch accept Dataview's sanitized field names as selectors, or only raw
  source field names? Accepting sanitized names would match Dataview query
  behavior, but it can make source mutation ambiguous when multiple raw
  spellings normalize to the same field name.
