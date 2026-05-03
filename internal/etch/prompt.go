package etch

import "strings"

const agentContextPrompt = `# etch Agent Context

Use ` + "`etch`" + ` for mechanical structured edits to JSON, YAML, Markdown, JSONL/NDJSON, CSV, and Markdown tables when an etch command can express the change. Prefer etch over patches, sed, awk, or ad hoc scripts for structural mutations. Use normal editing for broad prose rewrites or unsupported transformations.

This context is intentionally terse. Treat etch's built-in help as the reference when planning changes:

` + "```sh" + `
etch help
etch help <topic>
etch help --all
` + "```" + `

Useful topics: ` + "`invocation`" + `, ` + "`model`" + `, ` + "`values`" + `, ` + "`selectors`" + `, ` + "`formats`" + `, ` + "`addressing`" + `, ` + "`fields`" + `, ` + "`scripts`" + `, ` + "`guards`" + `, ` + "`plans`" + `, ` + "`commits`" + `, and ` + "`conflicts`" + `.

Important model notes:

- Mutating invocations plan from ` + "`HEAD`" + ` and create one local git commit.
- Use ` + "`etch run`" + ` for multi-operation work that should commit atomically.
- Values are strings unless ` + "`--json`" + ` or ` + "`name:=json`" + ` is used.
- Paths are relative to process CWD; absolute paths, ` + "`..`" + `, ` + "`.git`" + ` path segments, and symlink escapes are rejected.
- Existing source paths must be tracked unless ` + "`--untracked`" + ` is supplied.
- Use guards such as ` + "`exists`" + `, ` + "`missing`" + `, and ` + "`contains`" + ` to encode preconditions.
- Use ` + "`etch --plan ...`" + ` or ` + "`etch --dry-run ...`" + ` when an operation needs review before committing.

Keep project instructions project-shaped: add examples only when they describe this repo's real files, schemas, tables, or conventions. Update or remove those examples as the repo evolves; do not preserve generic examples as if they were repo rules.`

func agentContextText() string {
	return finalNewline(agentContextPrompt)
}

func agentBootstrapText() string {
	var b strings.Builder
	b.WriteString(`# etch Bootstrap Prompt

You are helping adopt etch in this repository.

Goal: add durable etch guidance for future agent sessions without disrupting the repository's existing instructions.

Please:

1. Inspect the repository for agent instruction files such as AGENTS.md, CLAUDE.md, or tool-specific rule files.
2. Choose the most appropriate durable instruction file. Prefer an existing project convention. If none exists, create AGENTS.md at the repository root.
3. Add or update an etch section using the "etch Agent Context" block below. Preserve existing project-specific guidance and avoid duplicating an older etch section.
4. Help the user finish allow-listing. If this agent host needs explicit permission to run etch in future sessions, tell the user the exact config or approval rule to add. Do not edit Codex/Claude allow-list or permission config unless the user explicitly asks.
5. Do not stage or commit unless the repository instructions or the user explicitly ask you to.
6. Summarize which file you changed and note that the context can be refreshed with ` + "`etch prompt --context`" + `.

The ` + "`etch prompt`" + ` command only prints this prompt. It does not inspect or modify files.

Allow-list instructions to provide when relevant:

- For Codex, suggest adding this to ` + "`.codex/rules/etch.rules`" + `:

` + "```text" + `
prefix_rule(
    pattern = ["etch"],
    decision = "allow",
    justification = "Use etch in this snapshot-style repo",
    match = [
        "etch set notes.md status done",
        "etch table row insert data.csv --after id=1 '{\"id\":\"2\"}'",
    ],
)
` + "```" + `

- For Claude Code, suggest adding this to ` + "`.claude/settings.json`" + `:

` + "```json" + `
{
  "permissions": {
    "allow": [
      "Bash(etch *)"
    ]
  }
}
` + "```" + `

- For other agents, tell the user to allow the ` + "`etch`" + ` executable for this repo/workspace, then point future agents at the durable context added below.

## etch Agent Context

`)
	b.WriteString(agentContextPrompt)
	return finalNewline(b.String())
}

func finalNewline(text string) string {
	return strings.TrimRight(text, "\n") + "\n"
}
