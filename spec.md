# etch — Design

> Status: v0, in progress. Captures decisions made through initial design discussion. Verb surface and error taxonomy are explicitly deferred.

## 1. What etch is

A small CLI for **mechanical mutations to text and data files**, where each successful mutating invocation becomes a git commit by default. Operations are atomic (per op) and transactional (per invocation): all the operations in one mutating invocation either land as a single commit or none of them do.

The primary audience is agentic coding tools operating against git-backed wiki-style repositories — multiple agents and humans iterating forward, where git history is the transaction log and mechanical edits ("set this frontmatter field," "replace this markdown section," "delete this JSON key") are common enough that doing them through a general-purpose patch tool burns tokens and risks subtle errors. etch trades generality for precision: it knows about a small, fixed set of file formats and a small, fixed set of operations, and it does them quickly and predictably.

The headline product is a binary. There may eventually be a Go library underneath suitable for embedding; that question is deferred.

## 2. Mental model

Three principles:

1. **Commits are the default contract for mutating verbs.** A successful mutating etch invocation produces a commit unless the caller explicitly opts out with `--no-commit`, or targets untracked/non-git paths with `--untracked`. Read verbs are a separate class: they print results and never commit. The default remains that the commit is the whole point.

2. **The script line and the CLI argv are the same thing.** A line in an etch script is the same tokens you'd type at the shell, modulo the binary name. This symmetry is the primary ergonomic property. It means agents can prototype on the command line and concatenate script files without translation, and it means the help page documents both surfaces simultaneously.

3. **No escape hatches inside the DSL.** The script syntax has no variables, no expansions, no subshells, no pipes, no conditionals. Composition happens at the shell level, *outside* etch. This keeps etch's own blast radius statically bounded and makes it a narrow permission target.

## 3. Invocation surface

Verb-first, with a small set of top-level flags.

```
etch <verb> [args...]                  one-shot
etch run <script>                      multi-op transaction from script
etch --plan <verb> [args...]           emit JSON plan, do not execute
etch --plan run <script>               emit JSON plan for batch
etch --dry-run <verb> [args...]        emit git-am-compatible patch
etch help [verb]                       human-readable help
etch verbs --json                      machine-readable verb catalog
etch --help                            terse one-screen reference
```

For `run`, `<script>` is a path to an etch script. The conventional path `-` means stdin.

Top-level flags:

| Flag | Env | Effect |
|---|---|---|
| `--plan` | — | Emit JSON plan to stdout, do not write or commit. |
| `--dry-run` | — | Emit git-am-compatible patch to stdout, do not write or commit. |
| `--no-commit` | — | Apply changes to working tree, skip commit. |
| `--no-checkout` | — | After committing, do not materialize touched paths into the working tree. Invalid with `--no-commit`. |
| `--untracked` | `ETCH_UNTRACKED=1` | Permit target paths that are not tracked by git. Outside a git repo, run as a pure working-tree mutator. |
| `--message <m>` | — | Override auto-generated commit message. Invalid with `--no-commit`. |
| `--message-prefix <m>` | — | Prepend to auto-generated message. Invalid with `--no-commit`. |
| `--message-suffix <m>` | — | Append to auto-generated message. Invalid with `--no-commit`. |
| `--retries <n>` | — | Retry budget on optimistic-concurrency conflict. `-1` = retry forever. Default `3`. |
| `--allow-empty` | — | Permit a commit with no content change. Invalid with `--no-commit`. |

`--plan`, `--dry-run`, and `--no-commit` are mutually exclusive. The first two skip execution entirely; the third writes but doesn't commit. `--no-checkout` applies only to successful committing invocations; it has no meaning with `--plan`, `--dry-run`, or `--no-commit`. Read verbs do not accept mutation or commit-control flags, because they have no write, plan, commit, or materialization phase.

Inside a git repo, `--untracked` permits existing untracked target paths to participate in the transaction; those paths can become tracked if the invocation commits. Outside a git repo, `--untracked` implies pure mutation with no commit, so commit-message flags, `--allow-empty`, and `--no-checkout` are invalid.

