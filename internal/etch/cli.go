package etch

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	return runCLIAt("", args, stdout, stderr)
}

func runCLIAt(cwd string, args []string, stdout, stderr io.Writer) (exitCode, error) {
	opts := GlobalOptions{CWD: cwd, Retries: 3}
	var code exitCode
	stopAfterVerb := 1

	cmd := &cli.Command{
		Name:                          "etch",
		Usage:                         "mechanical mutations to text and data files",
		UsageText:                     "etch [flags] <command> [args...]",
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
			&cli.StringFlag{Name: "subject-prefix", Usage: "prepend literal text to generated commit subject", Destination: &opts.SubjectPrefix},
			&cli.StringFlag{Name: "subject-suffix", Usage: "append literal text to generated commit subject", Destination: &opts.SubjectSuffix},
			&cli.StringFlag{Name: "body-prefix", Usage: "prepend a block to generated commit body", Destination: &opts.BodyPrefix},
			&cli.StringFlag{Name: "body-suffix", Usage: "append a block to generated commit body", Destination: &opts.BodySuffix},
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
	if opts.Message != "" && (opts.SubjectPrefix != "" || opts.SubjectSuffix != "" || opts.BodyPrefix != "" || opts.BodySuffix != "") {
		return exitUsage, usagef("--message is mutually exclusive with subject/body message modifiers")
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
		stmts, err := ParseScriptAt(opts.CWD, script)
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
		decoded, err := DecodeOperations(stmt)
		if err != nil {
			if stmt.Loc.Name != "" {
				return exitUsage, errWithCode{code: exitUsage, err: fmt.Errorf("%s%s", stmt.Loc.Prefix(), err)}
			}
			return exitUsage, err
		}
		ops = append(ops, decoded...)
	}
	if len(ops) == 0 {
		fmt.Fprintln(stderr, "etch: nothing to do")
		return exitOK, nil
	}
	exec := NewExecutor(opts)
	return exec.Run(ops, stdout, stderr)
}

func completeShell(_ context.Context, cmd *cli.Command) {
	args := cmd.Args().Slice()
	if len(args) > 0 && strings.HasPrefix(args[len(args)-1], "-") {
		values := commandLocalFlagCompletions(args[:len(args)-1])
		if len(args) == 1 {
			values = append(globalFlagCompletions(), values...)
		}
		printCompletions(cmd.Root().Writer, args[len(args)-1], values)
		return
	}
	printCompletions(cmd.Root().Writer, "", commandCompletions(args))
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
		"--subject-prefix",
		"--subject-suffix",
		"--body-prefix",
		"--body-suffix",
		"--retries",
		"--allow-empty",
		"--version",
		"--help",
	}
}

func commandCompletions(args []string) []string {
	prior, prefix := completionPriorAndPrefix(args)
	seen := map[string]bool{}
	values := []string{}
	for _, path := range commandCompletionPaths() {
		if len(path) <= len(prior) || !completionPathHasPrefix(path, prior) {
			continue
		}
		next := path[len(prior)]
		if !strings.HasPrefix(next, prefix) || seen[next] {
			continue
		}
		seen[next] = true
		values = append(values, next)
	}
	return values
}

func commandCompletionPaths() [][]string {
	paths := [][]string{
		{"help"},
		{"run"},
		{"verbs"},
		{"version"},
	}
	for _, spec := range commandSpecs() {
		paths = append(paths, spec.Path)
	}
	return paths
}

func completionPriorAndPrefix(args []string) ([]string, string) {
	if len(args) == 0 {
		return nil, ""
	}
	return args[:len(args)-1], args[len(args)-1]
}

func completionPathHasPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, part := range prefix {
		if path[i] != part {
			return false
		}
	}
	return true
}

func commandLocalFlagCompletions(args []string) []string {
	match, ok := matchCommandSpec(args)
	if !ok {
		return nil
	}
	return match.Spec.LocalFlags
}
