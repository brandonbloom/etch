package etch

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/brandonbloom/etch/internal/jsonx"
)

type VerbInfo struct {
	Name        string       `json:"name"`
	Signature   string       `json:"signature"`
	Description string       `json:"description"`
	Class       CommandClass `json:"class"`
	Canonical   bool         `json:"canonical"`
}

type commandParser func(commandInvocation) ([]Operation, error)

type commandSpec struct {
	Path        []string
	Signature   string
	Description string
	Class       CommandClass
	Canonical   bool
	LocalFlags  []string
	Parse       commandParser
}

type commandInvocation struct {
	Spec commandSpec
	Args []string
	Op   Operation
}

type commandMatch struct {
	Spec commandSpec
	Args []string
}

type structuredValueArgs struct {
	Path     string
	Selector string
	Value    string
	Mode     ValueMode
}

type assignmentItem struct {
	Selector string
	Value    string
	Mode     ValueMode
	Present  bool
}

type structuredCommandFamily struct {
	Format            string
	ValueDescription  string
	DeleteDescription string
	Sequence          string
}

type tableCommandFamily struct {
	Prefix    string
	Format    string
	ScopeArgs string
	Cells     string
	Row       string
	Rows      string
	Column    string
}

func (s commandSpec) name() string {
	return strings.Join(s.Path, " ")
}

func (s commandSpec) verbInfo() VerbInfo {
	return VerbInfo{
		Name:        s.name(),
		Signature:   s.Signature,
		Description: s.Description,
		Class:       s.Class,
		Canonical:   s.Canonical,
	}
}

func command(path, signature, description string, class CommandClass, canonical bool, parse commandParser, flags ...string) commandSpec {
	return commandSpec{
		Path:        strings.Fields(path),
		Signature:   signature,
		Description: description,
		Class:       class,
		Canonical:   canonical,
		LocalFlags:  append([]string(nil), flags...),
		Parse:       parse,
	}
}

func structuredCommands(f structuredCommandFamily) []commandSpec {
	return []commandSpec{
		command(f.Format+" set", f.Format+" set <path> <selector> <value>|<selector=value>...", "Set "+f.ValueDescription+".", ClassIdempotent, true, parseStructured(f.Format, "set"), "--json"),
		command(f.Format+" delete", f.Format+" delete <path> <selector>", f.DeleteDescription, ClassIdempotent, true, parseStructured(f.Format, "delete")),
		command(f.Format+" append", f.Format+" append <path> <selector> <value|--json value>", "Append to a "+f.Sequence+".", ClassNonIdempotent, true, parseStructured(f.Format, "append"), "--json"),
		command(f.Format+" add", f.Format+" add <path> <selector> <value|--json value>", "Ensure a "+f.Sequence+" contains a value.", ClassIdempotent, true, parseStructured(f.Format, "add"), "--json"),
		command(f.Format+" remove", f.Format+" remove <path> <selector> <value|--json value>", "Remove matching values from a "+f.Sequence+".", ClassIdempotent, true, parseStructured(f.Format, "remove"), "--json"),
	}
}

func tableCommands(f tableCommandFamily) []commandSpec {
	return []commandSpec{
		tableCommand(f, "set", "<range> <value>", "Set "+f.Cells+".", ClassIdempotent, parseTable(f.Format, "set")),
		tableCommand(f, "row append", "<row-json>", "Append a "+f.Row+".", ClassNonIdempotent, parseTable(f.Format, "row", "append")),
		tableCommand(f, "row insert", "(--before <row>|--after <row>) <row-json>", "Insert a "+f.Row+".", ClassNonIdempotent, parseTable(f.Format, "row", "insert"), "--before", "--after"),
		tableCommand(f, "row delete", "<row>", "Delete "+f.Rows+".", ClassIdempotent, parseTable(f.Format, "row", "delete")),
		tableCommand(f, "column add", "<column> [--after <column>] [--default <value>]", "Add a "+f.Column+".", ClassIdempotent, parseTable(f.Format, "column", "add"), "--after", "--default"),
		tableCommand(f, "column rename", "<old-column> <new-column>", "Rename a "+f.Column+".", ClassIdempotent, parseTable(f.Format, "column", "rename")),
		tableCommand(f, "column delete", "<column>", "Delete a "+f.Column+".", ClassIdempotent, parseTable(f.Format, "column", "delete")),
	}
}

