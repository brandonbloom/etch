---
title: etch
---

## What etch does

etch turns small structured changes into one command and one commit.

It edits structured text by intent: set a field, replace a section, close a task, update a table, append a log. Every successful mutating call writes the file and commits the result.

<pre class="demo"><span class="dim"># Mark a task as complete in a markdown file</span>
<span class="prompt">$</span> etch set tasks/onboarding.md status complete

<span class="dim">[main a1f9c3e] etch set tasks/onboarding.md status</span>
<span class="dim"> 1 file changed, 1 insertion(+), 1 deletion(-)</span></pre>

One command. The frontmatter field is updated, the resulting bytes are written, and git history gets a deterministic, reviewable commit. No patch construction, no staging, no commit message to write.

## Built for snapshot-style knowledge repos

etch is for repos where small commits are the operating model: agent home directories, personal and team wikis, markdown task trackers, people/CRM directories, and config-as-data repos.

In these repos, git is the audit log. Every meaningful mutation should be small, explicit, and committed.

etch supports common knowledge-repo formats including Markdown, JSON, JSONL/NDJSON, YAML, and CSV, with focused operations for fields, sections, tasks, lists, tables, files, and append-only logs.

## One operation instead of a tool choreography

Without etch, an agent usually has to inspect the file, construct an edit, apply it, inspect again, stage, inspect git state, and commit. Even when everything goes right, that is more tool calls than the actual change deserves. When it goes wrong, the agent is debugging patches instead of editing knowledge.

etch collapses the edit, verification surface, and commit into the operation itself.

## Concurrent by design

Multiple agents and humans can work in the same repo without a lock server. etch reads committed HEAD, plans the edit, commits with compare-and-swap, and retries if another writer moved the branch.

Everyone keeps rolling forward through git history.

## Install

```sh
go install github.com/brandonbloom/etch/cmd/etch@latest
```

Requires Go 1.25.3+ and Git. Then see the [quickstart]({{< ref "/quickstart" >}}) to set up etch in your project.
