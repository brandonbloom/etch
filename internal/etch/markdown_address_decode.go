package etch

type markdownAddressFlagOptions struct {
	Body     bool
	Section  bool
	Item     bool
	ItemType bool
	Task     bool
	After    bool
	Before   bool
	Head     bool
	Tail     bool
	Hidden   bool
}

var markdownFieldAddressFlagOptions = markdownAddressFlagOptions{
	Body: true, Section: true, Item: true, ItemType: true, Task: true,
	After: true, Before: true, Head: true, Tail: true,
}

var markdownTaskListAddressFlagOptions = markdownAddressFlagOptions{
	Section: true, After: true, Before: true,
}

func isMarkdownAddressFlag(arg string) bool {
	switch arg {
	case "--body", "--section", "--item", "--item-type", "--task", "--after", "--before", "--head", "--tail", "--hidden":
		return true
	default:
		return false
	}
}

func parseMarkdownAddressFlag(args []string, index int, address *markdownAddress, options markdownAddressFlagOptions) (bool, int, error) {
	arg := args[index]
	switch arg {
	case "--body":
		if !options.Body {
			return false, index, nil
		}
		address.Body = true
		return true, index, nil
	case "--section", "--item", "--item-type", "--task", "--after", "--before":
		if !markdownAddressFlagWithValueAllowed(arg, options) {
			return false, index, nil
		}
		if index+1 >= len(args) {
			return true, index, usagef("%s requires a value", arg)
		}
		value := args[index+1]
		switch arg {
		case "--section":
			address.Section = value
		case "--item":
			address.Item = value
		case "--item-type":
			address.ItemTypes = append(address.ItemTypes, value)
		case "--task":
			address.Task = value
		case "--after":
			address.After = value
		case "--before":
			address.Before = value
		}
		return true, index + 1, nil
	case "--head":
		if !options.Head {
			return false, index, nil
		}
		address.Head = true
		return true, index, nil
	case "--tail":
		if !options.Tail {
			return false, index, nil
		}
		address.Tail = true
		return true, index, nil
	case "--hidden":
		if !options.Hidden {
			return false, index, nil
		}
		address.Hidden = true
		return true, index, nil
	default:
		return false, index, nil
	}
}

func markdownAddressFlagWithValueAllowed(arg string, options markdownAddressFlagOptions) bool {
	switch arg {
	case "--section":
		return options.Section
	case "--item":
		return options.Item
	case "--item-type":
		return options.ItemType
	case "--task":
		return options.Task
	case "--after":
		return options.After
	case "--before":
		return options.Before
	default:
		return false
	}
}