## 4. Script syntax

A script is a sequence of lines. Each line is either blank, a comment (starts with `#`), or a statement. A statement is the same token sequence that follows `etch` on the command line, tokenized using shell single-quote, double-quote, and backslash-escape rules. There are no expansions of any kind — `$FOO` is a literal four-character string.

Multi-line values use heredocs with the same syntax shell uses, but heredocs are an etch-level construct (not delegated to a shell parser) and are the only multi-line affordance:

```
# Set a scalar
set posts/hello.md frontmatter.title "Hello, world"

# JSON literal as a single argument
set posts/hello.md frontmatter.tags '["draft","intro"]'

# Multi-line content via heredoc
replace-section posts/hello.md "## Summary" <<EOF
This post introduces the project and its goals.
EOF

# Delete a key
delete posts/hello.md frontmatter.draft
```

Statement shapes are regular, but not literally fixed at three arguments. The common mutating shape is `verb path selector value`; read verbs often omit `value`; file verbs may have two paths; plumbing commands add a format namespace before the verb. Where a command needs additional knobs, they are flags (`--depth=2`), not positional surprises. If a command cannot be summarized compactly in the help table, the command is wrong.

### Why this and not the alternatives

A decision matrix considered six options against ten criteria (LLM correctness, CLI/script symmetry, multi-line value ergonomics, parse error locality, implementation surface, quoting pit count, shell composability, familiarity, doc density, auditability). The chosen option (shell-quoted argv lines) scored highest by a wide margin. The two key disqualifiers for embedded shell (e.g., via `mvdan/sh`) were:

- **Auditability.** Embedded shell has command substitution and expansion, so a static enumeration of "what will this script do" requires evaluation, and even then can reach outside the etch operation set entirely. That breaks the authorization story.
- **Quoting pit count.** Inheriting shell's expansion rules means inheriting its quoting traps, which are the #1 LLM failure mode in scripted-tool tasks.

JSON, S-expression, Tcl-like, and bespoke-DSL alternatives were ruled out (CLI/script asymmetry, familiarity cost, or implementation overhead with no offsetting benefit).

## 5. Verbs

Two tiers:

**Porcelain** verbs infer file format from path extension (`.md` → markdown-with-optional-frontmatter, `.json` → json, `.yaml`/`.yml` → yaml). They cover the common cases and are what most scripts use.

**Plumbing** commands are format-explicit subcommands (`json set`, `yaml set`, `frontmatter set`, `md replace-section`). They have no inference and no surprises, suitable for scripts where the file extension might lie or where the porcelain heuristics would pick the wrong format.

The MVP verb surface is constrained by §3 (regular command shapes) and §10 (must fit in a dense help page). Verbs fall into two semantic classes:

- **Mutating verbs** compute new file contents. They may internally read one or more paths, but their user-visible effect is the plan/commit/materialization pipeline.
- **Read verbs** print information to stdout and have no write, plan, commit, or materialization phase. In MVP, `run` accepts mutating verbs only; mixed read/write scripts and batched read-only scripts are deferred.

### Selector syntax

Structured data selectors are paths, not programs. For JSON, YAML, and YAML frontmatter, etch selectors are a subset of RFC 9535 JSONPath singular queries: they can identify at most one node and exclude wildcards, recursive descent, slices, filters, unions, and function extensions. This gives etch a standard selector grammar without importing JSONPath's full query language.

- `$.agents.assistant.last_run` for ordinary object fields.
- `$.items[0].title` for array indexes if indexes are admitted in MVP.
- `$["key.with.dots"]` for keys that cannot be represented as plain dotted segments.

Verbs treat a zero-match selector according to the verb: `set` may create a missing final object member, while `delete`, `get`, and `append` require the selected target to exist unless a verb says otherwise. Syntax that can produce multiple matches is rejected before evaluation.

JSON Pointer was considered because it is standardized and unambiguous, but `/agents/assistant/last_run` is less natural at the CLI and composes poorly with Markdown part selectors. Full JSONPath was considered because it is standardized and familiar, but wildcard/predicate/multi-match behavior is the wrong default for single-target mutation. Full `jq` was considered because its path syntax is good, but its execution model is far broader than etch's mutation contract.

