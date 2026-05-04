# Feedback pass

- [x] Cleanup rough edges from `docs/feedback.md`.
- [x] Implement no-brainer wishlist items.
- [x] Create proposals for more complex wishlist items.

# Documentation pass

- [x] Generate navigable website reference pages from CLI help topics.
- [x] Review/enhance/add help topics for the touched functionality.

# Follow-up

- Review [query commands](docs/proposals/query-commands.md).
- Review [negative array indexes](docs/proposals/negative-array-indexes.md).
- Review [JSONC support](docs/proposals/jsonc-support.md).
- Review [JSON formatting options](docs/proposals/json-formatting.md).
- Review the Markdown template additions in
  [template-based creation](docs/proposals/template-creation.md).

# 0.1.0 adopter feedback pass

- [x] Externalize the adopter report into proposal docs.
- [x] Review [format prefix safety](docs/proposals/format-prefix-safety.md).
- [x] Review [help and usage polish](docs/proposals/help-and-usage-polish.md).
- [x] Make the generated Reference command tables workflow-ordered.
- [x] Move the exhaustive Command Index below the explanatory Reference topics.
- [x] Tighten `README.md` into a GitHub landing page that links to the website
  for full docs.
- [x] Align Go and Git requirements across `README.md`, Overview, and
  Quickstart.
- [ ] Add a human "try it by hand" Quickstart path before agent bootstrap.
- [ ] Expand the Cheatsheet with workflow flags: `--plan`, `--dry-run`,
  `--no-checkout`, `--untracked`, `--message`, `--subject-prefix`,
  `--subject-suffix`, `--body-prefix`, and `--body-suffix`.
- [ ] Decide whether `--retries`, `--allow-empty`, and `--version` belong in the
  Cheatsheet or the Reference only.
- [ ] Add `etch prompt`, `etch prompt --bootstrap`, and
  `etch prompt --context` to the Cheatsheet.
- [ ] Name the MIT license on install-facing pages and where the site calls
  Etch open-source software.
- [ ] Decide whether to add a cookbook/examples page for 0.1.0 or defer it.

# Review gates

- [ ] Brandon reviews the Cheatsheet examples and behavior descriptions before
  the docs changes are finalized.
- [ ] Verify generated site/reference output after docs implementation.
