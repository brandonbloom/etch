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

func TestJSONSetPreservesSurroundingSource(t *testing.T) {
	before := []byte("{\n  \"z\": 0,\n  \"status\" : \"open\",\n  \"nested\": {\"keep\":true}\n}\n")

	got, changed, err := evalJSON("status", "set", "complete", before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("evalJSON reported no change")
	}
	want := "{\n  \"z\": 0,\n  \"status\" : \"complete\",\n  \"nested\": {\"keep\":true}\n}\n"
	if string(got) != want {
		t.Fatalf("JSON output:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSONSetTargetsFirstDuplicateMember(t *testing.T) {
	before := []byte(`{"status":"first","status":"second"}` + "\n")

	got, changed, err := evalJSON("status", "set", "complete", before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("evalJSON reported no change")
	}
	want := `{"status":"complete","status":"second"}` + "\n"
	if string(got) != want {
		t.Fatalf("JSON output = %s, want %s", got, want)
	}
}

func TestJSONSetMissingMemberUsesExistingSeparatorStyle(t *testing.T) {
	before := []byte("{\n  \"z\" : 0\n}\n")

	got, changed, err := evalJSON("status", "set", "complete", before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("evalJSON reported no change")
	}
	want := "{\n  \"z\" : 0,\n  \"status\" : \"complete\"\n}\n"
	if string(got) != want {
		t.Fatalf("JSON output:\n%s\nwant:\n%s", got, want)
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
	if !strings.Contains(yamlOut, "body: |") {
		t.Fatalf("YAML output did not use a literal block scalar:\n%s", yamlOut)
	}
	if !strings.Contains(yamlOut, "00000000") || !strings.Contains(yamlOut, "00000010") {
		t.Fatalf("YAML output lost hexdump string:\n%s", yamlOut)
	}
	if strings.Contains(yamlOut, `\n00000010`) {
		t.Fatalf("YAML output escaped newlines instead of using block content:\n%s", yamlOut)
	}
	if strings.Contains(yamlOut, "body: 0.0") {
		t.Fatalf("YAML output parsed hexdump as number:\n%s", yamlOut)
	}
	subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s"))
	if strings.HasSuffix(subject, " 0") {
		t.Fatalf("commit subject used JSON-prefix preview: %q", subject)
	}
}

func TestYAMLRoundTripPreservesRepresentation(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "# top\nz: 0x1A # hex spelling\ndefaults: &defaults\n  retries: 3\nitems:\n  - *defaults\na: yes\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "set", "config.yaml", "status", "draft")

	got := testGit(t, dir, "show", "HEAD:config.yaml")
	for _, want := range []string{"# top", "0x1A # hex spelling", "&defaults", "- *defaults", "a: yes", "status: draft"} {
		if !strings.Contains(got, want) {
			t.Fatalf("YAML output missing %q:\n%s", want, got)
		}
	}
	for _, pair := range [][2]string{
		{"z: 0x1A", "defaults: &defaults"},
		{"defaults: &defaults", "items:"},
		{"items:", "a: yes"},
		{"a: yes", "status: draft"},
	} {
		if strings.Index(got, pair[0]) > strings.Index(got, pair[1]) {
			t.Fatalf("YAML key order changed around %q and %q:\n%s", pair[0], pair[1], got)
		}
	}
}

func TestYAMLAnchorMutationPreservesAliases(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "defaults: &defaults\n  status: open\ncopy: *defaults\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "set", "config.yaml", "defaults.status", "closed")

	got := testGit(t, dir, "show", "HEAD:config.yaml")
	if !strings.Contains(got, "defaults: &defaults\n  status: closed") || !strings.Contains(got, "copy: *defaults") {
		t.Fatalf("YAML anchor/alias output:\n%s", got)
	}
}

func TestYAMLSelectorBelowAliasFails(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "defaults: &defaults\n  status: open\ncopy: *defaults\n")
	head := commitAll(t, dir, "initial")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"set", "config.yaml", "copy.status", "closed"}, &out, &errb)
	if err == nil || code == exitOK {
		t.Fatalf("alias mutation succeeded stdout=%s stderr=%s", out.String(), errb.String())
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed alias mutation moved HEAD to %s", got)
	}
}