### Markdown parts

Markdown files have several addressable parts: YAML frontmatter, section bodies, task list items, pipe tables, and Obsidian Dataview-style fields. In porcelain Markdown commands, `frontmatter` is a selector namespace, not a variable. `set task.md frontmatter.status complete` means "within this Markdown file, mutate the YAML frontmatter field `status`." The structured selector inside that part is still normalized as JSONPath (`$.status`) in plans.

Alternatives considered:

- **Implicit frontmatter for Markdown paths.** Shorter, but blocks body-level fields with the same names and makes `set task.md status complete` ambiguous.
- **Separate porcelain verbs only.** `frontmatter set task.md status complete` is explicit but loses the convenient format-inferred `set path selector value` shape.
- **Virtual YAML file model.** Treat frontmatter as a hidden YAML document adjacent to the Markdown body. Accurate internally, but awkward to explain at the CLI.

The resulting rule: porcelain Markdown selectors may use explicit part prefixes such as `frontmatter.*` for CLI ergonomics; canonical plans store the Markdown part and normalized structured selector separately. Plumbing commands can use namespaces instead (`frontmatter set <path> <selector> <value>`), where `<selector>` is already relative to that part.

### Mutating surface

| Format | Verbs | Selector/value behavior |
|---|---|---|
| JSON | `set`, `delete`, `append` | Selectors use the JSONPath subset above. `set` creates a missing leaf and missing object containers; it fails if an intermediate component exists but is not an object. Values are parsed as JSON when they are valid JSON literals, otherwise treated as strings. `delete` removes an existing key. `append` requires the selector to name an array and appends the parsed value. |
| YAML | `set`, `delete`, `append` | Same selector and value semantics as JSON, but using a round-tripping YAML representation so comments, key order, indentation style, and scalar spelling are preserved where the parser can preserve them. |
| Markdown frontmatter | `set`, `delete`, `append` | Selectors under `frontmatter.*` operate on YAML frontmatter using the YAML rules above. If a Markdown file has no frontmatter, `set frontmatter.*` creates a frontmatter block; `delete` and `append` require an existing block. |
| Markdown body | `replace-section` | The selector is a heading line such as `## Notes`. Replacement covers the content under that heading up to the next heading of equal or higher level. Missing or ambiguous headings are errors. |
| Files | `create`, `rm`, `mv`, `cp` | File-level verbs borrow familiar Unix names where the operation is truly the same shape, but etch does not inherit shell globbing, recursive deletion, or arbitrary flags. Signatures are `create <path> <content>`, `rm <path>`, `mv <src> <dst>`, and `cp <src> <dst>`. `create` and `cp` fail if the destination exists. `rm` fails if the path is absent. `mv` fails if the source is absent or destination exists. |

Deferred mutating operations include list `prepend`, `insert`, and value-based `remove`; Markdown `append-section`, `prepend-section`, `delete-section`, `move-section`, and `split-section`; and cross-file transforms that read from one location and write/remove/insert elsewhere. These are still mutating verbs, not read verbs, because their contract is new file contents rather than stdout.

### Markdown conventions to design early

Markdown is not just prose in the target repositories. The early Markdown surface needs explicit conventions for:

- **Sections.** ATX headings are stable anchors. Section selectors should support exact heading text first, then consider disambiguators for repeated headings.
- **Tasks.** GitHub-style task list items (`- [ ]` / `- [x]`) need verbs for completion and metadata updates. Candidate commands: `task complete <path> <task-selector>` and `task set <path> <task-selector> <field> <value>`.
- **Inline fields.** Obsidian Dataview-style fields such as `[due:: 2026-03-01]` should be considered first-class Markdown data. Candidate commands: `field set <path> <field-selector> <value>`, `field delete <path> <field-selector>`, and `field get <path> <field-selector>`.
- **Tables.** Markdown pipe tables and CSV files likely share a table mutation model: set cell, append row, delete row, and maybe upsert row by key column. Table selectors need a way to identify the table, row, and column without relying on brittle line numbers.

