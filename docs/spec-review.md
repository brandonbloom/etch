# Spec Conformance Review

Review of the etch implementation against `spec.md`. Organized by severity.

---

## Critical: Plan Hashing Is Not JCS (RFC 8785)

**Spec:** Plan hashes are computed over RFC 8785 JSON Canonicalization Scheme bytes. Object members sorted by RFC 8785 UTF-16 code-unit order. Selected candidate: `github.com/lattice-substrate/json-canon/jcs`. (§6, §15, §17)

**Implementation:** `planHash` in `planner.go:422-434` uses `json.Marshal` on a Go struct alias. This produces deterministic output for a given Go version (struct field order), but it is not JCS:

- Key order follows Go struct declaration order, not UTF-16 code-unit sorting.
- Numeric canonicalization is not guaranteed (IEEE-754 binary64 rules per JCS).
- Nested map keys use Go's default map iteration order (randomized in Go), not JCS sorting.
- No duplicate-key rejection or I-JSON domain validation.
- Output includes whitespace/formatting from struct tags (though `json.Marshal` without indent is compact).

Two JCS implementations would not agree on bytes. Plan hashes are therefore not interoperable and will silently break if Go changes struct serialization order.

**Impact:** Plan hashes are the authorization cache primitive (§6, §9). Any host caching approval against plan hashes will get wrong answers if paired with a conforming JCS implementation.

---

## Critical: YAML Round-Trip Preservation Is Lost

**Spec:** "Round-tripping YAML representation so comments, key order, indentation style, anchors, aliases, and scalar spelling are preserved where the parser can preserve them." Anchors and aliases are concrete syntax nodes, not dereferenced. (§5, §15)

**Implementation:** `formats.go:38-60` does `yaml.Unmarshal` into `any`, converts to Go maps/slices via `yamlToJSON`, mutates via JSON-style operations, then `yaml.MarshalWithOptions` with `UseLiteralStyleIfMultiline(true)`.

This destroys:
- **Comments** (Unmarshal discards them)
- **Key order** (Go maps are unordered; output reorders keys)
- **Indentation style** (MarshalWithOptions uses its own default)
- **Scalar spelling** (e.g., `yes` vs `true`, `0x1A` vs `26`, quoted vs unquoted)
- **Anchors and aliases** (Unmarshal resolves them; output has no anchors)
- **Tag information** (lost in `any` unmarshaling)

The spec selected `goccy/go-yaml` specifically for "parser/token/AST APIs for comments, key order, anchors, aliases, token positions, and generated snippets" and says "etch owns selector evaluation and localized byte rewrites." The implementation does not use these AST APIs.

**Impact:** Any YAML file with comments, deliberate ordering, or anchors will be silently reformatted on every etch mutation. This is the opposite of the stated reliability value proposition.

---

## Major: Missing Dependencies per Spec Selections

### urfave/cli/v3 (§15, §17, status: "Selected, passthrough-gated")

Not used. Global flag parsing is hand-rolled in `cli.go:110-189`. Consequences:
- No env-var-backed flags.
- No shell completion.
- No generated help matching urfave conventions.
- Boolean flags with `=` syntax (`--plan=false`) silently set the flag true instead of parsing the value.

### encoding/json/v2 (§17, status: "Brandon-selected with toolchain caveat")

Not used despite `go.mod` declaring Go 1.25.3. Implementation uses `encoding/json` v1 throughout. Consequences:
- No strict JSON behavior (duplicate keys accepted, trailing commas depend on decoder settings).
- Deterministic output differences from v2.

### github.com/theory/jsonpath (§15, §17, status: "Recommended candidate")

Not used. Hand-rolled selector parser in `selector.go`. The parser appears functionally correct for the admitted subset but:
- No RFC 9535 compliance testing.
- No reuse of a standards-tested AST.
- Additional surface area to maintain and audit.

### github.com/yuin/goldmark (§15, §17, status: "Selected, parser-only")

Implemented. Markdown heading, section, and GFM table discovery use goldmark in `formats.go`.

Resolved behaviors:
- Headings inside fenced code blocks and HTML blocks are ignored.
- Closing ATX marker sequences and setext headings are recognized.
- GFM table discovery ignores indented code blocks that look like pipe tables.
- Escaped pipes in table cells are read as data.

Residual caveat: Markdown table writes normalize table formatting and alignment markers through `renderMarkdownTable`.

---

## Major: Dry-Run Creates Git Objects

**Spec:** "--dry-run: Emit git-am-compatible patch to stdout, do not write or commit." (§3) "--plan and --dry-run skip execution entirely." (§3)

Implemented. `PlanOperations` computes the planned tree OID through an ephemeral object store, while execution crosses an explicit `writePlannedTree` / `writeCommitObject` boundary before updating refs. `RenderDryRun` uses the same ephemeral object-store path for temporary tree and commit objects needed to delegate patch rendering to native Git.

Regression coverage asserts that both direct planning and CLI `--plan`/`--dry-run` leave the repository object database unchanged.

---

## Major: `--help` vs `help` Not Distinguished

