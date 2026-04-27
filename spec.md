# etch — Design

> Status: v0, in progress. Captures decisions made through initial design discussion. Verb surface and error taxonomy are explicitly deferred.

## 1. What etch is

A small CLI for **mechanical mutations to text and data files**, where each successful invocation is also a git commit. Operations are atomic (per op) and transactional (per invocation): all the operations in one invocation either land as a single commit or none of them do.

The primary audience is agentic coding tools operating against git-backed wiki-style repositories — multiple agents and humans iterating forward, where mechanical edits ("set this frontmatter field," "replace this markdown section," "delete this JSON key") are common enough that doing them through a general-purpose patch tool burns tokens and risks subtle errors. etch trades generality for precision: it knows about a small, fixed set of file formats and a small, fixed set of operations, and it does them quickly and predictably.

The headline product is a binary. There may eventually be a Go library underneath suitable for embedding; that question is deferred.

## 2. Mental model

Three principles:

1. **Commits are not optional.** A successful etch invocation produces a commit, period. Outside a git repo, etch errors out. The escape hatch (`ETCH_UNTRACKED=1` / `--untracked`) exists so etch can operate as a pure mutator in non-git contexts, but the default is that the commit is the whole point.

2. **The script line and the CLI argv are the same thing.** A line in an etch script is the same tokens you'd type at the shell, modulo the binary name. This symmetry is the primary ergonomic property. It means agents can prototype on the command line and concatenate script files without translation, and it means the help page documents both surfaces simultaneously.

3. **No escape hatches inside the DSL.** The script syntax has no variables, no expansions, no subshells, no pipes, no conditionals. Composition happens at the shell level, *outside* etch. This is what makes etch's blast radius statically bounded and makes "always allow etch" a defensible authorization stance.

## 3. Invocation surface

Verb-first, with a small set of top-level flags that apply to any verb.

```
etch <verb> [args...]                  one-shot
etch tx <script-or-->                  multi-op transaction from file or stdin
etch run <script-or-->                 alias for tx
etch --plan <verb> [args...]           emit JSON plan, do not execute
etch --plan tx <script-or-->           emit JSON plan for batch
etch --dry-run <verb> [args...]        emit format-patch-style preview
etch help [verb]                       human-readable help
etch verbs --json                      machine-readable verb catalog
etch --help                            terse one-screen reference
```

Top-level flags that apply to any verb form:

| Flag | Env | Effect |
|---|---|---|
| `--plan` | — | Emit JSON plan to stdout, do not write or commit. |
| `--dry-run` | — | Emit format-patch preview to stdout, do not write or commit. |
| `--no-commit` | — | Apply changes to working tree, skip commit. |
| `--untracked` | `ETCH_UNTRACKED=1` | Permit running outside a git repo. |
| `--message <m>` | — | Override auto-generated commit message. |
| `--message-prefix <m>` | — | Prepend to auto-generated message. |
| `--message-suffix <m>` | — | Append to auto-generated message. |
| `--retries <n>` | — | Retry budget on optimistic-concurrency conflict. `-1` = retry forever. Default `3`. |
| `--require-plan-hash <hex>` | — | Refuse to execute unless the computed plan hashes to this value. |
| `--allow-empty` | — | Permit a commit with no content change. |

`--plan`, `--dry-run`, `--no-commit` are mutually exclusive. The first two skip execution entirely; the third writes but doesn't commit.

## 4. Script syntax

A script is a sequence of lines. Each line is either blank, a comment (starts with `#`), or a statement. A statement is `verb arg arg arg...`, tokenized using shell single-quote, double-quote, and backslash-escape rules. There are no expansions of any kind — `$FOO` is a literal four-character string.

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

The `verb arg arg arg` shape is uniform across all verbs. Where a verb needs additional knobs, they are flags (`--depth=2`), not positional surprises. If a verb cannot fit the uniform shape, the verb is wrong.

### Why this and not the alternatives

A decision matrix considered six options against ten criteria (LLM correctness, CLI/script symmetry, multi-line value ergonomics, parse error locality, implementation surface, quoting pit count, shell composability, familiarity, doc density, auditability). The chosen option (shell-quoted argv lines) scored highest by a wide margin. The two key disqualifiers for embedded shell (e.g., via `mvdan/sh`) were:

