package etch

import (
	"fmt"
	"io"
	"strings"
)

type HelpReference struct {
	Topics []HelpTopic `json:"topics"`
}

type HelpTopic struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Group      string      `json:"group"`
	Invocation string      `json:"invocation"`
	Summary    string      `json:"summary"`
	Aliases    []string    `json:"aliases,omitempty"`
	Blocks     []HelpBlock `json:"blocks"`
}

type HelpBlock struct {
	Kind    string           `json:"kind"`
	Heading string           `json:"heading,omitempty"`
	Text    string           `json:"text,omitempty"`
	Rows    []HelpCommandRow `json:"rows,omitempty"`
	Links   []HelpTopicLink  `json:"links,omitempty"`
}

type HelpCommandRow struct {
	Signature   string       `json:"signature"`
	Class       CommandClass `json:"class"`
	Description string       `json:"description"`
}

type HelpTopicLink struct {
	Title string `json:"title"`
	ID    string `json:"id"`
}

const shortHelp = `usage: etch [flags] <command> [args...]
       etch run [script]

Global flags must appear before the command path.

Global flags:
  --plan                 emit canonical JSON plan
  -n, --dry-run          emit git-am-compatible patch preview
  --no-checkout          commit without materializing touched paths
  --untracked            admit untracked source paths under CWD
  --message <m>          override generated commit message
  --subject-prefix <s>   prepend literal text to generated commit subject
  --subject-suffix <s>   append literal text to generated commit subject
  --body-prefix <s>      prepend a block to generated commit body
  --body-suffix <s>      append a block to generated commit body
  --retries <n>          retry CAS conflicts, default 3
  --allow-empty          permit empty commit for mutating invocations
  --version              print version and exit

Use "etch help" for common commands, "etch help scripts" for batch scripts,
"etch help --all" for advanced commands, or "etch prompt" for agent setup.
`

const (
	helpGroupCommands   = "Commands"
	helpGroupBasics     = "Basics"
	helpGroupData       = "Data and Addressing"
	helpGroupFamilies   = "Command Families"
	helpGroupExecution  = "Execution and Safety"
	helpCommandSummary  = "etch mutates structured files and commits each successful mutating invocation."
	helpGlobalTopicText = "model, invocation, prompts, scripts, selectors, values, formats, addressing, fields, files, guards, section, tasks, table, plans, commits, security, conflicts"
)

func printHelp(w io.Writer, topic string, all bool) error {
	if topic == "" {
		writeHelpTopic(w, commandHelpTopic(all))
		return nil
	}
	helpTopic, ok := helpTopicByName(topic)
	if !ok {
		return usagef("unknown help topic %s", topic)
	}
	writeHelpTopic(w, helpTopic)
	return nil
}

func writeHelpTopic(w io.Writer, topic HelpTopic) {
	if topic.Summary != "" {
		fmt.Fprintln(w, topic.Summary)
		fmt.Fprintln(w)
	}
	for _, block := range topic.Blocks {
		writeHelpBlock(w, block)
	}
}

func writeHelpBlock(w io.Writer, block HelpBlock) {
	switch block.Kind {
	case "paragraph":
		fmt.Fprintln(w, block.Text)
		fmt.Fprintln(w)
	case "heading":
		fmt.Fprintf(w, "%s:\n", block.Heading)
	case "pre":
		fmt.Fprintln(w, block.Text)
		fmt.Fprintln(w)
	case "command-table":
		fmt.Fprintf(w, "%s:\n", block.Heading)
		for _, row := range block.Rows {
			fmt.Fprintf(w, "  %-31s %-16s %s\n", row.Signature, row.Class, row.Description)
		}
		fmt.Fprintln(w)
	case "topic-links":
		fmt.Fprintf(w, "Topics: %s\n", block.Text)
	}
}

func BuildHelpReference() HelpReference {
	topics := []HelpTopic{
		commandHelpTopic(false),
		commandHelpTopic(true),
	}
	topics = append(topics, namedHelpTopics()...)
	return HelpReference{Topics: topics}
}

func helpTopicByName(name string) (HelpTopic, bool) {
	for _, topic := range namedHelpTopics() {
		for _, alias := range topic.Aliases {
			if name == alias {
				return topic, true
			}
		}
	}
	return HelpTopic{}, false
}

