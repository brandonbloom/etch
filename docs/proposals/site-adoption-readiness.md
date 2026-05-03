---
status: draft
depends_on: []
---

# Site Adoption Readiness

## Summary

Improve install-facing docs and website pages so adopters can evaluate Etch
without reading the full reference or using an agent on the first attempt.

The adopter report praised the four-page site structure and generated
Reference, but found gaps in install consistency, human Quickstart flow,
Cheatsheet coverage, license visibility, and long-page navigation.

## Phase 1: Adoption Blockers And Rough Edges

- Verify the intended minimum Go version from `go.mod`, toolchain use, and CI,
  then make README, Overview, and Quickstart state the same Go and Git
  requirements.
- Add a "try it by hand" Quickstart path before the agent bootstrap flow. The
  manual path should use a small repository, one structural mutation, and either
  `git show` or `--dry-run` so the user sees what Etch commits.
- Expand the Cheatsheet with global workflow flags:
  `--plan`, `--dry-run`, `--no-checkout`, `--untracked`, `--message`,
  `--subject-prefix`, `--subject-suffix`, `--body-prefix`, and
  `--body-suffix`.
- Decide whether to include `--retries`, `--allow-empty`, and `--version` in
  the Cheatsheet or leave them to the Reference.
- Add `etch prompt`, `etch prompt --bootstrap`, and `etch prompt --context` to
  the Cheatsheet because they are central to agent onboarding.
- Name the MIT license on install-facing pages and anywhere the site calls Etch
  open-source software.

## Phase 2: Site Enhancements

- Evaluate a cookbook/examples page that adapts the README examples for site
  navigation: JSON, JSONL, frontmatter, inline fields, tasks, sections, CSV,
  Markdown tables, guards, and scripts.
- Evaluate a version indicator or changelog page so readers know which docs
  match which release.
- Improve Reference navigation with sticky local navigation, back-to-top links,
  or search.

## Cheatsheet Review Gate

Before implementation is called done, Brandon should review the Cheatsheet
examples and behavior descriptions. The review should check:

- Examples use commands Brandon wants to recommend to adopters.
- Behavior descriptions match implemented semantics and help topics.
- Global flags are complete enough for daily use without duplicating the full
  Reference page.
- The `prompt` examples steer agents toward durable repo guidance without
  overexplaining the workflow.

## Rationale

The website already has a good shallow-to-deep structure. The next pass should
make the first successful human experience as direct as the first agent
experience, and it should put adoption-critical facts where evaluators look:
requirements, license, safe preview flags, and examples.

## Impact

Docs:

- Update README, Overview, Quickstart, Cheatsheet, and footer text as needed.
- Keep generated Reference content sourced from CLI help.

Code:

- Site content and layout updates may be enough for Phase 1.
- Phase 2 search or sticky navigation may require CSS or template changes.

Validation:

- Run the site generation task.
- Run any cheatsheet fixture validation.
- Review generated pages in a browser or rendered build output.

## Open Questions

- Should the minimum Go version stay exactly at `go 1.25.3`, or can it be
  lowered after testing?
- Should the Cheatsheet include every global flag or only the workflow flags
  users reach for during normal editing?
- Should the cookbook be a standalone page or part of Quickstart?
