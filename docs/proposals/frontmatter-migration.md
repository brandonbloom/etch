---
status: draft
depends_on:
  - inline-fields
---

# Frontmatter Migration Support

## Summary

Support adopter migrations that move note-global metadata from Markdown body
text into YAML frontmatter. This proposal does not add a new mutation
primitive; it defines documentation, examples, and validation expectations
around existing frontmatter commands.

## Recommendation

- Store note-global, schema-like metadata in YAML frontmatter when the field's
  location in body text has no meaning.
- Examples include attention level, DRI, owner, source system, and stable
  identifiers.
- Keep Dataview inline fields when the field is intentionally attached to a
  paragraph, list item, or local note context.
- Use Markdown `set`, `add`, and `remove` frontmatter behavior for migrated
  fields.

Example:

```sh
etch set memory/programs/spender-agent.md attention '"Driving"'
```

## Impact

Spec:

- Clarify that bare Markdown `set <path.md> <field> <value>` targets
  frontmatter by default.
- Cross-link this guidance from the Markdown fields proposal.
- No new verbs or selector syntax.

Docs:

- Add a short "frontmatter or inline field?" guide with adopter examples.
- Show before/after examples for migrating note-global body metadata to
  frontmatter.
- Document when not to migrate: metadata attached to a paragraph, task, or list
  item should remain inline.

Code:

- No new implementation is required if Markdown frontmatter `set`, `add`, and
  `remove` cover the migration targets.
- Add fixtures only if the frontmatter default changes as part of the Markdown
  fields proposal.

## Rationale

Etch should make structured metadata easy to maintain, but it should not infer
an adopter's data model. Moving note-global metadata into frontmatter is an
adopter migration choice. Etch's role is to make the resulting representation
deterministic and easy to update.

## Open Questions

- Which adopter fields are note-global metadata rather than body-local metadata?
- Should adopters provide migration scripts from body fields to frontmatter?
- Should Etch add validation helpers for required frontmatter schemas, or is
  that out of scope?
