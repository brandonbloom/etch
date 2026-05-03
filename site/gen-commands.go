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
	Setup   []File   `yaml:"setup"`
	Results []string `yaml:"results"`
	Args    []string `yaml:"args"`
	Stdin   string   `yaml:"stdin"`
	Verify  *bool    `yaml:"verify"`
}

type File struct {
	Path    string  `yaml:"path" json:"file"`
	Content string  `yaml:"content,omitempty" json:"-"`
	Before  *string `json:"before"`
	After   *string `json:"after"`
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
	Results []File  `json:"results,omitempty"`
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

		results, commitMsg, err := verify(*etchBin, &f)
		if err != nil {
			fatal("\n  FAIL: %v", err)
		}

		var filePtr, beforePtr, afterPtr *string
		if len(results) > 0 {
			fileStr := results[0].Path
			filePtr = &fileStr
			beforePtr = results[0].Before
			afterPtr = results[0].After
		}
		var allResults []File
		if len(results) > 1 {
			allResults = results
		}

		fmt.Fprintf(os.Stderr, " ok\n")
		commands = append(commands, Command{
			Cat:     f.Cat,
			Name:    f.Name,
			Syntax:  strings.TrimRight(f.Syntax, "\n"),
			Desc:    f.Desc,
			Example: strings.TrimRight(f.Example, "\n"),
			File:    filePtr,
			Before:  beforePtr,
			After:   afterPtr,
			Commit:  &commitMsg,
			Results: allResults,
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

func verify(etchBin string, f *Fixture) (results []File, commitMsg string, err error) {
	tmp, err := os.MkdirTemp("", "etch-fixture-*")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp dir: %w", err)
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
		return nil, "", err
	}

	setup := append([]File(nil), f.Setup...)
	if f.Before != nil {
		setup = append([]File{{Path: f.File, Content: *f.Before}}, setup...)
	}

	if len(setup) > 0 {
		for _, file := range setup {
			if file.Path == "" {
				return nil, "", fmt.Errorf("setup file missing path")
			}
			filePath := filepath.Join(tmp, file.Path)
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				return nil, "", fmt.Errorf("mkdir: %w", err)
			}
			if err := os.WriteFile(filePath, []byte(file.Content), 0o644); err != nil {
				return nil, "", fmt.Errorf("writing setup file %s: %w", file.Path, err)
			}
		}
		if err := git("add", "-f", "."); err != nil {
			return nil, "", err
		}
		if err := git("commit", "-m", "setup"); err != nil {
			return nil, "", err
		}
	} else {
		// Etch needs an initial commit even when the fixture starts with no files.
		readme := filepath.Join(tmp, "README.md")
		if err := os.WriteFile(readme, []byte("# setup\n"), 0o644); err != nil {
			return nil, "", fmt.Errorf("writing readme: %w", err)
		}
		if err := git("add", "-f", "."); err != nil {
			return nil, "", err
		}
		if err := git("commit", "-m", "setup"); err != nil {
			return nil, "", err
		}
	}

	setupRev, err := gitOutput(tmp, "rev-parse", "HEAD")
	if err != nil {
		return nil, "", err
	}
	setupRev = strings.TrimSpace(setupRev)

	cmd := exec.Command(etchBin, f.Args...)
	cmd.Dir = tmp
	if f.Stdin != "" {
		cmd.Stdin = strings.NewReader(f.Stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("etch %s: %s", strings.Join(f.Args, " "), out)
	}

	paths := append([]string(nil), f.Results...)
	if len(paths) == 0 && f.File != "" {
		paths = append(paths, f.File)
	}
	for _, path := range paths {
		before, err := gitFile(tmp, setupRev, path)
		if err != nil {
			return nil, "", err
		}
		after, err := gitFile(tmp, "HEAD", path)
		if err != nil {
			return nil, "", err
		}
		if before == nil && after == nil {
			return nil, "", fmt.Errorf("result file %s is missing before and after", path)
		}
		results = append(results, File{Path: path, Before: before, After: after})
	}

	msgOut, err := gitOutput(tmp, "log", "-1", "--format=%s")
	if err != nil {
		return nil, "", fmt.Errorf("git log: %w", err)
	}

	return results, strings.TrimSpace(msgOut), nil
}

func gitFile(dir, rev, path string) (*string, error) {
	out, err := gitOutput(dir, "show", rev+":"+path)
	if err != nil {
		return nil, nil
	}
	trimmed := strings.TrimRight(out, "\n")
	return &trimmed, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
