package etch

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type VerbInfo struct {
	Name        string       `json:"name"`
	Signature   string       `json:"signature"`
	Description string       `json:"description"`
	Class       CommandClass `json:"class"`
	Canonical   bool         `json:"canonical"`
}

func verbCatalog() []VerbInfo {
	return []VerbInfo{
		{"set", "set <path> <selector> <value>", "Set a JSON/YAML/frontmatter value.", ClassIdempotent, true},
		{"delete", "delete <path> [<selector>]", "Delete a file or selected JSON/YAML/frontmatter value.", ClassIdempotent, true},
		{"append", "append <path> <selector> <value>", "Append a value to an array.", ClassNonIdempotent, true},
		{"add", "add <path> <selector> <value>", "Ensure an array contains a value.", ClassIdempotent, true},
		{"remove", "remove <path> <selector> <value>", "Ensure an array does not contain a value.", ClassIdempotent, true},
		{"replace-section", "replace-section <path> <heading> <content>", "Replace the body under one Markdown ATX heading.", ClassIdempotent, true},
		{"create", "create <path> <content>", "Create a new file.", ClassIdempotent, true},
		{"move", "move <src> <dst>", "Move a file path.", ClassIdempotent, true},
		{"copy", "copy <src> <dst>", "Copy a file path.", ClassIdempotent, true},
		{"exists", "exists <path>", "Guard that a path exists in the admitted input view.", ClassGuard, true},
		{"missing", "missing <path>", "Guard that a path is missing in the admitted input view.", ClassGuard, true},
		{"contains", "contains <path> <literal>", "Guard that admitted file bytes contain a literal.", ClassGuard, true},
		{"json set", "json set <path> <selector> <value>", "Set a JSON value.", ClassIdempotent, true},
		{"json delete", "json delete <path> <selector>", "Delete a JSON value.", ClassIdempotent, true},
		{"json append", "json append <path> <selector> <value>", "Append to a JSON array.", ClassNonIdempotent, true},
		{"json add", "json add <path> <selector> <value>", "Ensure a JSON array contains a value.", ClassIdempotent, true},
		{"json remove", "json remove <path> <selector> <value>", "Remove matching values from a JSON array.", ClassIdempotent, true},
		{"yaml set", "yaml set <path> <selector> <value>", "Set a YAML value.", ClassIdempotent, true},
		{"yaml delete", "yaml delete <path> <selector>", "Delete a YAML value.", ClassIdempotent, true},
		{"yaml append", "yaml append <path> <selector> <value>", "Append to a YAML sequence.", ClassNonIdempotent, true},
		{"yaml add", "yaml add <path> <selector> <value>", "Ensure a YAML sequence contains a value.", ClassIdempotent, true},
		{"yaml remove", "yaml remove <path> <selector> <value>", "Remove matching values from a YAML sequence.", ClassIdempotent, true},
		{"frontmatter set", "frontmatter set <path> <selector> <value>", "Set Markdown YAML frontmatter.", ClassIdempotent, true},
		{"frontmatter delete", "frontmatter delete <path> <selector>", "Delete Markdown YAML frontmatter.", ClassIdempotent, true},
		{"frontmatter append", "frontmatter append <path> <selector> <value>", "Append to a frontmatter sequence.", ClassNonIdempotent, true},
		{"frontmatter add", "frontmatter add <path> <selector> <value>", "Ensure a frontmatter sequence contains a value.", ClassIdempotent, true},
		{"frontmatter remove", "frontmatter remove <path> <selector> <value>", "Remove matching values from a frontmatter sequence.", ClassIdempotent, true},
		{"md replace-section", "md replace-section <path> <heading> <content>", "Replace the body under one Markdown ATX heading.", ClassIdempotent, true},
		{"table set", "table set <path> [<scope> [<table>]] <range> <value>", "Set CSV or Markdown table cells.", ClassIdempotent, true},
		{"table row append", "table row append <path> [<scope> [<table>]] <row-json>", "Append a CSV or Markdown table row.", ClassNonIdempotent, true},
		{"table row insert", "table row insert <path> [<scope> [<table>]] (--before <row>|--after <row>) <row-json>", "Insert a table row.", ClassNonIdempotent, true},
		{"table row delete", "table row delete <path> [<scope> [<table>]] <row>", "Delete table rows.", ClassIdempotent, true},
		{"table column add", "table column add <path> [<scope> [<table>]] <column> [--after <column>] [--default <value>]", "Add a table column.", ClassIdempotent, true},
		{"table column rename", "table column rename <path> [<scope> [<table>]] <old-column> <new-column>", "Rename a table column.", ClassIdempotent, true},
		{"table column delete", "table column delete <path> [<scope> [<table>]] <column>", "Delete a table column.", ClassIdempotent, true},
		{"csv set", "csv set <path> <range> <value>", "Set CSV cells.", ClassIdempotent, true},
		{"csv row append", "csv row append <path> <row-json>", "Append a CSV row.", ClassNonIdempotent, true},
		{"csv row insert", "csv row insert <path> (--before <row>|--after <row>) <row-json>", "Insert a CSV row.", ClassNonIdempotent, true},
		{"csv row delete", "csv row delete <path> <row>", "Delete CSV rows.", ClassIdempotent, true},
		{"csv column add", "csv column add <path> <column> [--after <column>] [--default <value>]", "Add a CSV column.", ClassIdempotent, true},
		{"csv column rename", "csv column rename <path> <old-column> <new-column>", "Rename a CSV column.", ClassIdempotent, true},
		{"csv column delete", "csv column delete <path> <column>", "Delete a CSV column.", ClassIdempotent, true},
		{"md table set", "md table set <path> <scope> [<table>] <range> <value>", "Set Markdown table cells.", ClassIdempotent, true},
		{"md table row append", "md table row append <path> <scope> [<table>] <row-json>", "Append a Markdown table row.", ClassNonIdempotent, true},
		{"md table row insert", "md table row insert <path> <scope> [<table>] (--before <row>|--after <row>) <row-json>", "Insert a Markdown table row.", ClassNonIdempotent, true},
		{"md table row delete", "md table row delete <path> <scope> [<table>] <row>", "Delete Markdown table rows.", ClassIdempotent, true},
		{"md table column add", "md table column add <path> <scope> [<table>] <column> [--after <column>] [--default <value>]", "Add a Markdown table column.", ClassIdempotent, true},
		{"md table column rename", "md table column rename <path> <scope> [<table>] <old-column> <new-column>", "Rename a Markdown table column.", ClassIdempotent, true},
		{"md table column delete", "md table column delete <path> <scope> [<table>] <column>", "Delete a Markdown table column.", ClassIdempotent, true},
	}
}

