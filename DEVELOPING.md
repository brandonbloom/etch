# Development

This file collects project workflow notes for human contributors and coding
agents.

## Command Changes

When changing command syntax, command behavior, help text, or command metadata,
update the published-reference sources in the same change. Check the README,
`spec.md`, committed help snapshots under `internal/etch/testdata/help/`, and
the website inputs under `site/`, including the Reference example fixtures in
`site/fixtures/`.

Regenerate help snapshots after changing help output:

```sh
mise run help:snapshots
```

The website Reference page is derived from `etch help --json`; the committed
`internal/etch/testdata/help/reference.json` snapshot is the review surface for
that shared help model. `mise run site` copies that snapshot into Hugo's data
directory, verifies the site fixtures, and builds the site.

## Verification

Run the Go test suite before handing off command or behavior changes:

```sh
go test ./...
```
