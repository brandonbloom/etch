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
	Stat    Stat    `json:"stat"`
	Diff    []Diff  `json:"diff,omitempty"`
}

type Stat struct {
	Status string `json:"status"`
	Adds   int    `json:"adds"`
	Dels   int    `json:"dels"`
	Label  string `json:"label"`
	AddBar string `json:"add_bar,omitempty"`
	DelBar string `json:"del_bar,omitempty"`
}

type Diff struct {
	Type       string `json:"type"`
	BeforeLine int    `json:"before_line,omitempty"`
	AfterLine  int    `json:"after_line,omitempty"`
	Marker     string `json:"marker"`
	Text       string `json:"text"`
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
	referenceOutput := flag.String("reference-output", "site/content/reference.md", "output Markdown file for CLI help reference")
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
			Results: results,
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

	if err := writeReference(*etchBin, *referenceOutput); err != nil {
		fatal("writing reference: %v", err)
	}

	fmt.Fprintf(os.Stderr, "\nwrote %s (%d commands)\n", *output, len(commands))
	fmt.Fprintf(os.Stderr, "wrote %s\n", *referenceOutput)
}

func writeReference(etchBin, output string) error {
	topics := []struct {
		Title string
		Args  []string
	}{
		{Title: "Common Commands", Args: []string{"help"}},
		{Title: "All Commands", Args: []string{"help", "--all"}},
		{Title: "Model", Args: []string{"help", "model"}},
		{Title: "Scripts", Args: []string{"help", "scripts"}},
		{Title: "Selectors", Args: []string{"help", "selectors"}},
		{Title: "Values", Args: []string{"help", "values"}},
		{Title: "Fields", Args: []string{"help", "fields"}},
		{Title: "Plans", Args: []string{"help", "plans"}},
		{Title: "Security", Args: []string{"help", "security"}},
		{Title: "Conflicts", Args: []string{"help", "conflicts"}},
		{Title: "Addressing", Args: []string{"help", "addressing"}},
		{Title: "Section", Args: []string{"help", "section"}},
		{Title: "Tasks", Args: []string{"help", "tasks"}},
		{Title: "Table And CSV", Args: []string{"help", "table"}},
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: Reference\n")
	b.WriteString("description: CLI help topics generated from etch help.\n")
	b.WriteString("---\n\n")
	b.WriteString("This page is generated from `etch help` output by `mise run site:verify`.\n")

	for _, topic := range topics {
		out, err := commandOutput("", etchBin, topic.Args...)
		if err != nil {
			return err
		}
		b.WriteString("\n## ")
		b.WriteString(topic.Title)
		b.WriteString("\n\n")
		b.WriteString("```text\n")
		b.WriteString(strings.TrimRight(out, "\n"))
		b.WriteString("\n```\n")
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("creating reference dir: %w", err)
	}
	return os.WriteFile(output, []byte(b.String()), 0o644)
}

func commandOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), out)
	}
	return string(out), nil
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
		file := File{Path: path, Before: before, After: after}
		file.Diff = diffLines(before, after)
		file.Stat = diffStat(before, after, file.Diff)
		results = append(results, file)
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

func diffLines(before, after *string) []Diff {
	beforeLines := splitLines(before)
	afterLines := splitLines(after)
	switch {
	case before == nil:
		return allLines("add", "+", afterLines, false)
	case after == nil:
		return allLines("del", "-", beforeLines, true)
	}

	m, n := len(beforeLines), len(afterLines)
	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if beforeLines[i] == afterLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var rows []Diff
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case beforeLines[i] == afterLines[j]:
			rows = append(rows, Diff{Type: "eq", BeforeLine: i + 1, AfterLine: j + 1, Marker: " ", Text: beforeLines[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			rows = append(rows, Diff{Type: "del", BeforeLine: i + 1, Marker: "-", Text: beforeLines[i]})
			i++
		default:
			rows = append(rows, Diff{Type: "add", AfterLine: j + 1, Marker: "+", Text: afterLines[j]})
			j++
		}
	}
	for ; i < m; i++ {
		rows = append(rows, Diff{Type: "del", BeforeLine: i + 1, Marker: "-", Text: beforeLines[i]})
	}
	for ; j < n; j++ {
		rows = append(rows, Diff{Type: "add", AfterLine: j + 1, Marker: "+", Text: afterLines[j]})
	}
	return rows
}

func allLines(typ, marker string, lines []string, before bool) []Diff {
	rows := make([]Diff, 0, len(lines))
	for i, line := range lines {
		row := Diff{Type: typ, Marker: marker, Text: line}
		if before {
			row.BeforeLine = i + 1
		} else {
			row.AfterLine = i + 1
		}
		rows = append(rows, row)
	}
	return rows
}

func splitLines(s *string) []string {
	if s == nil || *s == "" {
		return nil
	}
	return strings.Split(*s, "\n")
}

func diffStat(before, after *string, rows []Diff) Stat {
	stat := Stat{}
	switch {
	case before == nil:
		stat.Status = "created"
	case after == nil:
		stat.Status = "deleted"
	default:
		stat.Status = "modified"
	}
	for _, row := range rows {
		switch row.Type {
		case "add":
			stat.Adds++
		case "del":
			stat.Dels++
		}
	}
	total := stat.Adds + stat.Dels
	if total == 0 {
		if stat.Status == "created" {
			stat.Label = "created empty file"
		} else if stat.Status == "deleted" {
			stat.Label = "deleted empty file"
		} else {
			stat.Label = "unchanged"
		}
		return stat
	}
	pieces := []string{}
	if stat.Adds > 0 {
		pieces = append(pieces, fmt.Sprintf("+%d", stat.Adds))
		stat.AddBar = statBar("+", stat.Adds)
	}
	if stat.Dels > 0 {
		pieces = append(pieces, fmt.Sprintf("-%d", stat.Dels))
		stat.DelBar = statBar("-", stat.Dels)
	}
	lineWord := "lines"
	if total == 1 {
		lineWord = "line"
	}
	stat.Label = fmt.Sprintf("%d %s (%s)", total, lineWord, strings.Join(pieces, " "))
	return stat
}

func statBar(ch string, count int) string {
	if count > 24 {
		return strings.Repeat(ch, 24) + "..."
	}
	return strings.Repeat(ch, count)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