**Spec:** Two distinct surfaces (§10):
- `etch --help` — "terse one-screen reference" (~400 tokens)
- `etch help` — "humans, agents needing examples" (~1500 tokens)

Implemented. The urfave/cli root command uses `shortHelp` for `--help`, while the `help` command prints the longer command table and topics.

Regression coverage asserts that `etch --help` returns the short reference and `etch help` returns the long help surface.

---

## Major: `--no-checkout` with `--plan`/`--dry-run` Is an Error

**Spec:** "--no-checkout applies only to successful committing invocations; it has no meaning with --plan or --dry-run." (§3)

Implemented. Preview modes accept `--no-checkout` and ignore it, preserving `--plan` and `--dry-run` behavior without emitting checkout-skip diagnostics.

Regression coverage asserts that `--no-checkout --plan` and `--no-checkout --dry-run` both succeed, leave `HEAD` and repository objects unchanged, and do not report skipped materialization.

---

## Moderate: Retry Backoff Uses Fixed Delays, Not Randomized Windows

**Spec (§7):**

| Retry | Sleep window |
|---|---|
| 1 | immediate |
| 2 | 50-150ms |
| 3 | 100-300ms |

Implemented. Retry sleeps now use randomized delays within the spec windows, including capped exponential windows for explicit higher retry budgets:

| Retry | Sleep window |
|---|---|
| 1 | immediate |
| 2 | 50-150ms |
| 3 | 100-300ms |
| 4 | 200-600ms |
| 5 | 400-1200ms |
| 6+ | 800-2000ms |

Regression coverage asserts the retry windows and verifies that `sleepRetry` uses the randomized delay hook without sleeping on the first retry.

---

## Moderate: Numeric Precision in Value Parsing

Implemented. CLI JSON value parsing preserves number tokens as `jsonx.Number` instead of converting them to `float64`. Semantic equality canonicalizes JSON and YAML numeric scalars through exact rational values, so `1`, `1.0`, and `1e0` compare equal while distinct large integers stay distinct.

Regression coverage asserts:
- JSON value decoding and marshaling preserve large integer spelling.
- JSON `add` and `remove` distinguish `9007199254740992` from `9007199254740993`.
- YAML and Markdown frontmatter writes preserve large integer spelling.

---

## Moderate: No Unsupported Checkout Conversion Detection

**Spec (§7):** "etch does not emulate Git's full checkout conversion stack, including .gitattributes text/eol rules, core.autocrlf, working-tree-encoding, or clean/smudge/process filters. If a touched path appears to require such conversion for safe materialization, etch fails cleanly."

Implemented. Clean touched paths are materialized through `git restore --staged --worktree`, letting Git apply checkout conversion. If a touched path was dirty before the commit and appears to require conversion (`filter`, `working-tree-encoding`, `ident`, explicit `eol=crlf`, or `core.autocrlf=true`), etch refuses before writing the commit and suggests cleaning the path or using `--no-checkout`.

Regression coverage asserts that:
- Clean `eol=crlf` paths are committed with repository-normal LF bytes and materialized with CRLF worktree bytes.
- Dirty converted paths fail before `HEAD` moves and leave the worktree untouched.
- `--no-checkout` remains the escape hatch for dirty converted paths.

---

## Moderate: No Injected Workspace Interfaces

**Spec (§15, §16 Phase 2):** "Implementation code should receive explicit filesystem, index, object-store, and workspace handles instead of reaching for process-global disk state." "Define small filesystem, object-store, base-snapshot, live-index, and working-tree interfaces with in-memory test implementations."

Incremental progress:
- `OpenWorkspaceAt` and `runCLIAt` let callers provide an explicit working directory. Repository-dependent tests use those entry points instead of mutating process CWD, except for the regression whose subject is process-CWD independence.
- `Workspace` carries a narrow Git runner interface, with the production implementation isolated in `realGitRunner`. Workspace-owned Git operations route through that boundary, and a focused test verifies runner injection without creating a Git repository.
- `Workspace` carries a narrow working-tree filesystem interface for live file reads and writes. Materialization uses resolved absolute worktree paths instead of CWD-relative plan paths, with regression coverage for dirty-path materialization while the process CWD is elsewhere.
- `Workspace` carries a narrow temp-store interface for Git object directories and synthetic index files. Planning and dry-run temp allocations are injectable, isolated under a test-owned directory in regression coverage, and cleaned up after use.

Remaining implementation gap: path resolution and script reading still use process OS APIs directly. The production Git runner shells out to Git, and most tests use real temporary Git repositories. This means:
- Tests cannot model scenarios without touching the filesystem.
- The architecture diagram's separation between planner, snapshot store, and git backend is not reflected in the code.

---

## Moderate: `errors.As` Reimplemented Incorrectly

`cli.go:25-37` implements `asErr` instead of using `errors.As`:
```go
func asErr(err error, target any) bool {
    type causer interface{ As(any) bool }
    if e, ok := err.(causer); ok {
        return e.As(target)
    }
    ...
}
```

This doesn't handle errors wrapped with `fmt.Errorf("%w", err)` or `errors.Join`. Since `errWithCode` doesn't implement `Unwrap()`, wrapped error codes are silently lost, defaulting to exit code 1 instead of 2 for wrapped usage errors.

