---
status: draft
depends_on: []
---

# JSONC Support

## Summary

Add support for JSON-with-comments configuration files without weakening strict
JSON value operands or turning Etch into a whole-document formatter.

## Candidate Commands

```sh
etch set settings.jsonc editor.tabSize --json 2
etch jsonc set settings.json editor.tabSize --json 2
etch jsonc delete tsconfig.json compilerOptions.noUnusedLocals
```

## Semantics

- Prefer JSONC over full JSON5 for the first version: comments and trailing
  commas cover the common configuration-file cases while avoiding new value
  syntax.
- Porcelain infers JSONC for `.jsonc` paths.
- Porcelain inference for `.json` paths remains strict JSON unless an explicit
  and documented list of known JSONC files is approved.
- `jsonc` plumbing parses comments and trailing commas regardless of extension.
- JSONC input files preserve untouched comments, whitespace, key order, and
  trailing-comma style.
- JSON value operands remain strict JSON. Accepting JSONC in files does not
  imply accepting JSONC/JSON5 as command-line value syntax.
- Initial verbs should match JSON's structured mutation surface only when the
  parser can expose safe localized rewrites.

## Preservation Contract

JSONC support must preserve:

- comments before and after members
- inline comments
- blank lines
- object member order
- array element order
- indentation around untouched members/elements
- trailing comma presence where untouched
- numeric and string spelling for untouched values

Whole-document reformatting is a feature failure.

## Parser Acceptance Gate

Before JSONC support lands, candidate parser/rewriter integration must pass
fixtures for:

- updating an existing scalar
- creating a missing final member in an existing object
- deleting an existing member
- no-op deletion of a missing member
- comments before, after, and inline with members
- trailing commas in objects and arrays
- nested object and array edits
- malformed JSONC refusal without rewriting
- strict JSON value operands that contain comment-like strings

## Rationale

Files such as VS Code settings, `tsconfig` variants, and tool configuration
often use comments or trailing commas while retaining the JSON data model.
Etch should handle those files only if it can keep the human-authored
comment/formatting structure intact.

## Impact

Spec:

- Add JSONC as a format adapter.
- Define porcelain `.jsonc` inference and `jsonc` plumbing commands.
- Define strict JSON operand behavior.

Docs:

- Add JSONC examples and unsupported JSON5 syntax notes.
- Explain when `.json` remains strict JSON.

Code:

- Add parser/rewriter integration after the acceptance gate passes.
- Add catalog entries for `jsonc set/delete/append/add/remove` only for verbs
  that satisfy preservation fixtures.
- Add malformed input, preservation, porcelain/plumbing, and value-operand
  tests.

## Open Questions

- Which parser exposes enough source ranges for localized rewrites?
- Should any `.json` basenames infer JSONC, or should `.json` stay strict unless
  callers use `jsonc` plumbing?
- How should inserted members choose trailing-comma style?
- Is full JSON5 value syntax worth a separate proposal?