func tableCommand(f tableCommandFamily, action, tail, description string, class CommandClass, parse commandParser, flags ...string) commandSpec {
	path := commandPhrase(f.Prefix, action)
	signature := commandPhrase(path, "<path>", f.ScopeArgs, tail)
	return command(path, signature, description, class, true, parse, flags...)
}

func commandPhrase(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

var allCommandSpecs = buildCommandSpecs()

func commandSpecs() []commandSpec {
	return allCommandSpecs
}

func buildCommandSpecs() []commandSpec {
	specs := []commandSpec{
		command("set", "set <path> <selector> <value>|<selector=value>...", "Set JSON/YAML/frontmatter values.", ClassIdempotent, true, parsePorcelainStructured("set"), "--json"),
		command("delete", "delete <path> [<selector>]", "Delete a file or selected JSON/YAML/frontmatter value.", ClassIdempotent, true, parseDelete),
		command("append", "append <path> <selector> <value|--json value>|<path.jsonl> <json-value>", "Append a value to an array, or a JSONL record to .jsonl/.ndjson.", ClassNonIdempotent, true, parsePorcelainAppend, "--json"),
		command("add", "add <path> <selector> <value|--json value>", "Ensure an array contains a value.", ClassIdempotent, true, parsePorcelainStructured("add"), "--json"),
		command("remove", "remove <path> <selector> <value|--json value>", "Ensure an array does not contain a value.", ClassIdempotent, true, parsePorcelainStructured("remove"), "--json"),
		command("section replace", "section replace <path> <heading> <content>", "Replace the body under one Markdown heading.", ClassIdempotent, true, parseSection("replace")),
		command("section append", "section append <path> <heading> <content>", "Append a block fragment under one Markdown heading.", ClassNonIdempotent, true, parseSection("append")),
		command("section prepend", "section prepend <path> <heading> <content>", "Prepend a block fragment under one Markdown heading.", ClassNonIdempotent, true, parseSection("prepend")),
		command("create", "create <path> [<content>]", "Create a new file; omitted content uses an extension-aware default.", ClassIdempotent, true, parseCreate),
		command("move", "move <src> <dst>", "Move a file path.", ClassIdempotent, true, parseFileVerb("move")),
		command("copy", "copy <src> <dst>", "Copy a file path.", ClassIdempotent, true, parseFileVerb("copy")),
		command("exists", "exists <path>", "Guard that a path exists in the admitted input view.", ClassGuard, true, parsePathGuard("exists")),
		command("missing", "missing <path>", "Guard that a path is missing in the admitted input view.", ClassGuard, true, parsePathGuard("missing")),
		command("contains", "contains <path> <literal>", "Guard that admitted file bytes contain a literal.", ClassGuard, true, parseContains),

		command("md section replace", "md section replace <path> <heading> <content>", "Replace the body under one Markdown heading.", ClassIdempotent, true, parseSection("replace")),
		command("md section append", "md section append <path> <heading> <content>", "Append a block fragment under one Markdown heading.", ClassNonIdempotent, true, parseSection("append")),
		command("md section prepend", "md section prepend <path> <heading> <content>", "Prepend a block fragment under one Markdown heading.", ClassNonIdempotent, true, parseSection("prepend")),
		command("jsonl append", "jsonl append <path> <json-value>", "Append one compact JSON value as a JSONL/NDJSON record.", ClassNonIdempotent, true, parseJSONLAppend),
	}
	for _, family := range []structuredCommandFamily{
		{Format: "json", ValueDescription: "JSON values", DeleteDescription: "Delete a JSON value.", Sequence: "JSON array"},
		{Format: "yaml", ValueDescription: "YAML values", DeleteDescription: "Delete a YAML value.", Sequence: "YAML sequence"},
		{Format: "frontmatter", ValueDescription: "Markdown YAML frontmatter", DeleteDescription: "Delete Markdown YAML frontmatter.", Sequence: "frontmatter sequence"},
	} {
		specs = append(specs, structuredCommands(family)...)
	}
	for _, family := range []tableCommandFamily{
		{
			Prefix:    "table",
			Format:    "infer",
			ScopeArgs: "[<scope> [<table>]]",
			Cells:     "CSV or Markdown table cells",
			Row:       "CSV or Markdown table row",
			Rows:      "table rows",
			Column:    "table column",
		},
		{
			Prefix: "csv",
			Format: "csv",
			Cells:  "CSV cells",
			Row:    "CSV row",
			Rows:   "CSV rows",
			Column: "CSV column",
		},
		{
			Prefix:    "md table",
			Format:    "md",
			ScopeArgs: "<scope> [<table>]",
			Cells:     "Markdown table cells",
			Row:       "Markdown table row",
			Rows:      "Markdown table rows",
			Column:    "Markdown table column",
		},
	} {
		specs = append(specs, tableCommands(family)...)
	}
	return specs
}

func verbCatalog() []VerbInfo {
	specs := commandSpecs()
	verbs := make([]VerbInfo, 0, len(specs))
	for _, spec := range specs {
		verbs = append(verbs, spec.verbInfo())
	}
	return verbs
}

func DecodeOperations(stmt Statement) ([]Operation, error) {
	invocation, err := parseCommandInvocation(stmt)
	if err != nil {
		return nil, err
	}
	return invocation.Spec.Parse(invocation)
}

func DecodeOperation(stmt Statement) (Operation, error) {
	ops, err := DecodeOperations(stmt)
	if err != nil {
		return Operation{}, err
	}
	if len(ops) != 1 {
		return Operation{}, usagef("statement expands to multiple operations")
	}
	return ops[0], nil
}

func oneOperation(op Operation, err error) ([]Operation, error) {
	if err != nil {
		return nil, err
	}
	return []Operation{op}, nil
}

func parsedOperation(op Operation) ([]Operation, error) {
	fillDescriptor(&op)
	return []Operation{op}, nil
}

func parseCommandInvocation(stmt Statement) (commandInvocation, error) {
	tokens := stmt.Tokens
	if len(tokens) == 0 {
		return commandInvocation{}, usagef("empty statement")
	}
	match, ok := matchCommandSpec(tokens)
	if !ok {
		return commandInvocation{}, usagef("unknown command %s", tokens[0])
	}
	return commandInvocation{
		Spec: match.Spec,
		Args: match.Args,
		Op:   Operation{Raw: append([]string(nil), tokens...), Loc: stmt.Loc},
	}, nil
}

func matchCommandSpec(tokens []string) (commandMatch, bool) {
	match := commandMatch{}
	bestLen := 0
	for _, spec := range commandSpecs() {
		if !commandPathMatches(tokens, spec.Path) {
			continue
		}
		if len(spec.Path) > bestLen {
			match = commandMatch{Spec: spec, Args: tokens[len(spec.Path):]}
			bestLen = len(spec.Path)
		}
	}
	if bestLen == 0 {
		return commandMatch{}, false
	}
	return match, true
}

func commandPathMatches(tokens, path []string) bool {
	if len(tokens) < len(path) {
		return false
	}
	for i, part := range path {
		if tokens[i] != part {
			return false
		}
	}
	return true
}

func parsePathGuard(verb string) commandParser {
	return func(inv commandInvocation) ([]Operation, error) {
		spec, op, args := inv.Spec, inv.Op, inv.Args
		if len(args) != 1 {
			return nil, usagef("usage: etch %s", spec.Signature)
		}
		op.Verb, op.Kind, op.Class, op.Path = verb, "guard", spec.Class, args[0]
		return parsedOperation(op)
	}
}

func parseContains(inv commandInvocation) ([]Operation, error) {
	spec, op, args := inv.Spec, inv.Op, inv.Args
	if len(args) != 2 {
		return nil, usagef("usage: etch %s", spec.Signature)
	}
	op.Verb, op.Kind, op.Class, op.Path, op.Value = "contains", "guard", spec.Class, args[0], args[1]
	return parsedOperation(op)
}

func parseCreate(inv commandInvocation) ([]Operation, error) {
	spec, op, args := inv.Spec, inv.Op, inv.Args
	if len(args) != 1 && len(args) != 2 {
		return nil, usagef("usage: etch %s", spec.Signature)
	}
	value := defaultCreateContent(args[0])
	if len(args) == 2 {
		value = args[1]
	}
	op.Verb, op.Kind, op.Class, op.Path, op.Value = "create", "file", spec.Class, args[0], value
	op.Target = PlanTarget{Path: args[0]}
	return parsedOperation(op)
}

func parseFileVerb(verb string) commandParser {
	return func(inv commandInvocation) ([]Operation, error) {
		spec, op, args := inv.Spec, inv.Op, inv.Args
		if len(args) != 2 {
			return nil, usagef("usage: etch %s", spec.Signature)
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = verb, "file", spec.Class, args[0], args[1]
		op.Target = PlanTarget{Path: args[0]}
		return parsedOperation(op)
	}
}

func parseDelete(inv commandInvocation) ([]Operation, error) {
	spec, op, args := inv.Spec, inv.Op, inv.Args
	switch len(args) {
	case 1:
		op.Verb, op.Kind, op.Class, op.Path = "delete", "file", spec.Class, args[0]
		op.Target = PlanTarget{Path: args[0]}
		return parsedOperation(op)
	case 2:
		return oneOperation(decodeStructured(op, "infer", "delete", structuredValueArgs{Path: args[0], Selector: args[1]}))
	default:
		return nil, usagef("usage: etch %s", spec.Signature)
	}
}

func parsePorcelainStructured(verb string) commandParser {
	return func(inv commandInvocation) ([]Operation, error) {
		op, args := inv.Op, inv.Args
		if verb == "set" {
			if ops, ok, err := decodeAssignmentSet(op, "infer", args); err != nil || ok {
				return ops, err
			}
		} else if len(args) >= 2 {
			item, _ := splitAssignmentItem(args[1])
			if item.Present {
				return nil, usagef("assignment items are only accepted by set")
			}
		}
		valueArgs, err := parseStructuredValueArgs(verb, args)
		if err != nil {
			return nil, err
		}
		return oneOperation(decodeStructured(op, "infer", verb, valueArgs))
	}
}

func parsePorcelainAppend(inv commandInvocation) ([]Operation, error) {
	if len(inv.Args) > 0 && isJSONLPath(inv.Args[0]) {
		return decodeJSONLAppend(inv, "append <path.jsonl> <json-value>")
	}
	return parsePorcelainStructured("append")(inv)
}

func parseStructured(format, verb string) commandParser {
	return func(inv commandInvocation) ([]Operation, error) {
		spec, op, args := inv.Spec, inv.Op, inv.Args
		if verb == "delete" {
			if len(args) != 2 {
				return nil, usagef("usage: etch %s", spec.Signature)
			}
			return oneOperation(decodeStructured(op, format, verb, structuredValueArgs{Path: args[0], Selector: args[1]}))
		}
		if verb == "set" {
			if ops, ok, err := decodeAssignmentSet(op, format, args); err != nil || ok {
				return ops, err
			}
		} else if len(args) >= 2 {
			item, _ := splitAssignmentItem(args[1])
			if item.Present {
				return nil, usagef("assignment items are only accepted by set")
			}
		}
		valueArgs, err := parseStructuredValueArgs(spec.name(), args)
		if err != nil {
			return nil, err
		}
		return oneOperation(decodeStructured(op, format, verb, valueArgs))
	}
}

func parseJSONLAppend(inv commandInvocation) ([]Operation, error) {
	return decodeJSONLAppend(inv, inv.Spec.Signature)
}

func decodeJSONLAppend(inv commandInvocation, signature string) ([]Operation, error) {
	op, args := inv.Op, inv.Args
	if len(args) != 2 {
		return nil, usagef("usage: etch %s", signature)
	}
	op.Verb, op.Kind, op.Class, op.Path, op.Value, op.ValueMode = inv.Spec.name(), "jsonl", inv.Spec.Class, args[0], args[1], ValueModeJSON
	op.Target = PlanTarget{Path: args[0]}
	return parsedOperation(op)
}

func parseSection(action string) commandParser {
	return func(inv commandInvocation) ([]Operation, error) {
		spec, op, args := inv.Spec, inv.Op, inv.Args
		if len(args) != 3 {
			return nil, usagef("usage: etch %s", spec.Signature)
		}
		op.Verb, op.Kind, op.Class, op.Path, op.Value = "section "+action, "md-section", spec.Class, args[0], args[2]
		op.Target = PlanTarget{Path: args[0], Part: "body", Section: args[1]}
		return parsedOperation(op)
	}
}

func parseTable(format string, tablePath ...string) commandParser {
	return func(inv commandInvocation) ([]Operation, error) {
		op, args := inv.Op, inv.Args
		tokens := make([]string, 0, 1+len(tablePath)+len(args))
		tokens = append(tokens, "table")
		tokens = append(tokens, tablePath...)
		tokens = append(tokens, args...)
		return oneOperation(decodeTable(op, format, tokens))
	}
}

func parseStructuredValueArgs(command string, args []string) (structuredValueArgs, error) {
	switch {
	case len(args) == 3:
		if args[2] == "--json" {
			return structuredValueArgs{}, usagef("--json requires a value")
		}
		return structuredValueArgs{Path: args[0], Selector: args[1], Value: args[2], Mode: ValueModeString}, nil
	case len(args) == 4 && args[2] == "--json":
		return structuredValueArgs{Path: args[0], Selector: args[1], Value: args[3], Mode: ValueModeJSON}, nil
	default:
		return structuredValueArgs{}, usagef("usage: etch %s <path> <selector> <value>", command)
	}
}

func decodeAssignmentSet(base Operation, format string, args []string) ([]Operation, bool, error) {
	if len(args) < 2 {
		return nil, false, nil
	}
	path := args[0]
	items := args[1:]
	ops := make([]Operation, 0, len(items))
	seen := map[string]bool{}
	for i, item := range items {
		assignment, err := splitAssignmentItem(item)
		if err != nil {
			return nil, true, err
		}
		if !assignment.Present {
			if i == 0 {
				return nil, false, nil
			}
			return nil, true, usagef("cannot mix assignment items with positional set operands")
		}
		op := base
		decoded, err := decodeStructured(op, format, "set", structuredValueArgs{
			Path:     path,
			Selector: assignment.Selector,
			Value:    assignment.Value,
			Mode:     assignment.Mode,
		})
		if err != nil {
			return nil, true, err
		}
		key := decoded.Target.Part + "\x00" + decoded.Target.Selector
		if seen[key] {
			return nil, true, usagef("duplicate assignment target %s", decoded.Target.Selector)
		}
		seen[key] = true
		ops = append(ops, decoded)
	}
	return ops, true, nil
}

func splitAssignmentItem(item string) (assignmentItem, error) {
	var quote byte
	escape := false
	for i := 0; i < len(item); i++ {
		c := item[i]
		if quote != 0 {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == ':' && i+1 < len(item) && item[i+1] == '=' {
			return assignmentItem{Selector: item[:i], Value: item[i+2:], Mode: ValueModeJSON, Present: true}, nil
		}
		if c == '=' {
			return assignmentItem{Selector: item[:i], Value: item[i+1:], Mode: ValueModeString, Present: true}, nil
		}
	}
	if quote != 0 {
		return assignmentItem{}, usagef("unterminated quoted selector in assignment item")
	}
	return assignmentItem{}, nil
}

func decodeStructured(op Operation, format, verb string, args structuredValueArgs) (Operation, error) {
	op.Verb, op.Kind, op.Path, op.Value, op.ValueMode = verb, "structured", args.Path, args.Value, args.Mode
	if verb == "append" {
		op.Class = ClassNonIdempotent
	} else {
		op.Class = ClassIdempotent
	}
	part := ""
	actualSelector := args.Selector
	if format == "frontmatter" {
		part = "frontmatter"
	} else if format == "infer" && isMarkdownPath(args.Path) {
		if args.Selector == "frontmatter" {
			part = "frontmatter"
			actualSelector = "$"
		} else if strings.HasPrefix(args.Selector, "frontmatter.") {
			part = "frontmatter"
			actualSelector = strings.TrimPrefix(args.Selector, "frontmatter.")
		} else {
			return op, usagef("markdown structured selectors must use frontmatter.*")
		}
	}
	norm, err := NormalizeSelector(actualSelector)
	if err != nil {
		return op, err
	}
	target := PlanTarget{Path: args.Path, Selector: norm}
	if part != "" {
		target.Part = part
	}
	op.Target = target
	fillDescriptor(&op)
	return op, nil
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
	b, _ := jsonx.Marshal(m)
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
	if op.Kind == "structured" && op.Verb != "delete" {
		parts = append(parts, valuePreview(op.Value, op.ValueMode, 80))
	} else if op.Value != "" && op.Kind != "file" {
		parts = append(parts, valuePreview(op.Value, op.ValueMode, 80))
	} else if op.Kind == "file" && (op.Verb == "create" || op.Verb == "copy" || op.Verb == "move") {
		if op.Verb == "create" {
			parts = append(parts, valuePreview(op.Value, op.ValueMode, 80))
		} else {
			parts = append(parts, op.Value)
		}
	}
	op.Descriptor = strings.Join(parts, " ")
	if (op.Kind == "structured" && op.Verb != "delete") || op.Value != "" {
		op.ValueHash = valueHash(*op)
	}
}

func isJSONPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}

func isJSONLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jsonl" || ext == ".ndjson"
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

func defaultCreateContent(path string) string {
	switch {
	case isJSONPath(path):
		return "{}"
	case isYAMLPath(path):
		return "{}\n"
	case isJSONLPath(path), isMarkdownPath(path), isCSVPath(path):
		return ""
	default:
		return "{}"
	}
}

const shortHelp = `usage: etch [flags] <verb> [args...]
       etch run [script]

Core flags:
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

Use "etch help" for the porcelain verb table, "etch help scripts" for batch scripts,
or "etch help --all" for plumbing commands too.
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
			fmt.Fprint(w, "Topics: model, scripts, selectors, values, fields, plans, security, conflicts, section, table, csv\n")
		} else {
			fmt.Fprint(w, "Topics: model, scripts, selectors, values, fields, plans, security, conflicts, section, table, csv. Use --all for plumbing commands.\n")
		}
	case "scripts":
		fmt.Fprint(w, scriptsHelp)
	case "fields":
		fmt.Fprint(w, fieldsHelp)
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
	case "section":
		fmt.Fprint(w, sectionHelp)
	case "table", "csv":
		fmt.Fprint(w, tableHelp)
	default:
		return usagef("unknown help topic %s", topic)
	}
	return nil
}

func isPlumbingVerb(v VerbInfo) bool {
	for _, prefix := range []string{"json ", "jsonl ", "yaml ", "frontmatter ", "md ", "csv "} {
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

const valuesHelp = `Structured values are strings by default. Use --json for a strict JSON value.

Examples:
  etch set state.json status complete          # string "complete"
  etch set state.json count --json 12          # number 12
  etch append state.json events --json '{"kind":"prompt"}'
  etch append events.jsonl '{"kind":"prompt"}'
  etch set state.json status=complete count:=12

Assignment items are accepted by set only. NAME=value writes a string; NAME:=json writes JSON.
JSONL and NDJSON append values are always strict JSON and do not use --json.
`

const fieldsHelp = `Markdown fields: use frontmatter for note-global metadata; use inline fields for body-local metadata.

Frontmatter fits whole-note schema fields such as owner, source, status, and stable IDs.
Inline fields fit metadata attached to a paragraph, list item, task, or local note context.

Examples:
  etch set note.md status '"Driving"'
  etch set note.md done "2026-05-01" --task "Send follow-up"
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

const scriptsHelp = `etch run [script] executes a batch script as one transaction.

The script path is optional. Omit it or pass "-" to read the script from stdin.
Every statement is planned together against one base tree, so later statements see earlier statements.
If parsing, guards, or mutations fail, the batch produces no commit.
On success, the whole batch produces one commit unless every mutating statement is a no-op.
`

const sectionHelp = `Markdown sections are heading-delimited body ranges.

Commands:
  etch section replace note.md "## Status" "done"
  etch section append note.md Status "new block"
  etch section prepend note.md Status "new block"

Section selectors accept either a title such as Status or an ATX heading such as ## Status.
Repeated matching headings are ambiguous. Append/prepend trim payload boundary blank lines
and use one blank line between non-empty block fragments.
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
