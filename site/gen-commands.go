package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

type Fixture struct {
	Cat     string   `yaml:"cat"`
	Name    string   `yaml:"name"`
	Syntax  string   `yaml:"syntax"`
	Desc    string   `yaml:"desc"`
	Example string   `yaml:"example"`
	File    string   `yaml:"file"`
	Before  *string  `yaml:"before"`
	Args    []string `yaml:"args"`
	Stdin   string   `yaml:"stdin"`
	Verify  *bool    `yaml:"verify"`
}

type Command struct {
	Cat     string  `json:"cat"`
	Name    string  `json:"name"`
	Syntax  string  `json:"syntax"`
	Desc    string  `json:"desc"`
	Example string  `json:"example"`
	File    *string `json:"file"`
	Before  *string `json:"before"`
	After   *string `json:"after"`
	Commit  *string `json:"commit"`
}

func main() {
	etchBin := flag.String("etch", "./bin/etch", "path to etch binary")
	fixturesDir := flag.String("fixtures", "site/fixtures", "fixtures directory")
	output := flag.String("output", "site/data/commands.json", "output JSON file")
	flag.Parse()

	abs, err := filepath.Abs(*etchBin)
	if err != nil {
		fatal("resolving etch path: %v", err)
	}
	*etchBin = abs

	entries, err := filepath.Glob(filepath.Join(*fixturesDir, "*.yaml"))
	if err != nil {
		fatal("reading fixtures: %v", err)
	}
	sort.Strings(entries)

	if len(entries) == 0 {
		fatal("no fixtures found in %s", *fixturesDir)
	}

	var commands []Command
	for _, path := range entries {
		base := filepath.Base(path)
		fmt.Fprintf(os.Stderr, "  %s", base)

		data, err := os.ReadFile(path)
		if err != nil {
			fatal("\n  error reading %s: %v", base, err)
		}

		var f Fixture
		if err := yaml.Unmarshal(data, &f); err != nil {
			fatal("\n  error parsing %s: %v", base, err)
		}

		shouldVerify := f.Args != nil && (f.Verify == nil || *f.Verify)

		if !shouldVerify {
			fmt.Fprintf(os.Stderr, " (skip)\n")
			commands = append(commands, Command{
				Cat:     f.Cat,
				Name:    f.Name,
				Syntax:  strings.TrimRight(f.Syntax, "\n"),
				Desc:    f.Desc,
				Example: strings.TrimRight(f.Example, "\n"),
			})
			continue
		}

		after, commitMsg, err := verify(*etchBin, &f)
		if err != nil {
			fatal("\n  FAIL: %v", err)
		}

		var beforePtr *string
		if f.Before != nil {
			b := strings.TrimRight(*f.Before, "\n")
			beforePtr = &b
		}
		afterTrimmed := strings.TrimRight(after, "\n")
		fileStr := f.File

		fmt.Fprintf(os.Stderr, " ok\n")
		commands = append(commands, Command{
			Cat:     f.Cat,
			Name:    f.Name,
			Syntax:  strings.TrimRight(f.Syntax, "\n"),
			Desc:    f.Desc,
			Example: strings.TrimRight(f.Example, "\n"),
			File:    &fileStr,
			Before:  beforePtr,
			After:   &afterTrimmed,
			Commit:  &commitMsg,
		})
	}

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fatal("creating output dir: %v", err)
	}

	out, err := json.MarshalIndent(commands, "", "  ")
	if err != nil {
		fatal("marshaling JSON: %v", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(*output, out, 0o644); err != nil {
		fatal("writing output: %v", err)
	}

	fmt.Fprintf(os.Stderr, "\nwrote %s (%d commands)\n", *output, len(commands))
}

func verify(etchBin string, f *Fixture) (after string, commitMsg string, err error) {
	tmp, err := os.MkdirTemp("", "etch-fixture-*")
	if err != nil {
		return "", "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	git := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmp
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %s", strings.Join(args, " "), out)
		}
		return nil
	}

	if err := git("init", "-b", "main"); err != nil {
		return "", "", err
	}

	if f.Before != nil {
		filePath := filepath.Join(tmp, f.File)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return "", "", fmt.Errorf("mkdir: %w", err)
		}
		if err := os.WriteFile(filePath, []byte(*f.Before), 0o644); err != nil {
			return "", "", fmt.Errorf("writing before: %w", err)
		}
		if err := git("add", "."); err != nil {
			return "", "", err
		}
		if err := git("commit", "-m", "setup"); err != nil {
			return "", "", err
		}
	} else {
		// For create commands, need at least one commit
		readme := filepath.Join(tmp, "README.md")
		if err := os.WriteFile(readme, []byte("# setup\n"), 0o644); err != nil {
			return "", "", fmt.Errorf("writing readme: %w", err)
		}
		if err := git("add", "."); err != nil {
			return "", "", err
		}
		if err := git("commit", "-m", "setup"); err != nil {
			return "", "", err
		}
	}

	cmd := exec.Command(etchBin, f.Args...)
	cmd.Dir = tmp
	if f.Stdin != "" {
		cmd.Stdin = strings.NewReader(f.Stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("etch %s: %s", strings.Join(f.Args, " "), out)
	}

	// Read committed file content
	gitShow := exec.Command("git", "show", "HEAD:"+f.File)
	gitShow.Dir = tmp
	content, err := gitShow.Output()
	if err != nil {
		return "", "", fmt.Errorf("git show HEAD:%s: %w", f.File, err)
	}

	// Read commit message
	gitLog := exec.Command("git", "log", "-1", "--format=%s")
	gitLog.Dir = tmp
	msgOut, err := gitLog.Output()
	if err != nil {
		return "", "", fmt.Errorf("git log: %w", err)
	}

	return string(content), strings.TrimSpace(string(msgOut)), nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