---

## Minor Deviations

### Commit message `--message-prefix`/`--message-suffix` lack separators
`planner.go:481-486` concatenates directly without whitespace. `--message-prefix "feat: "` works, but `--message-prefix feat` produces `featetch set ...`.

### `create` command class
`catalog.go:28` classifies `create` as `ClassIdempotent`. The spec (§5) says file-level `create` "fails if the destination exists in the transaction base." This is correct for the class system (idempotent = may be no-op; non-idempotent = always changes), but could confuse readers since `create` is not retry-idempotent (it fails on retry if the first attempt succeeded).

### `simpleThreeWay` merge is limited
`materialize.go:211-230` implements a prefix/suffix merge that only handles non-overlapping single-hunk changes. If both ours and theirs modify different regions of the same file, it falls through to conflict. The spec allows this ("etch does not decide which semantic edit should win") but a more complete three-way merge would reduce false conflicts.

### No guard-specific error messages for `contains` with heredoc
The spec says "Multi-line literals use the same heredoc syntax as mutating values." Tests don't exercise `contains` with heredoc bodies.

### Table ordinal detection is heuristic
`catalog.go:350-351` detects table ordinals by checking `strings.HasPrefix(t[5], "@")`. A row selector like `--before @0` could be misinterpreted if positioned where a table ordinal is expected.

---

## Test Coverage Gaps

The following spec behaviors lack test evidence:

### No tests at all
- `--plan` output (JSON format, schema field, file hashes, tree OID, operations shape)
- Plan hash stability / reproducibility
- `--allow-empty` (both success and rejection cases)
- `--message`, `--message-prefix`, `--message-suffix`
- `exists` guard reading HEAD bytes when worktree file is deleted
- `missing` guard ignoring checkout-only untracked files without `--untracked`
- `contains` guard with multi-line heredoc literal
- File `copy` operation
- File `move` operation
- Script error location format (`etch: <script>:<line>: <message>` for failures during execution, not just parse errors)
- Plumbing commands end-to-end (`json set`, `yaml set`, `frontmatter set`, etc.)
- `frontmatter delete`, `frontmatter remove`
- `yaml delete`, `yaml add`, `yaml remove` end-to-end
- Table row insert (both `--before` and `--after`)
- Table column ranges (`@1..@3`, `[Sales Amount]..[Commission Amount]`)
- Markdown table with scope `doc`
- Markdown table with explicit ordinal (`@0`, `@1`)
- Format-explicit `md table ...` plumbing
- CSV column operations beyond add/rename/delete
- Detached HEAD behavior
- Unborn branch behavior
- `--untracked` with guards (`exists`, `missing`, `contains`)
- `.git` path segment rejection at runtime (only tested via `Resolve`)
- Symlink loop detection
- UTF-8 BOM for YAML, Markdown, CSV (only JSON BOM tested)
- Invalid UTF-8 refusal for structured targets
- Commit message formatting: 72-char subject limit, value preview truncation, `Value:` body fallback
- Staged-plus-unstaged materialization combinations
- Move source/destination split materialization outcomes
- Concurrent create within a transaction (two `create` ops targeting same path)
- `verbs --json` output schema validation (class field, canonical field)
- `etch help <topic>` for each topic through CLI (not just `printHelp`)
- Dry-run output stability for snapshot testing
- Dry-run `git am` compatibility for multi-op changes
- Dry-run for delete, move, copy operations

### Tests exist but are shallow
- Selector normalization: only 3 positive cases, 4 negative cases. Missing: bracket notation with RFC 9535 escaping, Unicode member names, boundary indexes, selectors that look like JSONPath extensions.
- Materialization: covers dirty worktree, add/add, delete/modify, binary refusal. Missing: staged changes, staged-plus-unstaged, clean merge where ours/theirs are same, untouched dirty paths preserved.
- Script parsing: covers quoting, heredoc, missing terminator. Missing: backslash escaping in various positions, empty heredoc, heredoc with delimiter that matches a line in the middle, mixed quoting styles, tab-separated tokens, carriage returns.
- Guard tests: only `exists`/`contains` passing, `missing` failing. Missing: `exists` failing, `missing` passing, `contains` failing, guards combined with mutations where guard fails.

---

## Deferred Spec Features Not Yet Needed

These are called out as deferred in the spec and correctly absent from implementation. Listed for completeness:

- Pinned execution (`--require-plan-hash`)
- Multi-execution transactions (`tx begin`/`commit`/`abort`)
- `-C` / `--cwd` root override
- Query/read verbs (`get`, `keys`, etc.)
- Task list items and inline field verbs
- Markdown `append-section`, `prepend-section`, `delete-section`, `move-section`
- Ordered-list `prepend` and positional `insert`
- Cross-file transforms
- Configurable commit message templates
- `etch help aliases`
- Machine-readable error codes / JSON error mode
- Separate exit code for retry-budget exhaustion
- Validation benchmark harness (binary exists at `cmd/etch-validate` but is not the full spec'd harness)
