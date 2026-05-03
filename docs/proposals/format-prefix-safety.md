---
status: draft
depends_on: []
---

# Format Prefix Safety

## Summary

Reject or explicitly gate format-explicit plumbing commands when the command
prefix selects one whole-file structured adapter and the file extension clearly
names another Etch-supported structured format.

This addresses the adopter report that `etch yaml set test.json b --json 2`
can parse a JSON file as YAML and then write YAML-shaped output back to a
`.json` path. Documentation explains why the command behaves that way, but the
failure mode is sharp enough to deserve an execution guard.

## Candidate Commands

```sh
etch yaml set state.json b --json 2
etch --force-format yaml set state.json b --json 2
etch jsonl append events.json '{"kind":"prompt"}'
```

The first and third commands should fail without an explicit override. The
second shows the proposed narrow override shape if Etch keeps a way to perform
cross-extension plumbing intentionally.

## Semantics

- Porcelain commands keep inferring the parser and writer from the path.
- Format-explicit plumbing prefixes keep selecting the parser and writer.
- Before planning, Etch compares the explicit prefix with the path extension
  when the extension maps to a different Etch-supported structured format.
- Known mismatches fail with a clear diagnostic before any read-modify-write
  planning occurs.
- Unknown or extensionless paths are allowed; Etch does not content-sniff to
  guess a better format.
- The guard applies to whole-file structured prefixes such as JSON, YAML, and
  JSONL/NDJSON. Part-specific Markdown commands such as frontmatter operations
  should have their own path admission rules.
- A narrow `--force-format` override, if accepted, admits the mismatch but does
  not change parser, writer, or canonical plan semantics.

## Rationale

Format-explicit commands are valuable for scripts and unusual extensions, but
the common typo is destructive: the command succeeds, commits, and leaves a
path whose extension tells downstream tools to parse it differently from the
bytes Etch wrote. An early refusal keeps the plumbing surface powerful while
making the common mistake loud.

Warning-only behavior is weaker for agents and batch scripts because stderr can
be ignored while the commit still lands. A narrow override keeps intentional
cross-extension use possible without making every mismatch silent.

## Impact

Spec:

- Define extension-to-format admission for format-explicit commands.
- Define whether mismatches are usage errors or runtime errors.
- Define any override flag and its placement among global flags.

Docs:

- Update `help formats` with the mismatch rule and override, if accepted.
- Add examples showing porcelain inference versus explicit plumbing.

Code:

- Add path-extension admission checks before operation planning.
- Add tests for JSON/YAML, YAML/JSON, JSONL/JSON, unknown extensions, and
  extensionless files.
- Add tests proving no file content changes or commits occur on mismatch.

## Open Questions

- Should the override be named `--force-format`, `--force`, or omitted?
- Should a mismatch exit with code 2 because the invocation is invalid, or code
  1 because the target path is unsafe?
- Should `.yml` and `.yaml` be the only YAML extensions?
- How should future TOML and JSONC support extend the extension map?