func commandHelpTopic(all bool) HelpTopic {
	title := "Common Commands"
	invocation := "etch help"
	heading := "Common commands"
	linksText := helpGlobalTopicText + ". Use --all for advanced commands."
	if all {
		title = "Command Index"
		invocation = "etch help --all"
		heading = "Commands"
		linksText = helpGlobalTopicText
	}

	blocks := []HelpBlock{
		{
			Kind:    "command-table",
			Heading: heading,
			Rows:    helpCommandRows(all),
		},
	}
	if all {
		blocks = append(blocks, HelpBlock{
			Kind: "paragraph",
			Text: "Format-explicit command prefixes select the parser and writer; they do not infer or validate the format from the file extension.",
		})
	}
	blocks = append(blocks, HelpBlock{
		Kind:  "topic-links",
		Text:  linksText,
		Links: helpTopicLinks(),
	})

	return HelpTopic{
		ID:         referenceID(title),
		Title:      title,
		Group:      helpGroupCommands,
		Invocation: invocation,
		Summary:    helpCommandSummary,
		Blocks:     blocks,
	}
}

func helpCommandRows(all bool) []HelpCommandRow {
	rows := []HelpCommandRow{
		{
			Signature:   "prompt [--context|--bootstrap]",
			Class:       ClassIntrospection,
			Description: "Print agent setup or durable context prompts.",
		},
	}
	for _, v := range verbCatalog() {
		if !v.Canonical || (!all && isPlumbingVerb(v)) {
			continue
		}
		rows = append(rows, HelpCommandRow{
			Signature:   v.Signature,
			Class:       v.Class,
			Description: v.Description,
		})
	}
	return rows
}

func helpTopicLinks() []HelpTopicLink {
	return []HelpTopicLink{
		{Title: "Model", ID: "model"},
		{Title: "Invocation", ID: "invocation"},
		{Title: "Prompts", ID: "prompts"},
		{Title: "Scripts", ID: "scripts"},
		{Title: "Selectors", ID: "selectors"},
		{Title: "Values", ID: "values"},
		{Title: "Formats", ID: "formats"},
		{Title: "Markdown Addressing", ID: "markdown-addressing"},
		{Title: "Inline Fields", ID: "fields"},
		{Title: "Files", ID: "files"},
		{Title: "Guards", ID: "guards"},
		{Title: "Sections", ID: "sections"},
		{Title: "Tasks", ID: "tasks"},
		{Title: "Tables and CSV", ID: "tables-and-csv"},
		{Title: "Plans", ID: "plans"},
		{Title: "Commits", ID: "commits"},
		{Title: "Security", ID: "security"},
		{Title: "Conflicts", ID: "conflicts"},
	}
}

