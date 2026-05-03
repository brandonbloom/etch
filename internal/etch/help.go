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
or "etch help --all" for advanced commands too.
`

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
	linksText := helpTopicsText + ". Use --all for advanced commands."
	if all {
		title = "Command Index"
		invocation = "etch help --all"
		heading = "Commands"
		linksText = helpTopicsText
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
		Invocation: invocation,
		Summary:    "etch mutates structured files and commits each successful mutating invocation.",
		Blocks:     blocks,
	}
}

func helpCommandRows(all bool) []HelpCommandRow {
	var rows []HelpCommandRow
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

const helpTopicsText = "model, scripts, selectors, values, fields, plans, security, conflicts, addressing, section, tasks, table, csv"

func helpTopicLinks() []HelpTopicLink {
	return []HelpTopicLink{
		{Title: "Model", ID: "model"},
		{Title: "Scripts", ID: "scripts"},
		{Title: "Selectors", ID: "selectors"},
		{Title: "Values", ID: "values"},
		{Title: "Fields", ID: "fields"},
		{Title: "Plans", ID: "plans"},
		{Title: "Security", ID: "security"},
		{Title: "Conflicts", ID: "conflicts"},
		{Title: "Addressing", ID: "addressing"},
		{Title: "Sections", ID: "sections"},
		{Title: "Tasks", ID: "tasks"},
		{Title: "Tables and CSV", ID: "tables-and-csv"},
	}
}

func namedHelpTopics() []HelpTopic {
	return []HelpTopic{
		{
			ID:         "model",
			Title:      "Model",
			Invocation: "etch help model",
			Summary:    "Mutating invocations read tracked inputs from HEAD, not from dirty checkout files. All operations in one invocation are planned together and commit as one transaction unless every mutating operation is a no-op.",
			Aliases:    []string{"model"},
		},
		{
			ID:         "scripts",
			Title:      "Scripts",
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
			},
		},
		{
			ID:         "values",
			Title:      "Values",
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
			},
		},
		{
			ID:         "fields",
			Title:      "Fields",
			Invocation: "etch help fields",
			Summary:    "Markdown fields: use frontmatter for note-global metadata; use inline fields for body-local metadata.",
			Aliases:    []string{"fields"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: "Frontmatter fits whole-note schema fields such as owner, source, status, and stable IDs.\nInline fields fit metadata attached to a paragraph, list item, task, or local note context."},
				{Kind: "heading", Heading: "Examples"},
				{Kind: "pre", Text: `  etch set note.md status Driving
  etch set note.md last "2026-05-02" --body
  etch set note.md done "2026-05-01" --task "Send follow-up"`},
			},
		},
		{
			ID:         "plans",
			Title:      "Plans",
			Invocation: "etch help plans",
			Summary:    "--plan emits JSON describing operations, input/output hashes, planned tree, and commit message.",
			Aliases:    []string{"plans"},
			Blocks:     []HelpBlock{{Kind: "paragraph", Text: "--dry-run lowers the same plan to a mailbox patch intended for git am."}},
		},
		{
			ID:         "security",
			Title:      "Security",
			Invocation: "etch help security",
			Summary:    "etch only accepts relative paths under CWD, rejects .. and .git path segments, and refuses symlink escapes.",
			Aliases:    []string{"security"},
			Blocks:     []HelpBlock{{Kind: "paragraph", Text: "It does not perform network operations. The implementation invokes git for repository/object/ref work."}},
		},
		{
			ID:         "conflicts",
			Title:      "Conflicts",
			Invocation: "etch help conflicts",
			Summary:    "When materialization cannot merge local checkout changes after the commit lands, etch leaves recovery text on stderr.",
			Aliases:    []string{"conflicts"},
			Blocks:     []HelpBlock{{Kind: "paragraph", Text: "The commit is durable once the ref update succeeds; resolve conflict markers, then commit or discard the checkout resolution."}},
		},
		{
			ID:         "addressing",
			Title:      "Addressing",
			Invocation: "etch help addressing",
			Summary:    "Markdown addressing uses exact matching with syntax normalization and ambiguity errors.",
			Aliases:    []string{"addressing"},
			Blocks: []HelpBlock{
				{Kind: "paragraph", Text: `Section selectors accept either a title such as "Status" or an ATX heading such as "## Status".
Title-only selectors search all ATX heading levels; ATX selectors include the heading level.`},
				{Kind: "paragraph", Text: `List-item selectors normalize away the list marker, task checkbox, surrounding whitespace,
and Dataview inline field annotations. Inline Markdown remains source text, so "**Buy milk**"
matches "**Buy milk**", not "Buy milk".`},
				{Kind: "paragraph", Text: "Item type filters are task, plain, numbered, and bullet. Repeated filters combine across\nindependent axes, such as task+numbered. Contradictory filters fail before planning."},
			},
		},
		{
			ID:         "sections",
			Title:      "Sections",
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
			Title:      "Tasks",
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
