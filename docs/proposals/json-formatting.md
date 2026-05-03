---
status: draft
depends_on: []
---

# JSON Formatting Options

## Summary

Add explicit formatting controls for generated JSON while preserving Etch's
localized-edit contract for existing JSON source.

## Candidate Commands

```sh
etch create state.json --json '{}' --pretty
etch create state.json --json '{}' --indent 2
etch set state.json payload --json '{"a":1}' --pretty
```

## Semantics

- Formatting flags affect generated JSON only, not untouched source ranges.
- Existing JSON files keep their local formatting contract: targeted structural
  edits should not reformat the whole document.
- `create` and missing-file structured writes are the first places to support
  pretty output because there is no existing style to preserve.
- `--pretty` renders generated JSON with a default indentation, likely two
  spaces.
- `--indent <n>` accepts a small positive integer and implies `--pretty`.
- `.editorconfig` detection is deferred until explicit command flags work.
- JSONL remains compact one-record-per-line and does not accept pretty output.

## Rationale

Fresh JSON files currently use compact single-line output. That is easy for
machines, but many repositories prefer human-readable JSON for tracked config
or state files. Formatting controls are valuable, but they must not become a
back door for reformatting existing files during unrelated mutations.

## Impact

Spec:

- Add formatting flags to generated JSON paths.
- Define which commands and adapters honor them.
- Define conflicts with JSONL and non-JSON formats.

Docs:

- Add examples for pretty `create` and generated missing-file structured writes.
- Explain that existing files preserve local source formatting.

Code:

- Extend command-local flag decoding for the chosen commands.
- Carry formatting options through operations or adapter calls.
- Add tests for create, missing-file set, existing-file localized edits, JSONL
  rejection, and plan descriptors.

## Open Questions

- Should formatting flags belong to `create`, structured verbs, or both?
- Should `--pretty` default to two spaces, or should it use tabs/width from a
  project convention?
- Should generated JSON nested inside YAML/frontmatter obey JSON formatting
  flags, or should YAML rendering remain independent?
- Should formatting options be represented in canonical plans?
