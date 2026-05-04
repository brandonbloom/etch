package etch

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateHelpSnapshots = flag.Bool("update-help-snapshots", false, "update generated help output snapshots")

type helpSnapshotCase struct {
	Path string
	Args []string
}

func TestHelpOutputSnapshots(t *testing.T) {
	expected := make(map[string]bool)

	for _, tc := range helpSnapshotCases() {
		rel := "pages/" + tc.Path
		expected[rel] = true
		t.Run(strings.TrimSuffix(rel, ".txt"), func(t *testing.T) {
			assertHelpSnapshot(t, rel, helpOutput(t, tc.Args...))
		})
	}

	expected["reference.json"] = true
	t.Run("reference.json", func(t *testing.T) {
		assertHelpSnapshot(t, "reference.json", helpOutput(t, "help", "--json"))
	})

	assertNoExtraHelpSnapshots(t, expected)
}

func TestHelpTopicAliasesMatchCanonicalOutput(t *testing.T) {
	for _, topic := range namedHelpTopics() {
		if len(topic.Aliases) == 0 {
			t.Fatalf("help topic %q has no aliases", topic.ID)
		}
		canonical := helpOutput(t, "help", topic.Aliases[0])
		for _, alias := range topic.Aliases[1:] {
			t.Run(topic.ID+"/"+alias, func(t *testing.T) {
				got := helpOutput(t, "help", alias)
				if !bytes.Equal(got, canonical) {
					t.Fatalf("help alias %q for topic %q differs from canonical alias %q", alias, topic.ID, topic.Aliases[0])
				}
			})
		}
	}
}

func TestHelpFlagMatchesShortHelpSnapshot(t *testing.T) {
	got := helpOutput(t, "--help")
	want := helpOutput(t)
	if !bytes.Equal(got, want) {
		t.Fatalf("--help output differs from default short help:\n%s", firstSnapshotDiff(want, got))
	}
}

func helpSnapshotCases() []helpSnapshotCase {
	cases := []helpSnapshotCase{
		{Path: "short.txt"},
		{Path: "help.txt", Args: []string{"help"}},
		{Path: "help-all.txt", Args: []string{"help", "--all"}},
	}
	for _, topic := range namedHelpTopics() {
		cases = append(cases, helpSnapshotCase{
			Path: "topics/" + topic.ID + ".txt",
			Args: []string{"help", topic.Aliases[0]},
		})
	}
	return cases
}

func helpOutput(t *testing.T, args ...string) []byte {
	t.Helper()
	var out, errb bytes.Buffer
	code, err := runCLI(args, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("runCLI(%s) code=%d err=%v stderr=%s", strings.Join(args, " "), code, err, errb.String())
	}
	if errb.Len() != 0 {
		t.Fatalf("runCLI(%s) wrote stderr: %s", strings.Join(args, " "), errb.String())
	}
	return out.Bytes()
}

func assertHelpSnapshot(t *testing.T, rel string, got []byte) {
	t.Helper()
	path := helpSnapshotPath(rel)
	if *updateHelpSnapshots {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating snapshot dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing snapshot %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading snapshot %s: %v\nRun `go test ./internal/etch -update-help-snapshots` to regenerate help snapshots.", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("snapshot %s mismatch\n%s\nRun `go test ./internal/etch -update-help-snapshots` to regenerate help snapshots.", path, firstSnapshotDiff(want, got))
	}
}

func assertNoExtraHelpSnapshots(t *testing.T, expected map[string]bool) {
	t.Helper()
	if *updateHelpSnapshots {
		return
	}
	root := helpSnapshotPath("")
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !expected[rel] {
			t.Errorf("unexpected help snapshot %s", filepath.Join(root, filepath.FromSlash(rel)))
		}
		return nil
	}); err != nil {
		t.Fatalf("walking help snapshots: %v", err)
	}
}

func helpSnapshotPath(rel string) string {
	return filepath.Join("testdata", "help", filepath.FromSlash(rel))
}

func firstSnapshotDiff(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	max := len(wantLines)
	if len(gotLines) > max {
		max = len(gotLines)
	}
	for i := 0; i < max; i++ {
		if lineAt(wantLines, i) != lineAt(gotLines, i) {
			return fmt.Sprintf("first difference at line %d\nwant: %q\ngot:  %q", i+1, lineAt(wantLines, i), lineAt(gotLines, i))
		}
	}
	return "outputs differ"
}

func lineAt(lines []string, i int) string {
	if i >= len(lines) {
		return "<missing>"
	}
	return lines[i]
}
