package etch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runOK(t *testing.T, args ...string) {
	t.Helper()
	var out, errb bytes.Buffer
	code, err := runCLI(args, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("etch %v code=%d err=%v stdout=%s stderr=%s", args, code, err, out.String(), errb.String())
	}
}

func TestJSONVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"tags":["a"],"remove":["x","y","x"]}`+"\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "append", "state.json", "tags", `"b"`)
	runOK(t, "add", "state.json", "tags", `"b"`)
	runOK(t, "remove", "state.json", "remove", `"x"`)
	runOK(t, "delete", "state.json", "missing")
	got := testGit(t, dir, "show", "HEAD:state.json")
	if strings.Count(got, `"b"`) != 1 || strings.Contains(got, `"x"`) {
		t.Fatalf("JSON verbs result:\n%s", got)
	}
}

func TestYAMLAndFrontmatterVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "tags:\n  - a\n")
	writeFile(t, dir, "note.md", "# Note\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "yaml", "append", "config.yaml", "tags", `"b"`)
	runOK(t, "set", "note.md", "frontmatter.status", "draft")
	runOK(t, "add", "note.md", "frontmatter.tags", `"x"`)
	yamlOut := testGit(t, dir, "show", "HEAD:config.yaml")
	if !strings.Contains(yamlOut, "b") {
		t.Fatalf("YAML output:\n%s", yamlOut)
	}
	mdOut := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.HasPrefix(mdOut, "---\n") || !strings.Contains(mdOut, "status: draft") || !strings.Contains(mdOut, "- x") {
		t.Fatalf("frontmatter output:\n%s", mdOut)
	}
}

func TestYAMLAndFrontmatterVerbsCreateMissingFiles(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "README.md", "# hi\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "append", "config.yaml", "tags", `"b"`)
	runOK(t, "set", "note.md", "frontmatter.status", "draft")
	runOK(t, "add", "other.md", "frontmatter.tags", `"x"`)

	yamlOut := testGit(t, dir, "show", "HEAD:config.yaml")
	if !strings.Contains(yamlOut, "tags:") || !strings.Contains(yamlOut, "- b") {
		t.Fatalf("YAML output:\n%s", yamlOut)
	}
	mdOut := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.HasPrefix(mdOut, "---\n") || !strings.Contains(mdOut, "status: draft") {
		t.Fatalf("frontmatter output:\n%s", mdOut)
	}
	otherOut := testGit(t, dir, "show", "HEAD:other.md")
	if !strings.HasPrefix(otherOut, "---\n") || !strings.Contains(otherOut, "- x") {
		t.Fatalf("frontmatter add output:\n%s", otherOut)
	}
}

func TestYAMLSetHexdumpLikeStringDoesNotParseJSONPrefix(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "doc.yml", "body: value\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	hexdump := "00000000  aa bb cc dd  |....|\n00000010  00 11 22 33  |..\"3|\n"
	runOK(t, "set", "doc.yml", "body", hexdump)

	yamlOut := testGit(t, dir, "show", "HEAD:doc.yml")
	if !strings.Contains(yamlOut, "00000000") || !strings.Contains(yamlOut, "00000010") {
		t.Fatalf("YAML output lost hexdump string:\n%s", yamlOut)
	}
	if strings.Contains(yamlOut, "body: 0.0") {
		t.Fatalf("YAML output parsed hexdump as number:\n%s", yamlOut)
	}
	subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s"))
	if strings.HasSuffix(subject, " 0") {
		t.Fatalf("commit subject used JSON-prefix preview: %q", subject)
	}
}

func TestMarkdownReplaceSection(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "# Title\n\n## Notes\nold\n\n## Next\nkeep\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "replace-section", "note.md", "## Notes", "new\nbody\n")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "## Notes\nnew\nbody\n## Next") || strings.Contains(got, "old") {
		t.Fatalf("replace-section output:\n%s", got)
	}
}

func TestCSVTableVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "data.csv", "id,status\n1,open\n2,open\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "table", "set", "data.csv", "all,status", "done")
	runOK(t, "table", "row", "append", "data.csv", `{"id":"3","status":"open"}`)
	runOK(t, "table", "column", "add", "data.csv", "owner", "--default", "Brandon")
	runOK(t, "table", "column", "rename", "data.csv", "owner", "assignee")
	runOK(t, "table", "row", "delete", "data.csv", "id=2")
	got := testGit(t, dir, "show", "HEAD:data.csv")
	if !strings.Contains(got, "id,status,assignee") || strings.Contains(got, "2,") || !strings.Contains(got, "Brandon") {
		t.Fatalf("CSV output:\n%s", got)
	}
}

func TestMarkdownTableVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n| sku | status |\n| --- | --- |\n| A1 | open |\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "table", "set", "note.md", "## Inventory", "sku=A1,status", "done")
	runOK(t, "table", "row", "append", "note.md", "## Inventory", `{"sku":"B2","status":"open"}`)
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "| A1 | done |") || !strings.Contains(got, "| B2 | open |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestBOMPreservedForJSON(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "state.json"), append([]byte{0xef, 0xbb, 0xbf}, []byte(`{"a":1}`+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, dir, "initial")
	chdir(t, dir)
	runOK(t, "set", "state.json", "b", "2")
	got := []byte(testGit(t, dir, "show", "HEAD:state.json"))
	if len(got) < 3 || got[0] != 0xef || got[1] != 0xbb || got[2] != 0xbf {
		t.Fatalf("BOM not preserved: %v", got[:min(3, len(got))])
	}
}
