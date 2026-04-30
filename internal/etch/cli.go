package etch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	cli "github.com/urfave/cli/v3"
)

func Main(args []string, stdout, stderr io.Writer) int {
	code, err := runCLI(args, stdout, stderr)
	if err != nil {
		var coded errWithCode
		if errors.As(err, &coded) {
			fmt.Fprintf(stderr, "etch: %s\n", coded.err)
			return int(coded.code)
		}
		fmt.Fprintf(stderr, "etch: %s\n", err)
		return int(code)
	}
	return int(code)
}

func runCLI(args []string, stdout, stderr io.Writer) (exitCode, error) {
	opts := GlobalOptions{Retries: 3}
	var code exitCode
	stopAfterVerb := 1

	cmd := &cli.Command{
		Name:                          "etch",
		Usage:                         "mechanical mutations to text and data files",
		UsageText:                     "etch [--plan|-n|--dry-run] [flags] <verb> [args...]",
		Writer:                        stdout,
		ErrWriter:                     stderr,
		HideHelpCommand:               true,
		HideVersion:                   true,
		EnableShellCompletion:         true,
		StopOnNthArg:                  &stopAfterVerb,
		CustomRootCommandHelpTemplate: shortHelp,
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "plan", Usage: "emit canonical JSON plan", Destination: &opts.Plan},
			&cli.BoolFlag{Name: "dry-run", Aliases: []string{"n"}, Usage: "emit git-am-compatible patch preview", Destination: &opts.DryRun},
			&cli.BoolFlag{Name: "no-checkout", Usage: "commit without materializing touched paths", Destination: &opts.NoCheckout},
			&cli.BoolFlag{Name: "untracked", Usage: "admit untracked source paths under CWD", Destination: &opts.Untracked},
			&cli.StringFlag{Name: "message", Usage: "override generated commit message", Destination: &opts.Message},
			&cli.StringFlag{Name: "message-prefix", Usage: "prepend generated commit message", Destination: &opts.MessagePrefix},
			&cli.StringFlag{Name: "message-suffix", Usage: "append generated commit message", Destination: &opts.MessageSuffix},
			&cli.IntFlag{Name: "retries", Usage: "retry CAS conflicts", Value: 3, Destination: &opts.Retries},
			&cli.BoolFlag{Name: "allow-empty", Usage: "permit empty commit for mutating invocations", Destination: &opts.AllowEmpty},
			&cli.BoolFlag{Name: "version", Usage: "print version and exit"},
		},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return usagef("%s", err)
		},
		ShellComplete: completeShell,
		Action: func(_ context.Context, cmd *cli.Command) error {
			code = exitOK
			rest := cmd.Args().Slice()
			if cmd.Bool("version") {
				if len(rest) != 0 {
					code = exitUsage
					return usagef("usage: etch version")
				}
				fmt.Fprintln(stdout, versionString)
				return nil
			}
			var err error
			code, err = runParsedCLI(opts, rest, stdout, stderr)
			return err
		},
	}

	osArgs := append([]string{"etch"}, args...)
	if err := cmd.Run(context.Background(), osArgs); err != nil {
		var coded errWithCode
		if errors.As(err, &coded) {
			return coded.code, err
		}
		return exitUsage, err
	}
	return code, nil
}

func runParsedCLI(opts GlobalOptions, rest []string, stdout, stderr io.Writer) (exitCode, error) {
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

func completeShell(_ context.Context, cmd *cli.Command) {
	args := cmd.Args().Slice()
	if len(args) > 0 && strings.HasPrefix(args[len(args)-1], "-") {
		printCompletions(cmd.Root().Writer, args[len(args)-1], globalFlagCompletions())
		return
	}
	if len(args) <= 1 {
		prefix := ""
		if len(args) == 1 {
			prefix = args[0]
		}
		printCompletions(cmd.Root().Writer, prefix, commandCompletions())
	}
}

func printCompletions(w io.Writer, prefix string, values []string) {
	for _, v := range values {
		if strings.HasPrefix(v, prefix) {
			fmt.Fprintln(w, v)
		}
	}
}

func globalFlagCompletions() []string {
	return []string{
		"--plan",
		"-n",
		"--dry-run",
		"--no-checkout",
		"--untracked",
		"--message",
		"--message-prefix",
		"--message-suffix",
		"--retries",
		"--allow-empty",
		"--version",
		"--help",
	}
}

func commandCompletions() []string {
	seen := map[string]bool{
		"help":    true,
		"run":     true,
		"verbs":   true,
		"version": true,
	}
	values := []string{"help", "run", "verbs", "version"}
	for _, v := range verbCatalog() {
		first, _, _ := strings.Cut(v.Name, " ")
		if seen[first] {
			continue
		}
		seen[first] = true
		values = append(values, first)
	}
	return values
}