func namedHelpTopics() []HelpTopic {
	return []HelpTopic{
		{
			ID:         "model",
			Title:      "Model",
			Group:      helpGroupBasics,
			Invocation: "etch help model",
			Summary:    "Mutating invocations plan from HEAD and commit as one transaction.",
			Aliases:    []string{"model"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "For tracked paths, etch reads base-tree bytes from HEAD, not from the live index or working tree. Dirty checkout edits are treated as concurrent local state and reconciled after the commit lands."},
				{Kind: "paragraph", Text: "All operations in one invocation are planned together. If any operation fails, no commit is created. If every mutating operation is an idempotent no-op, etch exits 0 without creating a commit unless --allow-empty is supplied."},
				{Kind: "paragraph", Text: "After a successful commit, etch materializes touched paths into the index and working tree unless --no-checkout is used."},
			},
		},
		{
			ID:         "invocation",
			Title:      "Invocation",
			Group:      helpGroupBasics,
			Invocation: "etch help invocation",
			Summary:    "Global flags appear before the command path; CWD is the path and capability boundary.",
			Aliases:    []string{"invocation", "flags"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Shapes"},
				{Kind: "pre", Text: `  etch [flags] <command> [args...]
  etch run [script]
  etch --plan <command> [args...]
  etch --dry-run <command> [args...]`},
				{Kind: "paragraph", Text: "Path operands are relative to process CWD, not the repository root. Absolute paths, .. segments, .git path segments, and symlink escapes are rejected. Mutating invocations require CWD to be inside a git worktree."},
				{Kind: "paragraph", Text: "Existing source paths must be tracked by default. --untracked admits untracked source paths under the same CWD boundary; those paths become tracked if the invocation commits."},
				{Kind: "paragraph", Text: "--plan emits canonical JSON and --dry-run emits a git-am-compatible patch preview without side effects. --no-checkout skips post-commit checkout synchronization. --retries controls optimistic ref-CAS retry attempts."},
				{Kind: "paragraph", Text: "--message replaces the generated commit message. --subject-prefix, --subject-suffix, --body-prefix, and --body-suffix modify generated messages and are mutually exclusive with --message."},
			},
		},
		{
			ID:         "prompts",
			Title:      "Prompts",
			Group:      helpGroupBasics,
			Invocation: "etch help prompts",
			Summary:    "etch prompt prints agent-facing Markdown. By default it prints a one-shot bootstrap prompt; --context prints durable instructions for future agent context.",
			Aliases:    []string{"prompts", "prompt"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Commands"},
				{Kind: "pre", Text: `  etch prompt
  etch prompt --bootstrap
  etch prompt --context`},
				{Kind: "paragraph", Text: "etch prompt has no side effects. The default bootstrap prompt is meant to start an agent, such as codex \"$(etch prompt)\" or claude \"$(etch prompt)\", so the agent can install durable etch guidance into this repository's agent instructions."},
				{Kind: "paragraph", Text: "etch prompt --context prints the durable guidance directly. Add that text to AGENTS.md, CLAUDE.md, or another project instruction file when you want future agents to know when and how to use etch."},
			},
		},
		{
			ID:         "scripts",
			Title:      "Scripts",
			Group:      helpGroupBasics,
			Invocation: "etch help scripts",
			Summary:    "etch run [script] executes a batch script as one transaction.",
			Aliases:    []string{"scripts"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: `The script path is optional. Omit it or pass "-" to read the script from stdin.
Every statement is planned together against one base tree, so later statements see earlier statements.
If parsing, guards, or mutations fail, the batch produces no commit.
On success, the whole batch produces one commit unless every mutating statement is a no-op.`},
				{Kind: "paragraph", Text: "Script lines use shell-style quoting, but no shell expansion. Quote values with spaces.\nQuote JSON values as one token, usually with single quotes:"},
				{Kind: "pre", Text: `  set posts/hello.md title "Hello, world"
  append events.jsonl '{"kind":"prompt","name":"first"}'
  set state.json payload --json '{"name":"first"}'`},
				{Kind: "paragraph", Text: "Multi-line values use heredocs. Heredoc bodies are literal text, and the\nterminator line contains only the delimiter:"},
				{Kind: "pre", Text: `section replace posts/hello.md "## Summary" <<EOF
$FOO is not expanded.
EOF`},
			},
		},
		{
			ID:         "selectors",
			Title:      "Selectors",
			Group:      helpGroupData,
			Invocation: "etch help selectors",
			Summary:    "Selectors are singular JSONPath-style paths.",
			Aliases:    []string{"selectors"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Accepted"},
				{Kind: "pre", Text: `  $.agents.assistant.last_run
  agents.assistant.last_run
  $.items[0].title
  $["key.with.dots"]`},
				{Kind: "paragraph", Text: "Rejected: wildcards, recursive descent, slices, filters, unions, functions, negative indexes."},
				{Kind: "paragraph", Text: "set may create missing object containers and final object members. append and add may create a missing final array under object-container rules. Syntax that can produce multiple matches is rejected before evaluation."},
			},
		},
		{
			ID:         "values",
			Title:      "Values",
			Group:      helpGroupData,
			Invocation: "etch help values",
			Summary:    "Structured values are strings by default. Use --json for a strict JSON value. For structured commands, --json may appear immediately before or after the value.",
			Aliases:    []string{"values"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Examples"},
				{Kind: "pre", Text: `  etch set state.json status complete          # string "complete"
  etch set state.json count --json 12          # number 12
  etch set state.json count 12 --json          # number 12
  etch append state.json events --json '{"kind":"prompt"}'
  etch append events.jsonl '{"kind":"prompt"}'
  etch set state.json status=complete count:=12`},
				{Kind: "paragraph", Text: "Assignment items are accepted by set only. NAME=value writes a string; NAME:=json writes JSON.\nJSONL and NDJSON append values are always strict JSON and do not use --json."},
				{Kind: "paragraph", Text: "For add and remove, array membership uses semantic structural equality within the edited format, not byte-spelling equality."},
			},
		},
		{
			ID:         "formats",
			Title:      "Formats",
			Group:      helpGroupData,
			Invocation: "etch help formats",
			Summary:    "Porcelain commands infer formats from path extensions; plumbing commands make the format explicit.",
			Aliases:    []string{"formats", "format", "json", "jsonl", "yaml", "frontmatter"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "Porcelain commands infer .json, .jsonl/.ndjson, .yaml/.yml, .md/.markdown, and .csv behavior from the path. Format-explicit commands such as json set, yaml set, frontmatter set, jsonl append, md table, and csv row append skip inference."},
				{Kind: "paragraph", Text: "JSON and YAML structured commands share selector and value semantics. JSON edits preserve surrounding representation where possible. YAML and frontmatter edits preserve comments, key order, anchors, aliases, indentation, and scalar spelling where the parser can preserve them."},
				{Kind: "paragraph", Text: "For Markdown paths, bare structured selectors target YAML frontmatter. If frontmatter is missing, set, append, and add can create it; delete and remove are no-ops when the final target is absent."},
				{Kind: "paragraph", Text: "JSONL and NDJSON append has no selector. The value is always strict JSON, missing logs are treated as empty, and non-empty logs must end at a record boundary."},
			},
		},
		{
			ID:         "fields",
			Title:      "Inline Fields",
			Group:      helpGroupData,
			Invocation: "etch help fields",
			Summary:    "Markdown fields: use frontmatter for note-global metadata; use inline fields for body-local metadata.",
			Aliases:    []string{"fields"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "On Markdown paths, bare structured selectors mutate YAML frontmatter. Markdown address flags such as --body, --section, --item, and --task switch set and delete to Dataview-style inline fields in the Markdown body."},
				{Kind: "paragraph", Text: "Frontmatter fits whole-note schema fields such as owner, source, status, and stable IDs. Inline fields fit metadata attached to a paragraph, list item, task, or local note context."},
				{Kind: "paragraph", Text: "Existing fields match first by exact source field name, then by Dataview-normalized field name if the normalized match is unique. Dataview implicit fields such as file.name, task status, and task text are reserved and not writable."},
				{Kind: "heading", Heading: "Examples"},
				{Kind: "pre", Text: `  etch set note.md status Driving
  etch set note.md last "2026-05-02" --body
  etch set note.md done "2026-05-01" --task "Send follow-up"`},
			},
		},
		{
			ID:         "markdown-addressing",
			Title:      "Markdown Addressing",
			Group:      helpGroupData,
			Invocation: "etch help addressing",
			Summary:    "Markdown addressing uses exact matching with syntax normalization and ambiguity errors.",
			Aliases:    []string{"addressing", "markdown"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: `Section selectors accept either a title such as "Status" or an ATX heading such as "## Status".
Title-only selectors search all ATX heading levels; ATX selectors include the heading level.`},
				{Kind: "paragraph", Text: `List-item selectors normalize away the list marker, task checkbox, surrounding whitespace,
and Dataview inline field annotations. Inline Markdown remains source text, so "**Buy milk**"
matches "**Buy milk**", not "Buy milk".`},
				{Kind: "paragraph", Text: "Item type filters are task, plain, numbered, and bullet. Repeated filters combine across\nindependent axes, such as task+numbered. Contradictory filters fail before planning."},
				{Kind: "paragraph", Text: "Placement flags such as --section, --before, and --after identify where Markdown list and task commands operate. --before and --after match list items, not arbitrary prose."},
			},
		},
		{
			ID:         "files",
			Title:      "Files",
			Group:      helpGroupFamilies,
			Invocation: "etch help files",
			Summary:    "File verbs create, replace, delete, move, and copy whole paths inside the transaction.",
			Aliases:    []string{"files", "file"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Commands"},
				{Kind: "pre", Text: `  etch create <path> [<content>]
  etch replace <path> <content>
  etch delete <path>
  etch move <src> <dst>
  etch copy <src> <dst>`},
				{Kind: "paragraph", Text: "create no-ops when an existing file already has the same content and fails when it has different content. Omitted create content uses {} for JSON paths and empty content for other paths."},
				{Kind: "paragraph", Text: "replace requires an existing regular file. delete is idempotent when the path is absent from the transaction base. move and copy fail if their destination exists in the transaction base."},
			},
		},
		{
			ID:         "guards",
			Title:      "Guards",
			Group:      helpGroupFamilies,
			Invocation: "etch help guards",
			Summary:    "Guards assert preconditions for a transaction without printing values or making content changes.",
			Aliases:    []string{"guards", "guard"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Commands"},
				{Kind: "pre", Text: `  etch exists <path>
  etch missing <path>
  etch contains <path> <literal>`},
				{Kind: "paragraph", Text: "For tracked paths, guards read the admitted input view at HEAD, not dirty working-tree bytes. With --untracked, admitted untracked paths use working-tree bytes."},
				{Kind: "paragraph", Text: "A failed guard exits 1 before mutation side effects. A satisfied guard contributes no content change and never creates a commit by itself. Guards are included in plans and dry-run output as checks."},
				{Kind: "paragraph", Text: "contains matches literal bytes. It is not a regex, does not normalize line endings, and does not perform case folding. Multi-line literals use heredocs."},
			},
		},
		{
			ID:         "sections",
			Title:      "Sections",
			Group:      helpGroupFamilies,
			Invocation: "etch help section",
			Summary:    "Markdown sections are heading-delimited body ranges.",
			Aliases:    []string{"section"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Commands"},
				{Kind: "pre", Text: `  etch section replace note.md "## Status" "done"
  etch section append note.md Status "new block"
  etch section prepend note.md Status "new block"`},
				{Kind: "paragraph", Text: `Section selectors accept either a title such as Status or an ATX heading such as ## Status.
Repeated matching headings are ambiguous. Section payloads are Markdown block fragments:
replace preserves existing boundary blank lines, and append/prepend use one blank line
between non-empty fragments.`},
			},
		},
		{
			ID:         "tasks",
			Title:      "Tasks and Lists",
			Group:      helpGroupFamilies,
			Invocation: "etch help tasks",
			Summary:    "Markdown task/list commands operate on exact source-normalized item text.",
			Aliases:    []string{"tasks"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "Commands"},
				{Kind: "pre", Text: `  etch task close note.md "Send follow-up" --section Actions
  etch task open note.md "Send follow-up" --section Actions
  etch list add note.md "Launch notes" --section Actions
  etch task add note.md "Send follow-up" --section Actions`},
				{Kind: "paragraph", Text: `task close changes [ ] to [x] and never creates missing tasks.
task open ensures an open task: it reopens [x]/[X], no-ops on [ ], and creates
missing tasks only when a destination address such as --section, --before, or
--after is supplied. Custom checkbox statuses fail.
--before and --after match list items, not arbitrary prose.
list add and task add create source from plain item text; do not include "- " or "- [ ]".
Without --section, --before, or --after, list insertion succeeds only when
there is a single obvious list target.`},
			},
		},
		{
			ID:         "tables-and-csv",
			Title:      "Tables and CSV",
			Group:      helpGroupFamilies,
			Invocation: "etch help table",
			Summary:    "Tables are ordered rows and named columns of string cells.",
			Aliases:    []string{"table", "csv"},
			Blocks: []HelpBlock{
				{Kind: "heading", Heading: "CSV"},
				{Kind: "pre", Text: `  etch table set data.csv all,status done
  etch table row append data.csv '{"id":"1","status":"open"}'
  etch table column add data.csv owner --default Brandon`},
				{Kind: "heading", Heading: "Markdown"},
				{Kind: "pre", Text: `  etch table set notes.md doc @0 all,status done
  etch table row append notes.md "## Inventory" '{"sku":"A1"}'`},
				{Kind: "paragraph", Text: "For Markdown paths, scope is doc or an exact heading selector, and the optional table ordinal is @0, @1, and so on. For CSV paths, scope and table ordinal are omitted because the file is one table."},
				{Kind: "paragraph", Text: "Ranges have the shape rows,columns. Rows and columns support all, zero-based ordinals, ordinal ranges, exact labels, and bracketed labels for spaces or selector punctuation. row-json is a strict JSON object keyed by column label."},
			},
		},
		{
			ID:         "plans",
			Title:      "Plans",
			Group:      helpGroupExecution,
			Invocation: "etch help plans",
			Summary:    "--plan emits canonical JSON describing operations, input/output hashes, planned tree, and commit message.",
			Aliases:    []string{"plans", "plan", "dry-run"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "A plan is computed by the same parser and evaluators as execution, stopped before side effects. It is based on HEAD for tracked paths and includes enough hashes to detect stale inputs and behavior changes."},
				{Kind: "paragraph", Text: "--dry-run lowers the same plan to a mailbox patch intended for git am. Dry-run output is optimized for review; plan JSON is the canonical machine contract."},
				{Kind: "paragraph", Text: "The plan hash is a SHA-256 hash of canonical JSON bytes and can be used by host runtimes as an approval-cache key."},
			},
		},
		{
			ID:         "commits",
			Title:      "Commits",
			Group:      helpGroupExecution,
			Invocation: "etch help commits",
			Summary:    "Every successful mutating invocation creates one local git commit unless all mutating operations are no-ops.",
			Aliases:    []string{"commits", "commit", "messages"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "Generated commit messages are built from normalized operation descriptors with bounded value previews. Full values live in file contents and are represented in plans by hashes."},
				{Kind: "paragraph", Text: "Single-op commits use subjects such as etch set note.md title \"Hello\" when the preview fits. Multi-op commits use a summary subject and a Changes body."},
				{Kind: "paragraph", Text: "--message replaces the generated message. --subject-prefix, --subject-suffix, --body-prefix, and --body-suffix modify generated messages and are mutually exclusive with --message."},
				{Kind: "paragraph", Text: "Idempotent no-op operations do not contribute to the commit. If every mutating operation is a no-op, etch exits 0 with nothing to do unless --allow-empty is supplied."},
			},
		},
		{
			ID:         "security",
			Title:      "Security",
			Group:      helpGroupExecution,
			Invocation: "etch help security",
			Summary:    "etch is designed as a narrow CWD-scoped, git-backed mutation capability.",
			Aliases:    []string{"security", "paths"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "etch accepts only relative paths under CWD, rejects .. and .git path segments, and refuses symlink escapes. There is no repo-root mode, script-local root, or etch-specific environment variable that changes the path root."},
				{Kind: "paragraph", Text: "etch does not perform network operations. The implementation invokes git only for local repository/object/ref/index work needed to plan, commit, preview, and materialize changes."},
				{Kind: "paragraph", Text: "The script syntax has no variables, command substitution, globbing, pipes, conditionals, or process execution. Composition happens outside etch."},
			},
		},
		{
			ID:         "conflicts",
			Title:      "Conflicts",
			Group:      helpGroupExecution,
			Invocation: "etch help conflicts",
			Summary:    "When materialization cannot merge local checkout changes after the commit lands, etch leaves recovery text on stderr.",
			Aliases:    []string{"conflicts", "checkout"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "The ref update is the durability boundary. If materialization fails, the commit exists and is not rolled back."},
				{Kind: "paragraph", Text: "Default materialization rebases touched index and working-tree states from old HEAD to new HEAD. Clean merges preserve staged and unstaged local changes; conflicts are written to the working tree when possible."},
				{Kind: "paragraph", Text: "Resolve conflict markers, then commit or discard the working-tree resolution. For binary or unmergeable local changes, etch fails cleanly without overwriting the current working-tree file."},
				{Kind: "paragraph", Text: "--no-checkout skips materialization entirely and reports that the working tree and index were not updated for touched paths."},
			},
		},
	}
}

func isPlumbingVerb(v VerbInfo) bool {
	for _, prefix := range []string{"json ", "jsonl ", "yaml ", "frontmatter ", "md ", "csv "} {
		if strings.HasPrefix(v.Name, prefix) {
			return true
		}
	}
	return false
}

func referenceID(title string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
