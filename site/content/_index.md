---
title: etch
---

## The problem

Agents do mechanical edits through general-purpose patch tools: construct a diff, apply it, hope the whitespace is right, stage the file, write a commit message. Each step burns tokens and introduces failure modes. Hallucinated diffs. Off-by-one line numbers. Quoting mistakes.

The edits themselves are boringly predictable: *set this frontmatter field, delete that YAML key, replace this markdown section, append to that list.* For structured, mechanical mutations on known formats, a diff is the wrong level of abstraction.

## What etch does

etch is a small CLI for mechanical mutations to text and data files. It knows about a fixed set of formats --- Markdown, JSON, JSONL/NDJSON, YAML, CSV --- and a fixed set of operations on them. Every successful mutating call becomes a git commit.

<pre class="demo"><span class="dim"># Mark a task as complete in a markdown file</span>
<span class="prompt">$</span> etch set tasks/onboarding.md status complete

<span class="dim">[main a1f9c3e] etch set tasks/onboarding.md status</span>
<span class="dim"> 1 file changed, 1 insertion(+), 1 deletion(-)</span></pre>

One command. The frontmatter field is updated and the result is committed. No patch construction, no staging, no commit message to write.

## Designed for agent home directories

A new kind of repository is emerging: the **agent home directory**. Not a codebase that ships to production, but a git-backed knowledge store where agents and humans collaborate on structured data. Personal knowledge bases, task trackers, CRM directories, config-as-data repos.

In these repos, git is the database engine. Commits are the transaction log. Every small mutation should be its own commit, because git history *is* the audit trail. etch makes this workflow reliable and token-efficient.

## Install

```sh
go install github.com/brandonbloom/etch/cmd/etch@latest
```

Requires Go 1.25.3+ and Git. Then see the [quickstart]({{< ref "/quickstart" >}}) to set up etch in your project.
