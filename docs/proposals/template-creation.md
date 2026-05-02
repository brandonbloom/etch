---
status: draft
depends_on: []
---

# Template-Based Creation

## Summary

Add a template mode to `create` for creating new files from tracked repository
templates. Exact template copying is a useful baseline; placeholder expansion
is the design work this proposal needs to settle.

## Candidate Commands

```sh
etch create --template <source> <dest>
etch create --template <source> <dest> --values <json-object>
```

Examples:

```sh
etch create --template memory/TEMPLATE.md memory/programs/new-program.md
etch create --template memory/programs/TEMPLATE.md memory/programs/spender-agent.md --values '{"name":"spender-agent"}'
```

## Semantics

- `create <path> <content>` remains raw content creation.
- `create --template <source> <dest>` creates `<dest>` from template
  `<source>`.
- `<source>` must be a tracked file in the transaction base.
- `<dest>` must not exist in the transaction base.
- Without `--values`, template creation is an exact copy with `create`
  destination semantics. It overlaps with `copy <source> <dest>`, but expresses
  template intent and leaves room for placeholder expansion.
- `--values` must be a strict JSON object.
- There is no shell, environment-variable, or command-substitution expansion.
- Missing placeholder values should fail.
- Placeholder expansion is text-only in the first version; frontmatter-aware or
  Markdown-section-aware template logic is out of scope.

## Placeholder Design To Decide

- Placeholder delimiter syntax.
- Escaping literal placeholder delimiters.
- Whether unused `--values` keys fail.
- Whether placeholders can specify defaults.
- Whether values render as strings only, or whether non-string JSON values have
  a deterministic rendering.

## Rationale

Template copying is already possible with `copy`, but repository templates are
a common enough creation workflow that `create --template` may be clearer for
agents and scripts. The hard part is not copying; it is making placeholder
expansion deterministic, auditable, and free of hidden shell behavior.

## Impact

Spec:

- Add command-local `--template` and `--values` flags to `create`.
- Define template source/destination admission rules.
- Define placeholder syntax, value rendering, escaping, and failure behavior.

Docs:

- Add examples for exact template creation and value-expanded creation.
- Document how `create --template` differs from `copy`.

Code:

- Extend `create` decoding for command-local `--template` and `--values`.
- Add a template expander once placeholder syntax is decided.
- Add fixtures for exact copy, destination-exists refusal, untracked template
  refusal, missing placeholder values, unused values, escaping, non-string
  values, and no shell/environment expansion.

## Open Questions

- What placeholder syntax should be supported if expansion is needed?
- Should unused `--values` keys fail?
- Should placeholders support defaults?
- How should non-string JSON values render?
- Should a future version understand frontmatter and Markdown sections, or
  should template expansion stay text-only?