These commands need a separate selector design pass before they graduate into the MVP table above.

Initial read verbs are `get`, `exists`, and `keys` for JSON/YAML/frontmatter paths, plus `get-section` and `list-sections` for Markdown bodies. They print to stdout, never commit, and exist mainly for scripting symmetry, tests, and shell composition.

Each mutating verb is annotated as idempotent or non-idempotent in the help table. Idempotent ops that produce no content change are no-ops and don't contribute to the commit; non-idempotent ops (`append`) always produce changes. Read verbs are explicitly marked read-only and are outside the plan/commit machinery described below.

## 6. Plans

A **plan** is etch's structured answer to "if I were to run this mutating invocation, what exactly would happen — every byte that would change, in what file, ending in which commit?" It is produced by the same code path as actual execution, stopped one step before side effects. Same parser, same operation evaluator, same commit-tree builder — everything except writing objects, updating the ref, and materializing touched paths into the checkout. The plan cannot drift from reality because it *is* reality minus the side effects. Read verbs have no plan form in MVP.

The canonical plan format is JSON. It is the machine contract for hashing, authorization caches, tests, and host integrations. It should not be replaced by a prose or email-shaped format, because plan identity depends on stable parsing, canonicalization, and versioning.

Human preview is a separate surface. `--dry-run` lowers the semantic JSON plan to a base-locked mailbox patch compatible with `git am`, following `git format-patch` conventions: metadata headers, commit-message block, a three-dash separator, diffstat, and patch hunks. It is optimized for review and mechanical replay, not for canonical plan identity.

### Plan contents

```json
{
  "etch_plan_version": 1,
  "ref": "refs/heads/main",
  "base_commit": "a1b2c3...",
  "operations": [
    {
      "verb": "set",
      "target": {
        "path": "posts/hello.md",
        "part": "frontmatter",
        "selector": "$.title"
      },
      "value_sha256": "5d41402a..."
    },
    {
      "verb": "replace-section",
      "target": {
        "path": "posts/hello.md",
        "part": "body",
        "section": "## Summary"
      },
      "value_sha256": "9f86d081..."
    }
  ],
  "files": {
    "posts/hello.md": {
      "before_sha256": "e3b0c442...",
      "after_sha256": "84983e44..."
    }
  },
  "tree": "7f4e8d...",
  "commit": {
    "message": "etch: 2 changes in posts/hello.md\n\nChanges:\n- set posts/hello.md frontmatter.title \"Hello, world\"\n- replace-section posts/hello.md \"## Summary\" \"A tighter summary.\""
  }
}
```

All operation values are stored as `value_sha256` (no inline values) for uniformity and small plan size. Commit messages may contain bounded value previews, but the actual content is recoverable from the file's before/after states when needed for display.

### Dry-run preview

`--dry-run` renders the plan as a reviewable patch message rather than canonical JSON. It is a lowered artifact: applying it replays the already-computed byte-level change against the planned base, but it does not re-run etch's semantic selectors, validation, retries, or materialization logic.

The output follows `git am`'s mailbox conventions:

- The first `From <oid> Mon Sep 17 00:00:00 2001` line is the mbox delimiter, not commit metadata. etch uses the zero object ID there, following `git format-patch --zero-commit`; plan identity lives in `Etch-Plan-Hash`.
- The mail `Subject:` is the commit-message subject. etch should not wrap it in `[PATCH]` or `[ETCH PLAN]`, because `git am` strips common `[PATCH ...]` prefixes.
- The body before the first three-dash line (`---`), `diff -`, or `Index:` line becomes the commit-message body.
- Everything after the first three-dash line is patch input or patch commentary, not commit-message text.
- `Etch-*` lines live in the mail header block. They are for etch-aware readers, not commit-message trailers, and should not appear in `git log`.
- The `From:` and `Date:` mail headers carry the planned author identity and author date for `git am`.
- The applying committer identity and committer date come from the `git am` environment and options, so `git am` compatibility promises the same tree, commit message, and author metadata, not the same commit OID.

