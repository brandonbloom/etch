package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

type Fixture struct {
	Cat             string   `yaml:"cat"`
	Name            string   `yaml:"name"`
	Syntax          string   `yaml:"syntax"`
	Desc            string   `yaml:"desc"`
	Example         string   `yaml:"example"`
	File            string   `yaml:"file"`
	Before          *string  `yaml:"before"`
	Setup           []File   `yaml:"setup"`
	Results         []string `yaml:"results"`
	Args            []string `yaml:"args"`
	Stdin           string   `yaml:"stdin"`
	Verify          *bool    `yaml:"verify"`
	ReferenceTopics []string `yaml:"reference_topics"`
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
	HTML       string `json:"html,omitempty"`
}

type Command struct {
	Cat             string   `json:"cat"`
	Name            string   `json:"name"`
	Syntax          string   `json:"syntax"`
	Desc            string   `json:"desc"`
	Example         string   `json:"example"`
	ReferenceTopics []string `json:"reference_topics,omitempty"`
	File            *string  `json:"file"`
	Before          *string  `json:"before"`
	After           *string  `json:"after"`
	Commit          *string  `json:"commit"`
	Results         []File   `json:"results,omitempty"`
}

type PromptData struct {
	Bootstrap string `json:"bootstrap"`
	Context   string `json:"context"`
}

