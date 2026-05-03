# Development

This file collects project workflow notes for human contributors and coding
agents.

## Command Changes

When changing command syntax, command behavior, help text, or command metadata,
update the published-reference sources in the same change. Check the README,
`spec.md`, relevant help tests, and the website inputs under `site/`, including
the cheatsheet fixtures in `site/fixtures/`.

## Verification

Run the Go test suite before handing off command or behavior changes:

```sh
go test ./...
```
