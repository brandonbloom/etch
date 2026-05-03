---
title: The agent home directory pattern
description: Why a new kind of repo is emerging and what makes it different.
weight: 1
---

A new kind of repository is emerging. Not a codebase that ships to production, but a **git-backed knowledge store** where agents and humans collaborate on structured data.

These repos look like:

- **Personal knowledge bases** --- daily logs, notes, and dossiers in markdown with rich frontmatter
- **Task trackers** --- markdown files where `status: complete` or `[snoozed:: 2026-05-10]` are real, queryable data
- **People/CRM directories** --- a folder of markdown files, one per person, with structured metadata and append-only interaction logs
- **Config-as-data repos** --- YAML and JSON files that represent system state, not just system configuration
- **Wiki-style collaboration** --- multiple agents and humans editing overlapping files, always committing, always rolling forward

## Git as a database

In these repos, git isn't version control for code --- it's the **database engine**. Commits aren't milestones; they're the transaction log. Every small mutation should be its own commit, because git history *is* the audit trail.

The filesystem conventions --- frontmatter fields, inline tags like `[due:: 2026-03-01]`, section headers as stable anchors --- form a lightweight schema that agents navigate constantly.

## Always commit, always roll forward

A single agent session might mark a task complete, add a snooze date to an action item, update a person's contact info, and append to a daily log. Each one should be its own auditable commit.

This is the workflow etch is built for. Instead of constructing patches for each of these mechanical edits, etch provides format-aware operations that land as atomic commits.

## Why patches are the wrong tool

Today, agents do these edits through general-purpose patch tools: construct a diff, apply it, hope the whitespace is right, stage the file, write a commit message. Each step burns tokens and introduces failure modes:

- Hallucinated diffs
- Off-by-one line numbers
- Trailing newline mismatches
- Quoting errors in YAML or JSON
- Corrupted frontmatter

The edits themselves are boringly predictable. You wouldn't use `sed` to set a JSON key. So why use a patch to update a frontmatter tag?

Reserve patches for the cases that actually need them --- novel edits to code, prose rewriting, complex refactors. The mechanical stuff should be a solved problem.