func TestFrontmatterRoundTripPreservesRepresentation(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "---\n# metadata\ntitle: Old\ntags: &tags\n  - a\ncopy: *tags\n---\n# Note\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "set", "note.md", "frontmatter.status", "draft")

	got := testGit(t, dir, "show", "HEAD:note.md")
	for _, want := range []string{"# metadata", "title: Old", "tags: &tags", "copy: *tags", "status: draft", "# Note"} {
		if !strings.Contains(got, want) {
			t.Fatalf("frontmatter output missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "copy: *tags") > strings.Index(got, "status: draft") {
		t.Fatalf("frontmatter key order changed:\n%s", got)
	}
}

func TestYAMLVerbMatrixPreservesRepresentation(t *testing.T) {
	cases := []struct {
		name      string
		before    string
		args      []string
		want      []string
		absent    []string
		unchanged bool
	}{
		{
			name:   "delete key",
			before: "# top\na: 1\nb: 2 # keep\nc: 3\n",
			args:   []string{"delete", "config.yaml", "a"},
			want:   []string{"# top", "b: 2 # keep", "c: 3"},
			absent: []string{"a: 1"},
		},
		{
			name:   "append array",
			before: "items:\n  # keep\n  - a\n",
			args:   []string{"append", "config.yaml", "items", `"b"`},
			want:   []string{"items:", "# keep", "- a", "- b"},
		},
		{
			name:      "add semantic object no-op",
			before:    "items:\n  - b: 2\n    a: 1\n",
			args:      []string{"add", "config.yaml", "items", `{"a":1,"b":2}`},
			want:      []string{"items:", "b: 2", "a: 1"},
			unchanged: true,
		},
		{
			name:   "remove all semantic matches",
			before: "items:\n  - x\n  - y\n  - x\n",
			args:   []string{"remove", "config.yaml", "items", `"x"`},
			want:   []string{"items:", "- y"},
			absent: []string{"- x"},
		},
		{
			name:   "set array index",
			before: "items:\n  - old\n  - stay\n",
			args:   []string{"set", "config.yaml", "items[0]", "new"},
			want:   []string{"items:", "- new", "- stay"},
			absent: []string{"- old"},
		},
		{
			name:   "set array append index",
			before: "items:\n  - old\n",
			args:   []string{"set", "config.yaml", "items[1]", "new"},
			want:   []string{"items:", "- old", "- new"},
		},
		{
			name:   "delete array index",
			before: "items:\n  - a\n  - b\n  - c\n",
			args:   []string{"delete", "config.yaml", "items[1]"},
			want:   []string{"items:", "- a", "- c"},
			absent: []string{"- b"},
		},
		{
			name:   "append nested array selected by index",
			before: "matrix:\n  - - a\n",
			args:   []string{"append", "config.yaml", "matrix[0]", `"b"`},
			want:   []string{"matrix:", "- - a", "  - b"},
		},
		{
			name:   "remove nested array selected by index",
			before: "matrix:\n  - - x\n    - y\n    - x\n",
			args:   []string{"remove", "config.yaml", "matrix[0]", `"x"`},
			want:   []string{"matrix:", "- - y"},
			absent: []string{"- x"},
		},
		{
			name:   "create nested containers",
			before: "# top\n",
			args:   []string{"set", "config.yaml", "parent.child.value", "1"},
			want:   []string{"# top", "parent:", "  child:", "    value: 1"},
		},
		{
			name:   "root set",
			before: "old: value\n",
			args:   []string{"yaml", "set", "config.yaml", "$", `{"next":true}`},
			want:   []string{"next: true"},
			absent: []string{"old: value"},
		},
		{
			name:   "root append",
			before: "- a\n",
			args:   []string{"yaml", "append", "config.yaml", "$", `"b"`},
			want:   []string{"- a", "- b"},
		},
		{
			name:   "flow style replacement",
			before: "flow: [old]\n",
			args:   []string{"set", "config.yaml", "flow[0]", `{"a":"one"}`},
			want:   []string{"flow: [{a: one}]"},
		},
		{
			name:   "bracket string selector",
			before: "a.b:\n  c: old\n",
			args:   []string{"set", "config.yaml", `["a.b"].c`, "new"},
			want:   []string{"a.b:", "  c: new"},
			absent: []string{"c: old"},
		},
		{
			name:   "tagged value preserved",
			before: "special: !custom value\n",
			args:   []string{"set", "config.yaml", "status", "draft"},
			want:   []string{"special: !custom value", "status: draft"},
		},
		{
			name:   "set below tagged mapping",
			before: "root: !custom\n  child: old\n",
			args:   []string{"set", "config.yaml", "root.child", "new"},
			want:   []string{"root: !custom", "  child: new"},
			absent: []string{"child: old"},
		},
		{
			name:   "append tagged sequence",
			before: "items: !custom\n  - a\n",
			args:   []string{"append", "config.yaml", "items", `"b"`},
			want:   []string{"items: !custom", "- a", "- b"},
		},
		{
			name:   "set tagged sequence index",
			before: "items: !custom\n  - old\n",
			args:   []string{"set", "config.yaml", "items[0]", "new"},
			want:   []string{"items: !custom", "- new"},
			absent: []string{"- old"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "config.yaml", tc.before)
			head := commitAll(t, dir, "initial")
			chdir(t, dir)

			var out, errb bytes.Buffer
			code, err := runCLI(tc.args, &out, &errb)
			if err != nil || code != exitOK {
				t.Fatalf("etch %v code=%d err=%v stdout=%s stderr=%s", tc.args, code, err, out.String(), errb.String())
			}
			gotHead := stringsTrim(testGit(t, dir, "rev-parse", "HEAD"))
			if tc.unchanged {
				if gotHead != head {
					t.Fatalf("no-op changed HEAD to %s", gotHead)
				}
			} else if gotHead == head {
				t.Fatal("mutation did not commit")
			}
			got := testGit(t, dir, "show", "HEAD:config.yaml")
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("YAML output missing %q:\n%s", want, got)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(got, absent) {
					t.Fatalf("YAML output still contains %q:\n%s", absent, got)
				}
			}
		})
	}
}