When a planned change can be represented as a Git patch, `--dry-run` output should be compatible with `git am`. The exact text is not part of the plan hash, but it should be stable enough for snapshot tests and easy human review:

```text
From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001
From: <author-name> <author-email>
Date: <author-date>
Subject: etch set posts/hello.md frontmatter.title "Hello, world"
Etch-Plan-Hash: sha256:<hex>
Etch-Base-Commit: a1b2c3...
Etch-Tree: 7f4e8d...

---
 posts/hello.md | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)

diff --git a/posts/hello.md b/posts/hello.md
...
```

The commit-message portion before `---` is exactly the generated commit message from §8. A multi-op plan's `Changes:` body therefore appears before `---`. After `---`, etch emits the diffstat and patch hunks. It does not embed the original command or script in the dry-run output; the generated commit message plus the patch hunks are the review surface, and the JSON plan is the canonical semantic record.

For a multi-op `run`, the generated commit body appears before `---`:

```text
From 0000000000000000000000000000000000000000 Mon Sep 17 00:00:00 2001
From: <author-name> <author-email>
Date: <author-date>
Subject: etch: 2 changes in posts/hello.md
Etch-Plan-Hash: sha256:<hex>
Etch-Base-Commit: a1b2c3...
Etch-Tree: 7f4e8d...

Changes:
- set posts/hello.md frontmatter.title "Hello, world"
- replace-section posts/hello.md "## Summary" "A tighter summary."

---
 posts/hello.md | 6 +++---

diff --git a/posts/hello.md b/posts/hello.md
...
```

If a future operation cannot be represented as a `git am`-compatible patch, `--dry-run` must either report that limitation or use an explicitly different format marker. Silent fallback to a non-applicable lookalike is not allowed.

### Canonicalization & hashing

Plans are serialized using JCS (RFC 8785): sorted keys, no whitespace, escaped consistently. The plan hash is SHA-256 of the canonical bytes. Two implementations of the canonicalizer agree byte-for-byte.

### What's in the hash, and why

- **Operations** anchor to user intent.
- **Per-file before/after sha256** anchors to both input state and computed output. A change in etch's *behavior* (a verb's semantics changed across versions) invalidates plans even when operations look textually identical, because the after-hashes differ. This means a runtime caching "I approved hash H" doesn't also need to track etch versions.
- **Base commit** pins to a point in history; if HEAD has moved, the plan is stale.
- **Tree OID** is the strongest possible "what would actually land" anchor — two plans with the same tree OID produce byte-identical commits.
- **Commit metadata** captures message and author. Author/timestamp are normalized during planning unless `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` are set.

### Plans drive both audit and execution

A plan serves three purposes from one structure:

1. **Audit record** for runtimes rendering "here's what will happen" to humans.
2. **Auth-cache key.** Hash a plan, cache the user's approval against the hash; reuse approval iff the same plan recomputes to the same hash.
3. **STM transaction record.** See §7.

## 7. Execution model & concurrency

etch executes optimistically and uses git's existing CAS primitives to detect conflict.

### Per-transaction temp index

Every mutating invocation creates a temp index (`GIT_INDEX_FILE` set to a tempfile) initialized from `HEAD^{tree}` (or the empty tree for an unborn branch), not by copying the user's live index. etch writes candidate blobs for touched paths into that temp index, builds a tree, creates the commit object, and updates the ref. The user's live index is never used to build the transaction, and unrelated staged or unstaged changes are invisible to etch.

### Working-tree materialization

After a successful ref update, etch materializes touched paths into the caller's working tree by default. Materialization is a checkout phase, not part of commit construction: it updates the working tree and live index entries for touched paths only so they match the committed tree. Unrelated paths are not touched.

`--no-checkout` skips this phase. It is useful for agent workflows that care only about the commit graph and do not need the shared checkout to reflect the new commit immediately.

If materialization cannot safely update a touched path because it changed after validation, the commit remains in history and etch reports a materialization failure. The transaction's durability boundary is the ref update; checkout is the post-commit synchronization step.

