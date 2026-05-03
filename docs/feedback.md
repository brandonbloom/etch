# Feedback Processing

The adopter feedback in this file has been triaged.

## Addressed

- ROUGH-10 / WISH-7: `etch help scripts` documents shell-style quoting, JSON
  values as single tokens, values with spaces, and heredocs.
- ROUGH-11: Markdown inline-field address parsing rejects `--body` combined
  with `--section`, `--item`, or `--task`.
- ROUGH-12: `task add` and `list add` no longer silently choose among multiple
  default list targets. Without `--section`, `--before`, or `--after`, insertion
  succeeds only when there is one obvious list target.
- WISH-5: `section replace` preserves existing blank-line boundaries around the
  replaced section body.

## Proposed

- WISH-1 is captured in [query commands](proposals/query-commands.md).
- WISH-2 is captured in
  [negative array indexes](proposals/negative-array-indexes.md).
- WISH-3 is captured in [JSONC support](proposals/jsonc-support.md).
- WISH-4 is captured in [JSON formatting options](proposals/json-formatting.md).
- WISH-6 is covered by
  [template-based creation](proposals/template-creation.md).
