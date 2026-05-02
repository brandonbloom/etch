---
status: deferred
depends_on: []
---

# Template-Based Creation

## Summary

Do not add template-based creation in the first post-MVP wave.

MVP `copy <src> <dst>` already covers exact tracked-template copying:

```sh
etch copy memory/TEMPLATE.md memory/programs/new-program.md
```

## Later Candidate

If adopters need placeholder expansion, propose a later command with no hidden
shell expansion:

```sh
etch create-from-template <template> <path> --values <json-object>
```

## Rationale

Template copying is useful, but it is not a core Etch gap unless the template
has structured placeholders. Placeholder syntax, escaping, defaults, and missing
values need a separate design pass.

## Impact

Spec:

- No change in the first post-MVP wave.
- If revisited, define `create-from-template` syntax, placeholder semantics,
  value parsing, and failure behavior.

Docs:

- Document `copy <src> <dst>` as the recommended tracked-template workflow.
- Add a deferred-design note for placeholder expansion.

Code:

- No implementation change unless exact `copy` proves insufficient.

## Open Questions

- Is exact tracked-template copying sufficient for adopter dossiers and
  programs?
- What placeholder syntax should be supported if expansion is needed?
- Should missing placeholder values fail, remain literal, or use defaults?
- Should template expansion be JSON-only, or should it understand frontmatter
  and Markdown sections?