### The two CAS checks

1. **Read-set validation.** The plan records `before_sha256` for every file etch read. At execution time, etch re-hashes those files; if any differ, the transaction aborts. This is the read-set check from optimistic STM — the transaction is valid iff its inputs haven't been invalidated.
2. **Ref CAS.** etch updates the ref using the old-value form (`update-ref <ref> <new> <old-expected>`), where old-expected is the plan's `base_commit`. If another writer committed to the ref between plan and execution, this fails atomically.

Both checks are necessary. Read-set validation prevents stale computation when other writers have edited the working tree without committing. Ref CAS prevents losing concurrent commits when other writers have committed without touching our files.

### Retry policy

On either CAS failure, etch re-plans from the latest observed state and tries again. Retries are bounded by `--retries` (default 3, `-1` = forever). Backoff strategy is an implementation detail (likely exponential with jitter, capped). Retry is internal and invisible to the caller in the success case.

### Pinned execution is deferred

Plan hashes are useful for external approval caches: a host can present a plan, remember that the user approved hash H, and later ask etch to execute only if the recomputed plan still hashes to H. That prevents a time-of-check/time-of-use gap where the approved command text is the same but the file contents, branch head, commit message, or etch behavior changed.

That said, pinned execution is not required for the standalone CLI MVP. The MVP exposes stable plan hashes but defers the execution flag (for example, `--require-plan-hash <hex>`) until there is a concrete host-runtime integration that needs it.

## 8. Commits

Every successful mutating etch invocation produces exactly one commit by default (or zero, if all operations were idempotent no-ops and `--allow-empty` was not passed). `--no-commit` is the explicit escape hatch for "apply, but do not record a commit." Read verbs never create commits.

### Auto-generated messages

The default commit message is generated from normalized operation descriptors. These descriptors are script-shaped and may include bounded value previews for operations whose main effect is writing a value. They never include unbounded raw payloads; full values live in file contents and are represented in plans by `value_sha256`.

Descriptor shape is `<verb> <path> <selector> [<value-preview>]`, adjusted for verbs without selectors or values.

Value previews are deterministic:

- Values are previewed after parsing, not from raw argv spelling. JSON/YAML/frontmatter values render as compact JSON. Markdown body text renders as a JSON string after line-ending normalization.
- Exact previews are allowed only for single-line UTF-8 values whose rendered preview is at most 80 characters.
- Longer or multi-line UTF-8 values render with `...` truncation only. For strings, the ellipsis lives inside the JSON string (`"prefix..."`). For objects and arrays, the compact JSON rendering is cut on a character boundary and ends with `...`; truncated previews are not required to be parseable JSON.
- Non-UTF-8 values render as `<binary, N bytes>`.
- Value previews should fit within 80 characters. If a descriptor would exceed 120 characters, the preview budget is reduced so the descriptor fits; if the target alone consumes the line budget, the value preview is omitted from that descriptor.
- Single-op subjects include an exact value preview only if the full subject fits on one line within 72 characters. Otherwise the subject omits the value and the body contains `Value: <value-preview>`.
- Commit messages do not include value hashes. Hashes are plan metadata; commit-message values are human previews.

- Single op with a short value: subject only, `etch <verb> <path> <selector> <value-preview>` (e.g., `etch set posts/hello.md frontmatter.title "Hello, world"`).
- Single op without a value preview in the subject: subject `etch <verb> <path> <selector>`, with a body containing `Value: <value-preview>` when the operation has a value.
- Multi-op in one file: subject `etch: N changes in <path>`, with a body headed `Changes:` and one descriptor per operation.
- Multi-op across files: subject `etch: N changes across M files`, with the same `Changes:` body.

Example multi-op message:

```text
etch: 2 changes in posts/hello.md

Changes:
- set posts/hello.md frontmatter.title "Hello, world"
- replace-section posts/hello.md "## Summary" "A tighter summary."
```

Example long-value message:

```text
etch set posts/hello.md frontmatter.summary

Value: "This summary is long enough that it needs a bounded preview..."
```