func main() {
	etchBin := flag.String("etch", "./bin/etch", "path to etch binary")
	fixturesDir := flag.String("fixtures", "site/fixtures", "fixtures directory")
	output := flag.String("output", "site/data/commands.json", "output JSON file")
	referenceOutput := flag.String("reference-output", "site/content/reference.md", "output Markdown file for CLI help reference")
	referenceDataInput := flag.String("reference-data-input", "internal/etch/testdata/help/reference.json", "input JSON file for CLI help reference")
	referenceDataOutput := flag.String("reference-data-output", "site/data/reference.json", "output JSON file for CLI help reference")
	promptDataOutput := flag.String("prompt-data-output", "site/data/prompts.json", "output JSON file for CLI prompt snippets")
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

		referenceTopics := f.ReferenceTopics
		if referenceTopics == nil {
			referenceTopics = referenceTopicsForFixture(f)
		}

		shouldVerify := f.Args != nil && (f.Verify == nil || *f.Verify)

		if !shouldVerify {
			fmt.Fprintf(os.Stderr, " (skip)\n")
			commands = append(commands, Command{
				Cat:             f.Cat,
				Name:            f.Name,
				Syntax:          strings.TrimRight(f.Syntax, "\n"),
				Desc:            f.Desc,
				Example:         strings.TrimRight(f.Example, "\n"),
				ReferenceTopics: referenceTopics,
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
			Cat:             f.Cat,
			Name:            f.Name,
			Syntax:          strings.TrimRight(f.Syntax, "\n"),
			Desc:            f.Desc,
			Example:         strings.TrimRight(f.Example, "\n"),
			ReferenceTopics: referenceTopics,
			File:            filePtr,
			Before:          beforePtr,
			After:           afterPtr,
			Commit:          &commitMsg,
			Results:         results,
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

	referenceTopics, err := writeReferenceData(*referenceDataInput, *referenceDataOutput)
	if err != nil {
		fatal("writing reference data: %v", err)
	}
	if err := writeReferenceContent(*referenceOutput); err != nil {
		fatal("writing reference page: %v", err)
	}
	if err := writePromptData(*etchBin, *promptDataOutput); err != nil {
		fatal("writing prompt data: %v", err)
	}

	fmt.Fprintf(os.Stderr, "\nwrote %s (%d commands)\n", *output, len(commands))
	fmt.Fprintf(os.Stderr, "wrote %s (%d topics)\n", *referenceDataOutput, referenceTopics)
	fmt.Fprintf(os.Stderr, "wrote %s\n", *referenceOutput)
	fmt.Fprintf(os.Stderr, "wrote %s\n", *promptDataOutput)
}

func referenceTopicsForFixture(f Fixture) []string {
	switch f.Cat {
	case "Workflow":
		switch f.Name {
		case "--plan", "--dry-run":
			return []string{"plans"}
		case "--no-checkout":
			return []string{"conflicts"}
		case "--untracked":
			return []string{"invocation"}
		case "--message", "--subject-prefix/suffix", "--body-prefix/suffix":
			return []string{"commits"}
		default:
			return nil
		}
	case "Mutate":
		return []string{"values"}
	case "Markdown":
		if f.Name == "section replace" {
			return []string{"sections", "markdown-addressing"}
		}
		return []string{"sections"}
	case "Tasks":
		if f.Name == "task close" {
			return []string{"tasks", "markdown-addressing"}
		}
		return []string{"tasks"}
	case "Fields":
		return []string{"fields", "markdown-addressing"}
	case "Tables":
		return []string{"tables-and-csv"}
	case "Whole files":
		return []string{"files"}
	case "Guards":
		return []string{"guards"}
	case "Scripts":
		return []string{"scripts"}
	case "JSONL":
		return []string{"formats"}
	default:
		return nil
	}
}

func writeReferenceData(input, output string) (int, error) {
	data, err := os.ReadFile(input)
	if err != nil {
		return 0, fmt.Errorf("reading committed help snapshot %s: %w", input, err)
	}
	if !json.Valid(data) {
		return 0, fmt.Errorf("committed help snapshot %s is invalid JSON", input)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return 0, fmt.Errorf("creating reference data dir: %w", err)
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return 0, err
	}

	var reference struct {
		Topics []struct{} `json:"topics"`
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		return 0, fmt.Errorf("counting reference topics: %w", err)
	}
	return len(reference.Topics), nil
}

func writeReferenceContent(output string) error {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: Reference\n")
	b.WriteString("description: Command behavior, addressing rules, workflow flags, and examples.\n")
	b.WriteString("layout: reference\n")
	b.WriteString("---\n\n")

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("creating reference dir: %w", err)
	}
	return os.WriteFile(output, []byte(b.String()), 0o644)
}

func writePromptData(etchBin, output string) error {
	bootstrap, err := commandOutput("", etchBin, "prompt")
	if err != nil {
		return err
	}
	context, err := commandOutput("", etchBin, "prompt", "--context")
	if err != nil {
		return err
	}
	if strings.TrimSpace(bootstrap) == "" || strings.TrimSpace(context) == "" {
		return fmt.Errorf("etch prompt returned empty output")
	}

	out, err := json.MarshalIndent(PromptData{Bootstrap: bootstrap, Context: context}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling prompt JSON: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("creating prompt data dir: %w", err)
	}
	return os.WriteFile(output, out, 0o644)
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
	return addIntralineHighlights(rows)
}

func addIntralineHighlights(rows []Diff) []Diff {
	for i := 0; i+1 < len(rows); i++ {
		if rows[i].Type != "del" || rows[i+1].Type != "add" {
			continue
		}
		delHTML, addHTML, ok := intralineHTML(rows[i].Text, rows[i+1].Text)
		if !ok {
			continue
		}
		rows[i].HTML = delHTML
		rows[i+1].HTML = addHTML
		i++
	}
	return rows
}

func intralineHTML(before, after string) (string, string, bool) {
	beforeRunes := []rune(before)
	afterRunes := []rune(after)

	prefix := 0
	for prefix < len(beforeRunes) && prefix < len(afterRunes) && beforeRunes[prefix] == afterRunes[prefix] {
		prefix++
	}

	beforeSuffix := len(beforeRunes)
	afterSuffix := len(afterRunes)
	for beforeSuffix > prefix && afterSuffix > prefix && beforeRunes[beforeSuffix-1] == afterRunes[afterSuffix-1] {
		beforeSuffix--
		afterSuffix--
	}

	if prefix == beforeSuffix && prefix == afterSuffix {
		return "", "", false
	}

	return highlightRunes(beforeRunes, prefix, beforeSuffix, "diff-hl-del"),
		highlightRunes(afterRunes, prefix, afterSuffix, "diff-hl-add"),
		true
}

func highlightRunes(runes []rune, start, end int, class string) string {
	var b strings.Builder
	b.WriteString(html.EscapeString(string(runes[:start])))
	if start < end {
		b.WriteString(`<span class="`)
		b.WriteString(class)
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(string(runes[start:end])))
		b.WriteString(`</span>`)
	}
	b.WriteString(html.EscapeString(string(runes[end:])))
	return b.String()
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