func DecodeOperation(stmt Statement) (Operation, error) {
	t := stmt.Tokens
	if len(t) == 0 {
		return Operation{}, usagef("empty statement")
	}
	op := Operation{Raw: append([]string(nil), t...), Loc: stmt.Loc}
	switch t[0] {
	case "exists", "missing":
		if len(t) != 2 {
			return op, usagef("usage: etch %s <path>", t[0])
		}
		op.Verb, op.Kind, op.Class, op.Path = t[0], "guard", ClassGuard, t[1]
	case "contains":
		if len(t) != 3 {
			return op, usagef("usage: etch contains <path> <literal>")
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = "contains", "guard", ClassGuard, t[1], t[2]
	case "create":
		if len(t) != 3 {
			return op, usagef("usage: etch create <path> <content>")
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = "create", "file", ClassIdempotent, t[1], t[2]
		op.Target = PlanTarget{Path: t[1]}
	case "copy", "move":
		if len(t) != 3 {
			return op, usagef("usage: etch %s <src> <dst>", t[0])
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = t[0], "file", ClassIdempotent, t[1], t[2]
		op.Target = PlanTarget{Path: t[1]}
	case "delete":
		if len(t) == 2 {
			op.Verb, op.Kind, op.Class, op.Path = "delete", "file", ClassIdempotent, t[1]
			op.Target = PlanTarget{Path: t[1]}
		} else if len(t) == 3 {
			return decodeStructured(op, "infer", "delete", t[1], t[2], "")
		} else {
			return op, usagef("usage: etch delete <path> [<selector>]")
		}
	case "set", "append", "add", "remove":
		if len(t) != 4 {
			return op, usagef("usage: etch %s <path> <selector> <value>", t[0])
		}
		return decodeStructured(op, "infer", t[0], t[1], t[2], t[3])
	case "replace-section":
		if len(t) != 4 {
			return op, usagef("usage: etch replace-section <path> <heading> <content>")
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = "replace-section", "md-section", ClassIdempotent, t[1], t[3]
		op.Target = PlanTarget{Path: t[1], Part: "body", Section: t[2]}
	case "json", "yaml", "frontmatter":
		if len(t) < 4 {
			return op, usagef("usage: etch %s <verb> <path> <selector> [<value>]", t[0])
		}
		verb := t[1]
		needValue := verb == "set" || verb == "append" || verb == "add" || verb == "remove"
		if verb != "delete" && !needValue {
			return op, usagef("unknown %s verb %s", t[0], verb)
		}
		want := 4
		if needValue {
			want = 5
		}
		if len(t) != want {
			return op, usagef("usage: etch %s %s <path> <selector>%s", t[0], verb, valueUsage(needValue))
		}
		value := ""
		if needValue {
			value = t[4]
		}
		return decodeStructured(op, t[0], verb, t[2], t[3], value)
	case "md":
		return decodeMD(op, t)
	case "csv":
		return decodeCSV(op, t)
	case "table":
		return decodeTable(op, "infer", t)
	default:
		return op, usagef("unknown command %s", t[0])
	}
	fillDescriptor(&op)
	return op, nil
}

func valueUsage(need bool) string {
	if need {
		return " <value>"
	}
	return ""
}

func decodeStructured(op Operation, format, verb, path, selector, value string) (Operation, error) {
	op.Verb, op.Kind, op.Path, op.Value = verb, "structured", path, value
	if verb == "append" {
		op.Class = ClassNonIdempotent
	} else {
		op.Class = ClassIdempotent
	}
	part := ""
	actualSelector := selector
	if format == "frontmatter" {
		part = "frontmatter"
	} else if format == "infer" && isMarkdownPath(path) {
		if selector == "frontmatter" {
			part = "frontmatter"
			actualSelector = "$"
		} else if strings.HasPrefix(selector, "frontmatter.") {
			part = "frontmatter"
			actualSelector = strings.TrimPrefix(selector, "frontmatter.")
		} else {
			return op, usagef("markdown structured selectors must use frontmatter.*")
		}
	}
	norm, err := NormalizeSelector(actualSelector)
	if err != nil {
		return op, err
	}
	target := PlanTarget{Path: path, Selector: norm}
	if part != "" {
		target.Part = part
	}
	op.Target = target
	fillDescriptor(&op)
	return op, nil
}

func decodeMD(op Operation, t []string) (Operation, error) {
	if len(t) >= 2 && t[1] == "replace-section" {
		if len(t) != 5 {
			return op, usagef("usage: etch md replace-section <path> <heading> <content>")
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = "replace-section", "md-section", ClassIdempotent, t[2], t[4]
		op.Target = PlanTarget{Path: t[2], Part: "body", Section: t[3]}
		fillDescriptor(&op)
		return op, nil
	}
	if len(t) >= 3 && t[1] == "table" {
		nt := append([]string{"table"}, t[2:]...)
		return decodeTable(op, "md", nt)
	}
	return op, usagef("unknown md command")
}

func decodeCSV(op Operation, t []string) (Operation, error) {
	if len(t) < 2 {
		return op, usagef("unknown csv command")
	}
	nt := append([]string{"table"}, t[1:]...)
	return decodeTable(op, "csv", nt)
}

func decodeTable(op Operation, format string, t []string) (Operation, error) {
	if len(t) < 3 {
		return op, usagef("usage: etch table <subcommand> ...")
	}
	op.Kind = "table"
	op.Class = ClassIdempotent
	op.Verb = "table " + t[1]
	switch t[1] {
	case "set":
		return decodeTableSet(op, format, t)
	case "row":
		if len(t) < 4 {
			return op, usagef("usage: etch table row <append|insert|delete> ...")
		}
		op.Verb = "table row " + t[2]
		if t[2] == "append" || t[2] == "insert" {
			op.Class = ClassNonIdempotent
		}
		return decodeTableRow(op, format, t)
	case "column":
		if len(t) < 4 {
			return op, usagef("usage: etch table column <add|rename|delete> ...")
		}
		op.Verb = "table column " + t[2]
		return decodeTableColumn(op, format, t)
	default:
		return op, usagef("unknown table command %s", t[1])
	}
}

func decodeTableSet(op Operation, format string, t []string) (Operation, error) {
	if format == "csv" || (format == "infer" && isCSVPath(t[2])) {
		if len(t) != 5 {
			return op, usagef("usage: etch table set <path> <range> <value>")
		}
		op.Path, op.Value = t[2], t[4]
		op.Target = PlanTarget{Path: t[2], Part: "table", Range: t[3]}
		fillDescriptor(&op)
		return op, nil
	}
	if len(t) != 6 && len(t) != 7 {
		return op, usagef("usage: etch table set <path> <scope> [<table>] <range> <value>")
	}
	table := ""
	rangeArg := t[4]
	value := t[5]
	if len(t) == 7 {
		table, rangeArg, value = t[4], t[5], t[6]
	}
	op.Path, op.Value = t[2], value
	op.Target = PlanTarget{Path: t[2], Part: "table", Scope: t[3], Table: table, Range: rangeArg}
	fillDescriptor(&op)
	return op, nil
}

func decodeTableRow(op Operation, format string, t []string) (Operation, error) {
	sub := t[2]
	pathIndex := 3
	if len(t) <= pathIndex {
		return op, usagef("usage: etch table row %s <path> ...", sub)
	}
	path := t[pathIndex]
	op.Path = path
	csv := format == "csv" || (format == "infer" && isCSVPath(path))
	if sub == "append" {
		if csv {
			if len(t) != 5 {
				return op, usagef("usage: etch table row append <path> <row-json>")
			}
			op.Value = t[4]
			op.Target = PlanTarget{Path: path, Part: "table"}
		} else {
			if len(t) != 6 && len(t) != 7 {
				return op, usagef("usage: etch table row append <path> <scope> [<table>] <row-json>")
			}
			table, rowJSON := "", t[5]
			if len(t) == 7 {
				table, rowJSON = t[5], t[6]
			}
			op.Value = rowJSON
			op.Target = PlanTarget{Path: path, Part: "table", Scope: t[4], Table: table}
		}
		fillDescriptor(&op)
		return op, nil
	}
	if sub == "insert" {
		return decodeTableInsert(op, csv, t)
	}
	if sub == "delete" {
		if csv {
			if len(t) != 5 {
				return op, usagef("usage: etch table row delete <path> <row>")
			}
			op.Target = PlanTarget{Path: path, Part: "table", Row: t[4]}
		} else {
			if len(t) != 6 && len(t) != 7 {
				return op, usagef("usage: etch table row delete <path> <scope> [<table>] <row>")
			}
			table, row := "", t[5]
			if len(t) == 7 {
				table, row = t[5], t[6]
			}
			op.Target = PlanTarget{Path: path, Part: "table", Scope: t[4], Table: table, Row: row}
		}
		fillDescriptor(&op)
		return op, nil
	}
	return op, usagef("unknown table row command %s", sub)
}

func decodeTableInsert(op Operation, csv bool, t []string) (Operation, error) {
	path := t[3]
	if csv {
		if len(t) != 7 || (t[4] != "--before" && t[4] != "--after") {
			return op, usagef("usage: etch table row insert <path> (--before <row>|--after <row>) <row-json>")
		}
		op.Value = t[6]
		op.Target = PlanTarget{Path: path, Part: "table", Row: t[4] + " " + t[5]}
	} else {
		if len(t) != 8 && len(t) != 9 {
			return op, usagef("usage: etch table row insert <path> <scope> [<table>] (--before <row>|--after <row>) <row-json>")
		}
		table := ""
		flagIndex := 5
		if strings.HasPrefix(t[5], "@") {
			table = t[5]
			flagIndex = 6
		}
		if t[flagIndex] != "--before" && t[flagIndex] != "--after" {
			return op, usagef("table row insert requires --before or --after")
		}
		op.Value = t[flagIndex+2]
		op.Target = PlanTarget{Path: path, Part: "table", Scope: t[4], Table: table, Row: t[flagIndex] + " " + t[flagIndex+1]}
	}
	fillDescriptor(&op)
	return op, nil
}

func decodeTableColumn(op Operation, format string, t []string) (Operation, error) {
	sub := t[2]
	path := t[3]
	op.Path = path
	csv := format == "csv" || (format == "infer" && isCSVPath(path))
	if sub == "rename" {
		if csv {
			if len(t) != 6 {
				return op, usagef("usage: etch table column rename <path> <old-column> <new-column>")
			}
			op.Target = PlanTarget{Path: path, Part: "table", Column: t[4]}
			op.Value = t[5]
		} else {
			if len(t) != 7 && len(t) != 8 {
				return op, usagef("usage: etch table column rename <path> <scope> [<table>] <old-column> <new-column>")
			}
			table, old, newName := "", t[5], t[6]
			if len(t) == 8 {
				table, old, newName = t[5], t[6], t[7]
			}
			op.Target = PlanTarget{Path: path, Part: "table", Scope: t[4], Table: table, Column: old}
			op.Value = newName
		}
		fillDescriptor(&op)
		return op, nil
	}
	if sub == "delete" {
		if csv {
			if len(t) != 5 {
				return op, usagef("usage: etch table column delete <path> <column>")
			}
			op.Target = PlanTarget{Path: path, Part: "table", Column: t[4]}
		} else {
			if len(t) != 6 && len(t) != 7 {
				return op, usagef("usage: etch table column delete <path> <scope> [<table>] <column>")
			}
			table, col := "", t[5]
			if len(t) == 7 {
				table, col = t[5], t[6]
			}
			op.Target = PlanTarget{Path: path, Part: "table", Scope: t[4], Table: table, Column: col}
		}
		fillDescriptor(&op)
		return op, nil
	}
	if sub == "add" {
		return decodeTableColumnAdd(op, csv, t)
	}
	return op, usagef("unknown table column command %s", sub)
}

func decodeTableColumnAdd(op Operation, csv bool, t []string) (Operation, error) {
	path := t[3]
	base := 4
	scope, table := "", ""
	if !csv {
		if len(t) < 6 {
			return op, usagef("usage: etch table column add <path> <scope> [<table>] <column> [--after <column>] [--default <value>]")
		}
		scope = t[4]
		base = 5
		if strings.HasPrefix(t[5], "@") {
			table = t[5]
			base = 6
		}
	}
	if len(t) <= base {
		return op, usagef("table column add requires a column name")
	}
	col := t[base]
	after, def := "", ""
	for i := base + 1; i < len(t); i++ {
		if i+1 >= len(t) {
			return op, usagef("%s requires a value", t[i])
		}
		switch t[i] {
		case "--after":
			after = t[i+1]
		case "--default":
			def = t[i+1]
		default:
			return op, usagef("unknown table column add flag %s", t[i])
		}
		i++
	}
	op.Target = PlanTarget{Path: path, Part: "table", Scope: scope, Table: table, Column: col}
	op.Value = columnAddValue(after, def)
	fillDescriptor(&op)
	return op, nil
}

func columnAddValue(after, def string) string {
	m := map[string]string{"after": after, "default": def}
	b, _ := json.Marshal(m)
	return string(b)
}

func fillDescriptor(op *Operation) {
	parts := []string{op.Verb}
	if op.Path != "" {
		parts = append(parts, op.Path)
	}
	if op.Target.Part == "frontmatter" && op.Target.Selector != "" {
		if op.Target.Selector == "$" {
			parts = append(parts, "frontmatter")
		} else {
			parts = append(parts, "frontmatter."+strings.TrimPrefix(op.Target.Selector, "$."))
		}
	} else if op.Target.Selector != "" {
		parts = append(parts, op.Target.Selector)
	}
	if op.Target.Section != "" {
		parts = append(parts, shellQuote(op.Target.Section))
	}
	if op.Target.Range != "" {
		parts = append(parts, op.Target.Range)
	}
	if op.Target.Row != "" {
		parts = append(parts, op.Target.Row)
	}
	if op.Target.Column != "" {
		parts = append(parts, op.Target.Column)
	}
	if op.Value != "" && op.Kind != "file" {
		parts = append(parts, valuePreview(op.Value, 80))
	} else if op.Kind == "file" && (op.Verb == "create" || op.Verb == "copy" || op.Verb == "move") {
		if op.Verb == "create" {
			parts = append(parts, valuePreview(op.Value, 80))
		} else {
			parts = append(parts, op.Value)
		}
	}
	op.Descriptor = strings.Join(parts, " ")
	if op.Value != "" {
		op.ValueHash = shaHex([]byte(op.Value))
	}
}

func isJSONPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func isMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown"
}

func isCSVPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".csv")
}

const shortHelp = `usage: etch [--plan|--dry-run] [flags] <verb> [args...]
       etch run <script>

Core flags:
  --plan                 emit canonical JSON plan
  --dry-run              emit git-am-compatible patch preview
  --no-checkout          commit without materializing touched paths
  --untracked            admit untracked source paths under CWD
  --message <m>          override generated commit message
  --message-prefix <m>   prepend generated commit message
  --message-suffix <m>   append generated commit message
  --retries <n>          retry CAS conflicts, default 3
  --allow-empty          permit empty commit for mutating invocations

Use "etch help" for the porcelain verb table, or "etch help --all" for plumbing commands too.
`

func printHelp(w io.Writer, topic string, all bool) error {
	switch topic {
	case "":
		fmt.Fprintln(w, "etch mutates structured files and commits each successful mutating invocation.")
		fmt.Fprintln(w)
		if all {
			fmt.Fprintln(w, "Commands:")
		} else {
			fmt.Fprintln(w, "Porcelain commands:")
		}
		for _, v := range verbCatalog() {
			if !v.Canonical || (!all && isPlumbingVerb(v)) {
				continue
			}
			fmt.Fprintf(w, "  %-31s %-16s %s\n", v.Signature, v.Class, v.Description)
		}
		fmt.Fprintln(w)
		if all {
			fmt.Fprint(w, "Topics: model, selectors, values, plans, security, conflicts, table, csv\n")
		} else {
			fmt.Fprint(w, "Topics: model, selectors, values, plans, security, conflicts, table, csv. Use --all for plumbing commands.\n")
		}
	case "selectors":
		fmt.Fprint(w, selectorsHelp)
	case "values":
		fmt.Fprint(w, valuesHelp)
	case "plans":
		fmt.Fprint(w, plansHelp)
	case "security":
		fmt.Fprint(w, securityHelp)
	case "conflicts":
		fmt.Fprint(w, conflictsHelp)
	case "model":
		fmt.Fprint(w, modelHelp)
	case "table", "csv":
		fmt.Fprint(w, tableHelp)
	default:
		return usagef("unknown help topic %s", topic)
	}
	return nil
}

func isPlumbingVerb(v VerbInfo) bool {
	for _, prefix := range []string{"json ", "yaml ", "frontmatter ", "md ", "csv "} {
		if strings.HasPrefix(v.Name, prefix) {
			return true
		}
	}
	return false
}

const selectorsHelp = `Selectors are singular JSONPath-style paths.

Accepted:
  $.agents.assistant.last_run
  agents.assistant.last_run
  $.items[0].title
  $["key.with.dots"]

Rejected: wildcards, recursive descent, slices, filters, unions, functions, negative indexes.
`

const valuesHelp = `Values are parsed as strict JSON when they are valid JSON literals; otherwise they are strings.

Examples:
  true
  12
  "literal string"
  ["draft","intro"]
  {"status":"done"}
`

const plansHelp = `--plan emits JSON describing operations, input/output hashes, planned tree, and commit message.
--dry-run lowers the same plan to a mailbox patch intended for git am.
`

const securityHelp = `etch only accepts relative paths under CWD, rejects .. and .git path segments, and refuses symlink escapes.
It does not perform network operations. The implementation invokes git for repository/object/ref work.
`

const conflictsHelp = `When materialization cannot merge local checkout changes after the commit lands, etch leaves recovery text on stderr.
The commit is durable once the ref update succeeds; resolve conflict markers, then commit or discard the checkout resolution.
`

const modelHelp = `Mutating invocations read tracked inputs from HEAD, not from dirty checkout files.
All operations in one invocation are planned together and commit as one transaction unless every mutating operation is a no-op.
`

const tableHelp = `Tables are ordered rows and named columns of string cells.

CSV:
  etch table set data.csv all,status done
  etch table row append data.csv '{"id":"1","status":"open"}'
  etch table column add data.csv owner --default Brandon

Markdown:
  etch table set notes.md doc @0 all,status done
  etch table row append notes.md "## Inventory" '{"sku":"A1"}'
`