`--message` overrides entirely. `--message-prefix` and `--message-suffix` compose with the auto-generated message. Configurable templates are deferred.

### Idempotency and the empty-commit case

Operations that produce no content change contribute nothing to the commit. If every op is a no-op, no commit is created and etch exits 0 with a `nothing to do` notice on stderr. `--allow-empty` forces an empty commit in the rare case where this is desired.

### Working-tree state of touched files

If a file etch is targeting differs between `HEAD`, the index, and the working tree, etch reads the working-tree state, applies operations to it, and commits that result by default. The working tree wins over the index for touched paths; unrelated staged changes are not pulled into the etch commit. After the commit lands, default materialization updates the touched working-tree files and index entries to the committed content. Under `--no-commit`, no commit is created; etch rewrites the file in place and leaves the result dirty. Other dirty files are unaffected. Rationale: agents shouldn't have to negotiate with human staging.

## 9. Security model

The threat model: an LLM-driven agent generates etch invocations. The user wants to grant blanket permission ("always allow `etch` on this repo") once, without that grant becoming equivalent to "always allow shell."

### Intrinsic capability bounds (MVP, non-negotiable)

- **No network.** etch makes no network calls.
- **No process spawning.** etch does not exec anything except git for the strict subset of operations it needs (read refs, write objects, update refs).
- **No filesystem escape.** etch reads and writes only within the active root: the git repository by default, or CWD when `--untracked` is operating outside git. No symlink chasing past the active root.
- **No DSL escape hatch.** §4 — the script syntax has no expansion, no subshells, no exec.

These properties are intended to make etch a narrow capability. They are not, by themselves, enough to claim that every host can safely allow a broad shell rule such as `etch *` or `Bash(etch:*)`.

The hard case is host permission matching. Some agents approve commands by executable name, some by shell command prefix, some by glob-like patterns, and some run commands in a sandbox rather than a command allow-list. A broad rule also admits etch's own wider-scope modes such as `--untracked`, unless the host can reliably deny or separately prompt for that argument. Therefore, the security claim for MVP is:

> etch's default repo-scoped mode is designed to be safe to grant directly. Blanket shell allow-list guidance is deferred until tested against the permission models of target hosts.

### Plan as authorization primitive (deferred)

A runtime can compute `etch --plan <args...>`, render the JSON to a human, hash it, cache the approval, and later execute only if the same plan would result. The execution pin is deferred, but the plan format should remain suitable for this use.

### Repo-level policy (deferred)

A `.etch/policy.toml` file restricting verbs/paths is anticipated but not in MVP. When designed, policy violations refuse the entire transaction (not partial execution) and exit before running operations. No specific design is committed to here.

### Auth protocol on stdio (out of scope)

An interactive `AUTH-REQUEST`/`AUTH-GRANT` protocol on stdio was considered and rejected — it leaks runtime concerns into etch and competes with whatever the host runtime already does. Etch instead exposes structured exit info that runtimes can drive themselves.

## 10. Documentation surfaces

Three layers, designed so an agent can load the entire feature set in a small token budget:

| Surface | Audience | Budget |
|---|---|---|
| `etch --help` | agents that already know CLIs | ~400 tokens |
| `etch help` / `man etch` | humans, agents needing examples | ~1500 tokens |
| `etch verbs --json` | runtimes building allow-lists | machine-readable |

The verb table is the bulk of the help. It's strictly tabular: name, signature, one-line description, idempotent Y/N. Deviations get one line of prose, not a section. The discipline is enforced by writing the help page first when designing each verb — if it doesn't fit, the verb is wrong (§5).

## 11. Exit codes

- `0` — success.
- `1` — operation failed.
- `2` — usage error (unknown flag, malformed argv).

MVP should not allocate a large taxonomy up front. Add distinct exit codes only when callers have a distinct automated response. For example, retry-budget exhaustion may eventually deserve a separate code if callers should retry later, while "selector not found" and "target file missing" can both be ordinary operation failures for a human-facing CLI.

## 12. Configuration

Environment variables:

| Var | Effect |
|---|---|
| `ETCH_UNTRACKED` | Same as `--untracked`. |
| `GIT_AUTHOR_DATE`, `GIT_COMMITTER_DATE` | Standard git semantics; used in commits and plan hashes. |
| `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL` | Standard git semantics. |

No custom config file in MVP. Repo-level policy file (§9) is deferred.

## 13. Non-goals

- **Generic text editing.** Use `sed`, `awk`, `sd`. etch is for *structural* mutations on known formats.
- **Turing-complete scripting.** Use a real language and shell out to etch.
- **Conflict resolution.** etch detects conflict and aborts; merging is the caller's problem.
- **Network operations.** No `git push`, no `git pull`. The "git side effect" is local commits only.
- **Authorization UX.** Surfaces (plan format, exit codes) are provided; the UX is the host runtime's job.

## 14. Open questions

- **Selector grammar.** Finalize the exact RFC 9535 JSONPath subset, including quoted segments, escaping, and whether array indexes are in MVP.
- **Markdown tables and CSV.** Whether Markdown pipe tables and CSV should share one table selector/mutation model in MVP.
- **Permission-model research.** Test Codex, Claude Code, Cursor, and other target hosts before documenting any blanket shell allow-list rule. In particular, decide whether `--untracked` must move behind a separate permission surface.
- **Exit-code splits.** Whether retry exhaustion or materialization failure needs a distinct code because callers can do something useful with it.
- **Ref scope.** HEAD-only in MVP; `--ref refs/heads/<branch>` for non-checked-out branches is plausible but adds complexity.
- **Backoff strategy.** Implementation detail, not in spec.
- **Plan inline values.** Plan values are always hashed for uniformity. May reconsider if plans become hard to read.
- **Materialization failures.** Exact exit code and stderr shape when the commit succeeds but post-commit checkout of touched paths fails.

## 15. Validation strategy

Most CLI behavior should be covered by snapshot tests over generated output:

- `--dry-run` snapshots use `git format-patch` output with etch metadata headers and are the primary golden surface for human-readable file mutation previews. For every representable dry-run fixture, tests should apply the output with `git am` in a temp repo at the planned base and assert the resulting tree OID matches the plan's tree. The same fixture should assert that `git am` creates the planned commit subject/body and author metadata, and that `Etch-*` headers do not appear in the commit log. Tests should not assert commit OID equality because committer metadata is supplied by the applying environment.
- `--plan` snapshots cover canonical JSON shape, redacted values, hashes, tree OIDs, and commit messages. Commit-message fixtures should cover exact value previews, ellipsis truncation, descriptor line budgets, and single-op subject/body fallback. Tests should also verify canonical plan bytes and plan hash stability.
- Successful commit tests compare `git show --stat --patch --format=fuller HEAD` against snapshots, with author and dates normalized by environment.
- `--no-commit` and `--no-checkout` tests assert working tree, live index, and `HEAD` state separately.
- Script tests cover tokenization, quoting, comments, heredocs, stdin via `run -`, and failure atomicity across multi-op runs.
- Format tests cover JSON, YAML, frontmatter, Markdown sections, Markdown fields/tasks/tables as they land, and CSV if admitted.
- Concurrency tests use multiple temp indexes and explicit ref CAS races to prove disjoint-path retry, same-path conflict behavior, and retry-budget exhaustion.
- Security tests cover path traversal, symlink escapes, absolute paths, untracked-path handling, outside-git behavior, and refusal to spawn non-git processes.

Unit tests should carry the parser, selector evaluator, format round-trippers, and commit-tree builder. End-to-end tests should prefer real temporary git repositories so the snapshots exercise the same object and ref behavior users get.

## 16. Implementation notes

- Language: Go.
- Module: `github.com/brandonbloom/etch`.
- git operations: prefer `go-git` for in-process git access; fall back to invoking `git` only where `go-git` lags. Decision deferred until prototyping reveals which surface area we actually need.
- Parser: hand-written tokenizer for the script DSL. ~200 lines target.
- Plan canonicalization: implement JCS directly or use a vetted library; verify byte-equivalence in tests.
