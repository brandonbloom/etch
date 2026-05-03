package etch

import "strings"

func decodeMarkdownSet(base Operation, args []string) ([]Operation, bool, error) {
	if len(args) < 2 {
		return nil, false, nil
	}
	if assignment, err := splitAssignmentItem(args[1]); err != nil {
		return nil, true, err
	} else if assignment.Present {
		if hasMarkdownAddressArgs(args[2:]) {
			return nil, true, usagef("assignment items cannot be combined with Markdown address flags")
		}
		return nil, false, nil
	}
	if !hasMarkdownAddressArgs(args[3:]) {
		return nil, false, nil
	}
	op, err := decodeMarkdownFieldSet(base, args)
	if err != nil {
		return nil, true, err
	}
	return []Operation{op}, true, nil
}

func decodeMarkdownFieldSet(op Operation, args []string) (Operation, error) {
	if len(args) < 3 {
		return op, usagef("usage: etch set <path.md> <field> <value> [address flags]")
	}
	if args[2] == "--json" {
		return op, usagef("--json is not supported for Markdown inline fields")
	}
	address, err := parseMarkdownAddressArgs(args[3:], true)
	if err != nil {
		return op, err
	}
	return decodeMarkdownField(op, "set", args[0], args[1], args[2], address)
}

func decodeMarkdownFieldDelete(op Operation, args []string) (Operation, error) {
	address, err := parseMarkdownAddressArgs(args[2:], false)
	if err != nil {
		return op, err
	}
	return decodeMarkdownField(op, "delete", args[0], args[1], "", address)
}

func decodeMarkdownField(op Operation, verb, path, field, value string, address markdownAddress) (Operation, error) {
	if strings.TrimSpace(field) == "" {
		return op, usagef("Markdown field name must not be blank")
	}
	if isReservedDataviewImplicitField(field) {
		return op, usagef("Dataview implicit field %q is not writable", field)
	}
	if address.hasItemLocation() && isReservedDataviewItemImplicitField(field) {
		return op, usagef("Dataview task/list implicit field %q is not writable", field)
	}
	op.Verb, op.Kind, op.Class, op.Path, op.Value, op.ValueMode = verb, "md-field", ClassIdempotent, path, value, ValueModeString
	op.Markdown = address
	op.Target = PlanTarget{Path: path, Part: "inline-field", Selector: field}
	if address.Section != "" {
		op.Target.Section = address.Section
	}
	switch {
	case address.Task != "":
		op.Target.Scope = "task"
	case address.Item != "":
		op.Target.Scope = "item"
	case address.Body:
		op.Target.Scope = "body"
	}
	fillDescriptor(&op)
	return op, nil
}

func hasMarkdownAddressArgs(args []string) bool {
	for _, arg := range args {
		if isMarkdownAddressFlag(arg) {
			return true
		}
	}
	return false
}

func parseMarkdownAddressArgs(args []string, allowHidden bool) (markdownAddress, error) {
	var address markdownAddress
	options := markdownFieldAddressFlagOptions
	options.Hidden = allowHidden
	for i := 0; i < len(args); i++ {
		arg := args[i]
		parsed, next, err := parseMarkdownAddressFlag(args, i, &address, options)
		if err != nil {
			return markdownAddress{}, err
		}
		if parsed {
			i = next
			continue
		}
		if arg == "--hidden" && !allowHidden {
			return markdownAddress{}, usagef("--hidden is only accepted by set")
		}
		if strings.HasPrefix(arg, "--") {
			return markdownAddress{}, usagef("unknown Markdown address flag %s", arg)
		}
		return markdownAddress{}, usagef("unexpected Markdown field argument %q", arg)
	}
	if address.Item != "" && address.Task != "" {
		return markdownAddress{}, usagef("--item and --task are mutually exclusive")
	}
	if address.Body && (address.Section != "" || address.Item != "" || address.Task != "") {
		return markdownAddress{}, usagef("--body cannot be combined with --section, --item, or --task")
	}
	if len(address.ItemTypes) > 0 && !address.hasItemLocation() {
		return markdownAddress{}, usagef("--item-type requires --item or --task")
	}
	if !address.hasBodyLocation() {
		return markdownAddress{}, usagef("Markdown inline field mutation requires an address flag")
	}
	if _, err := markdownItemTypeConstraintsFromArgs(address.ItemTypes); err != nil {
		return markdownAddress{}, err
	}
	if _, err := markdownPlacementFromFlags(address.Head, address.Tail, address.Before, address.After); err != nil {
		return markdownAddress{}, err
	}
	return address, nil
}

func isReservedDataviewImplicitField(field string) bool {
	field = strings.TrimSpace(field)
	return field == "file" || strings.HasPrefix(field, "file.") || strings.HasPrefix(field, "$.file.")
}

func isReservedDataviewItemImplicitField(field string) bool {
	switch dataviewFieldName(field) {
	case "status", "checked", "completed", "fullycompleted", "text", "line", "section", "children", "task", "parent", "blockid":
		return true
	default:
		return false
	}
}