- **Auditability.** Embedded shell has command substitution and expansion, so a static enumeration of "what will this script do" requires evaluation, and even then can reach outside the etch operation set entirely. That breaks the authorization story.
- **Quoting pit count.** Inheriting shell's expansion rules means inheriting its quoting traps, which are the #1 LLM failure mode in scripted-tool tasks.

JSON, S-expression, Tcl-like, and bespoke-DSL alternatives were ruled out (CLI/script asymmetry, familiarity cost, or implementation overhead with no offsetting benefit).

## 5. Verbs

Two tiers:

**Porcelain** verbs infer file format from path extension (`.md` → markdown-with-optional-frontmatter, `.json` → json, `.yaml`/`.yml` → yaml). They cover the common cases and are what most scripts use.

**Plumbing** verbs are format-explicit (`set-yaml`, `set-json`, `set-frontmatter`, `replace-section-md`). They have no inference and no surprises, suitable for scripts where the file extension might lie or where the porcelain heuristics would pick the wrong format.

The full verb surface is **deferred** — it will be designed against the constraints set by §3 (uniform `verb path selector value` shape) and §10 (must fit in a dense help page). Likely categories:

- Scalar set/delete on structured paths (json/yaml/frontmatter).
- List append/prepend/insert/remove.
- Markdown section replace/append/prepend/delete.
- File-level create / move / delete (these are commits even though they aren't "edits" in the usual sense).
- Read verbs (`get`, `exists`, `keys`) that don't commit and exist mainly for scripting symmetry and testing.

Each verb is annotated as idempotent or non-idempotent in the help table. Idempotent ops that produce no content change are no-ops and don't contribute to the commit; non-idempotent ops (`append`) always produce changes.

## 6. Plans

A **plan** is etch's structured answer to "if I were to run, what exactly would happen — every byte that would change, in what file, ending in which commit?" It is produced by the same code path as actual execution, stopped one step before write-back. Same parser, same operation evaluator, same commit-tree builder — everything except writing files to disk and updating the ref. The plan cannot drift from reality because it *is* reality minus the side effects.

### Plan contents

```json
{
  "etch_plan_version": 1,
  "ref": "refs/heads/main",
  "base_commit": "a1b2c3...",
  "operations": [
    {
      "verb": "set",
      "path": "posts/hello.md",
      "selector": "frontmatter.title",
      "value_sha256": "5d41402a..."
    },
    {
      "verb": "replace-section",
      "path": "posts/hello.md",
      "selector": "## Summary",
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
    "message": "etch: 2 changes in posts/hello.md\n\n- set frontmatter.title\n- replace-section \"## Summary\""
  }
}
```

All operation values are stored as `value_sha256` (no inline values) for uniformity and small plan size. The actual content is recoverable from the file's before/after states when needed for display.

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

1. **Audit format** for runtimes presenting "here's what will happen" to humans.
2. **Auth-cache key.** Hash a plan, cache the user's approval against the hash; reuse approval iff the same plan recomputes to the same hash.
3. **STM transaction record.** See §7.

## 7. Execution model & concurrency

etch executes optimistically and uses git's existing CAS primitives to detect conflict.

### Per-transaction temp index

Every invocation creates a temp index (`GIT_INDEX_FILE` set to a tempfile, current index copied in). etch updates only the paths it's touching, builds a tree, creates the commit object, and updates the ref. The user's working index is never touched — unstaged changes elsewhere are completely invisible to etch. This is the same isolation pattern `git stash` uses.

### The two CAS checks

1. **Read-set validation.** The plan records `before_sha256` for every file etch read. At execution time, etch re-hashes those files; if any differ, the transaction aborts. This is the read-set check from optimistic STM — the transaction is valid iff its inputs haven't been invalidated.
2. **Ref CAS.** etch updates the ref using the old-value form (`update-ref <ref> <new> <old-expected>`), where old-expected is the plan's `base_commit`. If another writer committed to the ref between plan and execution, this fails atomically.

Both checks are necessary. Read-set validation prevents stale computation when other writers have edited the working tree without committing. Ref CAS prevents losing concurrent commits when other writers have committed without touching our files.

### Retry policy

On either CAS failure, etch re-plans from current state and tries again. Retries are bounded by `--retries` (default 3, `-1` = forever). Backoff strategy is an implementation detail (likely exponential with jitter, capped). Retry is internal and invisible to the caller in the success case.

### `--require-plan-hash` opts out of retry

When the caller passes `--require-plan-hash=<hex>`, etch computes the plan, hashes it, and refuses to execute if the hash differs from the expected value. There is no retry — failure is reported immediately with exit code 22 (plan-mismatch). This is the path for "an external authority approved this exact plan, run it or don't."

### Two execution modes from one machinery

| Mode | Trigger | Behavior |
|---|---|---|
| Optimistic | default | Plan, validate, commit, retry on conflict. STM-style. |
| Pinned | `--require-plan-hash` | Plan, validate against expected hash, commit-or-fail. Caller owns retry. |

## 8. Commits

Every successful etch invocation produces exactly one commit (or zero, if all operations were idempotent no-ops and `--allow-empty` was not passed).

### Auto-generated messages

The default commit message is generated from the operations:

- Single op: `etch: <verb> <selector> in <path>` (e.g., `etch: set frontmatter.title in posts/hello.md`).
- Multi-op: `etch: N changes across M files`, with a body listing each operation.

`--message` overrides entirely. `--message-prefix` and `--message-suffix` compose with the auto-generated message. Configurable templates are deferred.

### Idempotency and the empty-commit case

Operations that produce no content change contribute nothing to the commit. If every op is a no-op, no commit is created and etch exits 0 with a `nothing to do` notice on stderr. `--allow-empty` forces an empty commit in the rare case where this is desired.

### Working-tree state of touched files

If a file etch is targeting has unstaged changes in the user's working tree, etch reads the working-tree state, applies operations to it, writes back, and commits the result. The file becomes clean (changes committed). Other dirty files are unaffected. Rationale: agents shouldn't have to negotiate with human staging.

## 9. Security model

The threat model: an LLM-driven agent generates etch invocations. The user wants to grant blanket permission ("always allow `etch` on this repo") once, without that grant becoming equivalent to "always allow shell."

### Intrinsic capability bounds (MVP, non-negotiable)

- **No network.** etch makes no network calls.
- **No process spawning.** etch does not exec anything except git for the strict subset of operations it needs (read refs, write objects, update refs).
- **No filesystem escape.** etch reads and writes only within the current git repository (or CWD when `--untracked`). No symlink chasing past the repo root.
- **No DSL escape hatch.** §4 — the script syntax has no expansion, no subshells, no exec.

These properties are what make "always allow etch" defensible in a way "always allow bash" isn't.

### Plan as authorization primitive (MVP)

A runtime can compute `etch --plan <args...>`, render the JSON to a human, hash it, cache the approval, and re-invoke with `etch --require-plan-hash <hex> <args...>` to execute only if the same plan would result. This composes with any host runtime's allow-list machinery without etch knowing anything about specific runtimes.

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
- `1` — generic failure (catch-all; future versions may narrow).
- `2` — usage error (unknown flag, malformed argv).
- `20`–`63` — etch-specific. Reserved as a contiguous block, allocated as the error taxonomy is fleshed out.

Initial allocations (subject to revision):

| Code | Meaning |
|---|---|
| `20` | Parse error in script. |
| `21` | Selector did not match (e.g., frontmatter key absent when required). |
| `22` | Plan-hash mismatch (`--require-plan-hash` failure). |
| `23` | Concurrency conflict, retry budget exhausted. |
| `24` | Outside git repo, `--untracked` not set. |
| `25` | Target file not found / outside repo. |

Errors during `--plan` use the same codes as actual execution.

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

- **Verb surface.** Full set of verbs and their signatures.
- **Error taxonomy.** Specific exit code allocations within 20–63.
- **Ref scope.** HEAD-only in MVP; `--ref refs/heads/<branch>` for non-checked-out branches is plausible but adds complexity.
- **Backoff strategy.** Implementation detail, not in spec.
- **Plan inline values.** Currently always-hashed for uniformity. May reconsider if plans become hard to read.

## 15. Implementation notes

- Language: Go.
- Module: `github.com/brandonbloom/etch`.
- git operations: prefer `go-git` for in-process git access; fall back to invoking `git` only where `go-git` lags. Decision deferred until prototyping reveals which surface area we actually need.
- Parser: hand-written tokenizer for the script DSL. ~200 lines target.
- Plan canonicalization: implement JCS directly or use a vetted library; verify byte-equivalence in tests.