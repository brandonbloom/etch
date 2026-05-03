---
status: draft
depends_on: []
---

# Negative Array Indexes

## Summary

Extend the existing singular selector subset with negative array indexes such
as `[-1]` for "last element", without admitting wildcards, slices, filters, or
multi-match behavior.

## Candidate Syntax

```sh
etch set state.json items[-1].status complete
etch delete state.json items[-1]
etch get state.json items[-1]
```

## Semantics

- Accept negative integer indexes in bracket notation only.
- `[-1]` resolves to the final array element, `[-2]` to the penultimate element,
  and so on.
- Negative indexes require the addressed array to exist during evaluation.
- Negative indexes never create array elements.
- Negative indexes cannot be used for append semantics. `set items[-1] value`
  updates an existing element only.
- Out-of-range negative indexes are zero-match or out-of-range errors according
  to the verb's existing array rules.
- Wildcards, recursive descent, slices, filters, unions, and functions remain
  rejected.

## Plan Representation

Negative indexes are document-relative selectors, so canonical planning needs a
clear representation:

- Option A: Preserve the caller selector in the plan and add a resolved
  non-negative selector field.
- Option B: Normalize to the resolved non-negative selector only.
- Option C: Reject negative indexes for mutating plans until plan schema
  explicitly records caller intent and resolved target.

Option A best preserves auditability, but it changes plan shape.

## Rationale

Adopter workflows often append records, events, or history entries and then
want to update the most recent item. `[-1]` is a familiar shorthand for that
case. The risk is transaction stability: Etch retries against a new `HEAD`, and
the final item may change between attempts. That makes resolved-target
reporting more important than ordinary positive indexes.

## Impact

Spec:

- Update the selector subset to include negative integer indexes.
- Define retry behavior and canonical plan representation.
- Define creation/update limits for negative indexes.

Docs:

- Add examples in selector help.
- Explain that negative indexes are resolved against the transaction input.

Code:

- Extend selector parsing/normalization.
- Resolve negative indexes inside JSON, YAML, and frontmatter adapters.
- Add plan/hash tests proving stable canonical behavior.
- Add retry tests where array length changes between attempts.

## Open Questions

- Is `[-1]` enough, or should all negative integer indexes be accepted?
- Should canonical selector normalization preserve negative syntax before
  document resolution?
- Should retries re-resolve `[-1]` against the new base, or detect that the
  originally resolved element moved?