func TestYAMLErrorMatrixDoesNotCommit(t *testing.T) {
	cases := []struct {
		name   string
		before string
		args   []string
	}{
		{
			name:   "set array index out of range",
			before: "items:\n  - a\n",
			args:   []string{"set", "config.yaml", "items[2]", "new"},
		},
		{
			name:   "append through scalar array item",
			before: "items:\n  - a\n",
			args:   []string{"append", "config.yaml", "items[0]", `"b"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "config.yaml", tc.before)
			head := commitAll(t, dir, "initial")
			chdir(t, dir)

			var out, errb bytes.Buffer
			code, err := runCLI(tc.args, &out, &errb)
			if err == nil || code == exitOK {
				t.Fatalf("etch %v unexpectedly succeeded stdout=%s stderr=%s", tc.args, out.String(), errb.String())
			}
			if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
				t.Fatalf("failed mutation moved HEAD to %s", got)
			}
		})
	}
}

func TestFrontmatterVerbMatrixPreservesRepresentation(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "---\n# metadata\ntitle: Old\ntags:\n  - a\nremove:\n  - x\n  - y\n---\n# Note\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "delete", "note.md", "frontmatter.title")
	runOK(t, "append", "note.md", "frontmatter.tags", `"b"`)
	runOK(t, "remove", "note.md", "frontmatter.remove", `"x"`)

	got := testGit(t, dir, "show", "HEAD:note.md")
	for _, want := range []string{"# metadata", "tags:", "- a", "- b", "remove:", "- y", "# Note"} {
		if !strings.Contains(got, want) {
			t.Fatalf("frontmatter output missing %q:\n%s", want, got)
		}
	}
	for _, absent := range []string{"title: Old", "- x"} {
		if strings.Contains(got, absent) {
			t.Fatalf("frontmatter output still contains %q:\n%s", absent, got)
		}
	}
}

func TestFrontmatterMultilineStringUsesLiteralBlock(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "# Note\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "set", "note.md", "frontmatter.body", "line one\nline two\n")

	mdOut := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(mdOut, "body: |") || !strings.Contains(mdOut, "  line one\n  line two\n") {
		t.Fatalf("frontmatter did not use literal block scalar:\n%s", mdOut)
	}
	if strings.Contains(mdOut, `line one\nline two`) {
		t.Fatalf("frontmatter escaped newlines instead of using block content:\n%s", mdOut)
	}
}

func TestMarkdownReplaceSection(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		content string
		want    []string
		reject  []string
	}{
		{
			name:    "replaces body until next peer heading",
			input:   "# Title\n\n## Notes\nold\n\n## Next\nkeep\n",
			content: "new\nbody\n",
			want:    []string{"## Notes\nnew\nbody\n## Next"},
			reject:  []string{"old"},
		},
		{
			name:    "ignores fenced headings",
			input:   "# Title\n\n```md\n## Notes\nold\n```\n\n## Notes\nreal\n\n## Next\nkeep\n",
			content: "new\n",
			want:    []string{"```md\n## Notes\nold\n```", "## Notes\nnew\n## Next"},
			reject:  []string{"real"},
		},
		{
			name:    "ignores HTML block headings",
			input:   "# Title\n\n<div>\n## Notes\nold\n</div>\n\n## Notes\nreal\n\n## Next\nkeep\n",
			content: "new\n",
			want:    []string{"<div>\n## Notes\nold\n</div>", "## Notes\nnew\n## Next"},
			reject:  []string{"real"},
		},
		{
			name:    "stops at setext heading",
			input:   "# Title\n\n## Notes\nold\n\nNext\n----\nkeep\n",
			content: "new\n",
			want:    []string{"## Notes\nnew\nNext\n----\nkeep\n"},
			reject:  []string{"old"},
		},
		{
			name:    "matches closing ATX markers",
			input:   "# Title\n\n## Notes ##\nold\n\n## Next\nkeep\n",
			content: "new\n",
			want:    []string{"## Notes ##\nnew\n## Next"},
			reject:  []string{"old"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "note.md", tc.input)
			commitAll(t, dir, "initial")
			chdir(t, dir)

			runOK(t, "replace-section", "note.md", "## Notes", tc.content)
			got := testGit(t, dir, "show", "HEAD:note.md")
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("replace-section output missing %q:\n%s", want, got)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(got, reject) {
					t.Fatalf("replace-section output still contains %q:\n%s", reject, got)
				}
			}
		})
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

func TestMarkdownTableVerbsIgnoreIndentedCodeTables(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n    | sku | status |\n    | --- | --- |\n    | A1 | open |\n\n| sku | status |\n| --- | --- |\n| A1 | open |\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "table", "set", "note.md", "## Inventory", "sku=A1,status", "done")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "    | A1 | open |") {
		t.Fatalf("Markdown table edit rewrote indented code:\n%s", got)
	}
	if !strings.Contains(got, "| A1 | done |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestMarkdownTableVerbsPreservePrecedingParagraph(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\nintro\n| sku | status |\n| --- | --- |\n| A1 | open |\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "table", "set", "note.md", "## Inventory", "sku=A1,status", "done")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "intro\n| sku | status |") || !strings.Contains(got, "| A1 | done |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestMarkdownTableVerbsReadEscapedPipes(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n| sku | status |\n| --- | --- |\n| A\\|1 | open |\n")
	commitAll(t, dir, "initial")
	chdir(t, dir)

	runOK(t, "table", "set", "note.md", "## Inventory", "sku=A|1,status", "done")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "| A\\|1 | done |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestMarkdownTableScopeRejectsAmbiguousHeading(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n| sku | status |\n| --- | --- |\n| A1 | open |\n\n## Inventory\n\n| sku | status |\n| --- | --- |\n| B2 | open |\n")
	head := commitAll(t, dir, "initial")
	chdir(t, dir)

	var out, errb bytes.Buffer
	code, err := runCLI([]string{"table", "set", "note.md", "## Inventory", "sku=A1,status", "done"}, &out, &errb)
	if err == nil || code == exitOK {
		t.Fatalf("ambiguous scope succeeded stdout=%s stderr=%s", out.String(), errb.String())
	}
	if !strings.Contains(err.Error(), `markdown scope "## Inventory" is ambiguous`) {
		t.Fatalf("err = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed scope mutation moved HEAD to %s", got)
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
