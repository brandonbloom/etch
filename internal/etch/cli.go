package etch

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func Main(args []string, stdout, stderr io.Writer) int {
	code, err := runCLI(args, stdout, stderr)
	if err != nil {
		var coded errWithCode
		if asErr(err, &coded) {
			fmt.Fprintf(stderr, "etch: %s\n", coded.err)
			return int(coded.code)
		}
		fmt.Fprintf(stderr, "etch: %s\n", err)
		return int(code)
	}
	return int(code)
}

func asErr(err error, target any) bool {
	type causer interface{ As(any) bool }
	if e, ok := err.(causer); ok {
		return e.As(target)
	}
	switch t := target.(type) {
	case *errWithCode:
		if e, ok := err.(errWithCode); ok {
			*t = e
			return true
		}
	}
	return false
}

func runCLI(args []string, stdout, stderr io.Writer) (exitCode, error) {
	opts, rest, err := parseGlobalFlags(args)
	if err != nil {
		return exitUsage, err
	}
	if opts.Plan && opts.DryRun {
		return exitUsage, usagef("--plan and --dry-run are mutually exclusive")
	}
	if opts.NoCheckout && (opts.Plan || opts.DryRun) {
		return exitUsage, usagef("--no-checkout has no effect with --plan or --dry-run")
	}
	if opts.Message != "" && (opts.MessagePrefix != "" || opts.MessageSuffix != "") {
		return exitUsage, usagef("--message is mutually exclusive with --message-prefix and --message-suffix")
	}
	if opts.Retries < 0 {
		return exitUsage, usagef("--retries must be non-negative")
	}

	if len(rest) == 0 {
		fmt.Fprint(stdout, shortHelp)
		return exitOK, nil
	}

	switch rest[0] {
	case "help":
		topic := ""
		all := false
		for _, arg := range rest[1:] {
			if arg == "--all" {
				all = true
				continue
			}
			if topic != "" {
				return exitUsage, usagef("usage: etch help [--all] [topic]")
			}
			topic = arg
		}
		return exitOK, printHelp(stdout, topic, all)
	case "version":
		if len(rest) != 1 {
			return exitUsage, usagef("usage: etch version")
		}
		fmt.Fprintln(stdout, versionString)
		return exitOK, nil
	case "verbs":
		if len(rest) != 2 || rest[1] != "--json" {
			return exitUsage, usagef("usage: etch verbs --json")
		}
		return exitOK, jsonOut(stdout, verbCatalog())
	}

	if rest[0] == "run" {
		if len(rest) > 2 {
			return exitUsage, usagef("usage: etch run [script]")
		}
		script := "-"
		if len(rest) == 2 {
			script = rest[1]
		}
		stmts, err := ParseScript(script)
		if err != nil {
			return exitUsage, err
		}
		return executeStatements(opts, stmts, stdout, stderr)
	}

	stmt := Statement{Tokens: rest}
	return executeStatements(opts, []Statement{stmt}, stdout, stderr)
}

func parseGlobalFlags(args []string) (GlobalOptions, []string, error) {
	opts := GlobalOptions{Retries: 3}
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			return opts, rest, nil
		}
		if arg == "--help" {
			rest = append(rest, "help")
			rest = append(rest, args[i+1:]...)
			return opts, rest, nil
		}
		if arg == "--version" {
			rest = append(rest, "version")
			continue
		}
		if arg == "-n" {
			opts.DryRun = true
			continue
		}
		if !strings.HasPrefix(arg, "--") || arg == "-" {
			rest = append(rest, args[i:]...)
			return opts, rest, nil
		}
		name, val, hasVal := strings.Cut(arg, "=")
		takeValue := func() (string, error) {
			if hasVal {
				return val, nil
			}
			i++
			if i >= len(args) {
				return "", usagef("%s requires a value", name)
			}
			return args[i], nil
		}
		switch name {
		case "--plan":
			opts.Plan = true
		case "--dry-run":
			opts.DryRun = true
		case "--no-checkout":
			opts.NoCheckout = true
		case "--untracked":
			opts.Untracked = true
		case "--allow-empty":
			opts.AllowEmpty = true
		case "--message":
			v, err := takeValue()
			if err != nil {
				return opts, nil, err
			}
			opts.Message = v
		case "--message-prefix":
			v, err := takeValue()
			if err != nil {
				return opts, nil, err
			}
			opts.MessagePrefix = v
		case "--message-suffix":
			v, err := takeValue()
			if err != nil {
				return opts, nil, err
			}
			opts.MessageSuffix = v
		case "--retries":
			v, err := takeValue()
			if err != nil {
				return opts, nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return opts, nil, usagef("--retries must be an integer")
			}
			opts.Retries = n
		default:
			return opts, nil, usagef("unknown flag %s", name)
		}
	}
	return opts, rest, nil
}

func executeStatements(opts GlobalOptions, stmts []Statement, stdout, stderr io.Writer) (exitCode, error) {
	ops := make([]Operation, 0, len(stmts))
	for _, stmt := range stmts {
		if len(stmt.Tokens) == 0 {
			continue
		}
		op, err := DecodeOperation(stmt)
		if err != nil {
			if stmt.Loc.Name != "" {
				return exitUsage, errWithCode{code: exitUsage, err: fmt.Errorf("%s%s", stmt.Loc.Prefix(), err)}
			}
			return exitUsage, err
		}
		ops = append(ops, op)
	}
	if len(ops) == 0 {
		fmt.Fprintln(stderr, "etch: nothing to do")
		return exitOK, nil
	}
	exec := NewExecutor(opts)
	return exec.Run(ops, stdout, stderr)
}

func ParseScript(path string) ([]Statement, error) {
	var data []byte
	var err error
	name := path
	if path == "-" {
		name = "<stdin>"
		data, err = readStdin()
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, failf("%s", err)
	}
	return ParseScriptBytes(name, data)
}

var readStdin = func() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}
