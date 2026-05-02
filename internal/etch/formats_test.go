package etch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brandonbloom/etch/internal/jsonx"
)

func runOK(t *testing.T, dir string, args ...string) {
	t.Helper()
	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, args, &out, &errb)
	if err != nil || code != exitOK {
		t.Fatalf("etch %v code=%d err=%v stdout=%s stderr=%s", args, code, err, out.String(), errb.String())
	}
}

func TestJSONVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"tags":["a"],"remove":["x","y","x"]}`+"\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "append", "state.json", "tags", "--json", `"b"`)
	runOK(t, dir, "add", "state.json", "tags", "--json", `"b"`)
	runOK(t, dir, "remove", "state.json", "remove", "--json", `"x"`)
	runOK(t, dir, "delete", "state.json", "missing")
	got := testGit(t, dir, "show", "HEAD:state.json")
	if strings.Count(got, `"b"`) != 1 || strings.Contains(got, `"x"`) {
		t.Fatalf("JSON verbs result:\n%s", got)
	}
}

func TestEvalJSONLAppend(t *testing.T) {
	tests := []struct {
		name   string
		before string
		value  string
		want   string
	}{
		{
			name:   "empty file",
			before: "",
			value:  `{"kind":"prompt","n":2}`,
			want:   `{"kind":"prompt","n":2}` + "\n",
		},
		{
			name:   "existing records",
			before: `{"kind":"old"}` + "\n",
			value:  `{"kind":"prompt","nested":{"ok":true}}`,
			want:   `{"kind":"old"}` + "\n" + `{"kind":"prompt","nested":{"ok":true}}` + "\n",
		},
		{
			name:   "malformed non-tail record is not parsed",
			before: "not json\n{\"ok\":true}\n",
			value:  `12`,
			want:   "not json\n{\"ok\":true}\n12\n",
		},
		{
			name:   "non-object value",
			before: "",
			value:  `"hello"`,
			want:   "\"hello\"\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := evalJSONLAppend(tc.value, []byte(tc.before))
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("evalJSONLAppend reported no change")
			}
			if string(got) != tc.want {
				t.Fatalf("JSONL output:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestEvalJSONLAppendErrors(t *testing.T) {
	tests := []struct {
		name   string
		before string
		value  string
		want   string
	}{
		{
			name:   "missing trailing newline",
			before: `{"kind":"old"}`,
			value:  `{"kind":"new"}`,
			want:   "must end with a newline",
		},
		{
			name:   "blank tail boundary",
			before: `{"kind":"old"}` + "\n \n",
			value:  `{"kind":"new"}`,
			want:   "boundary is blank",
		},
		{
			name:   "invalid new value",
			before: "",
			value:  `{`,
			want:   "invalid JSONL value",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := evalJSONLAppend(tc.value, []byte(tc.before)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestJSONAddAndRemoveDistinguishLargeNumbers(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "state.json", `{"ids":[9007199254740992]}`+"\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "add", "state.json", "ids", "--json", "9007199254740993")
	got := testGit(t, dir, "show", "HEAD:state.json")
	if !strings.Contains(got, "9007199254740992") || !strings.Contains(got, "9007199254740993") {
		t.Fatalf("JSON add result:\n%s", got)
	}

	runOK(t, dir, "remove", "state.json", "ids", "--json", "9007199254740993")
	got = testGit(t, dir, "show", "HEAD:state.json")
	if !strings.Contains(got, "9007199254740992") || strings.Contains(got, "9007199254740993") {
		t.Fatalf("JSON remove result:\n%s", got)
	}
}

func TestJSONRemoveAdjacentArrayElements(t *testing.T) {
	tests := []struct {
		name   string
		before string
		want   string
	}{
		{
			name:   "leading run",
			before: `{"items":["x","x","y"]}` + "\n",
			want:   `{"items":["y"]}` + "\n",
		},
		{
			name:   "trailing run",
			before: `{"items":["y","x","x"]}` + "\n",
			want:   `{"items":["y"]}` + "\n",
		},
		{
			name:   "entire array",
			before: `{"items":["x","x"]}` + "\n",
			want:   `{"items":[]}` + "\n",
		},
		{
			name:   "separate runs",
			before: `{"items":["x","y","x","x","z","x"]}` + "\n",
			want:   `{"items":["y","z"]}` + "\n",
		},
		{
			name:   "multiline leading run",
			before: "{\n  \"items\": [\n    \"x\",\n    \"x\",\n    \"y\"\n  ]\n}\n",
			want:   "{\n  \"items\": [\n    \"y\"\n  ]\n}\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := evalJSON("items", "remove", `"x"`, ValueModeJSON, []byte(tc.before))
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("evalJSON reported no change")
			}
			if string(got) != tc.want {
				t.Fatalf("JSON output:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

func TestJSONSetPreservesSurroundingSource(t *testing.T) {
	before := []byte("{\n  \"z\": 0,\n  \"status\" : \"open\",\n  \"nested\": {\"keep\":true}\n}\n")

	got, changed, err := evalJSON("status", "set", "complete", ValueModeString, before)
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

	got, changed, err := evalJSON("status", "set", "complete", ValueModeString, before)
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

	got, changed, err := evalJSON("status", "set", "complete", ValueModeString, before)
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

func TestJSONDeeplyNestedEdits(t *testing.T) {
	before := []byte(`{"outer":{"items":[{"name":"a","tags":["x","y"]},{"name":"b","tags":["z"]}],"keep":true}}` + "\n")

	got, changed, err := evalJSON("outer.items[1].tags[0]", "set", "done", ValueModeString, before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("evalJSON reported no change")
	}
	want := `{"outer":{"items":[{"name":"a","tags":["x","y"]},{"name":"b","tags":["done"]}],"keep":true}}` + "\n"
	if string(got) != want {
		t.Fatalf("JSON output = %s, want %s", got, want)
	}

	got, changed, err = evalJSON("outer.items[0].tags", "remove", `"y"`, ValueModeJSON, got)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("evalJSON reported no change")
	}
	want = `{"outer":{"items":[{"name":"a","tags":["x"]},{"name":"b","tags":["done"]}],"keep":true}}` + "\n"
	if string(got) != want {
		t.Fatalf("JSON output = %s, want %s", got, want)
	}
}

func TestJSONDeleteLastArrayElementWhitespace(t *testing.T) {
	before := []byte("{\n  \"items\": [\n    \"a\",\n    \"b\"\n  ]\n}\n")

	got, changed, err := evalJSON("items[1]", "delete", "", "", before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("evalJSON reported no change")
	}
	want := "{\n  \"items\": [\n    \"a\"\n  ]\n}\n"
	if string(got) != want {
		t.Fatalf("JSON output:\n%s\nwant:\n%s", got, want)
	}
}

func TestYAMLAndFrontmatterVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "tags:\n  - a\n")
	writeFile(t, dir, "note.md", "# Note\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "yaml", "append", "config.yaml", "tags", "--json", `"b"`)
	runOK(t, dir, "set", "note.md", "status", "draft")
	runOK(t, dir, "add", "note.md", "tags", "--json", `"x"`)
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

	runOK(t, dir, "append", "config.yaml", "tags", "--json", `"b"`)
	runOK(t, dir, "set", "note.md", "status", "draft")
	runOK(t, dir, "add", "other.md", "tags", "--json", `"x"`)

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

func TestYAMLAndFrontmatterPreserveLargeNumbers(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "ids:\n  - 9007199254740992\n")
	writeFile(t, dir, "note.md", "# Note\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "add", "config.yaml", "ids", "--json", "9007199254740993")
	runOK(t, dir, "set", "config.yaml", "id", "--json", "9007199254740993")
	runOK(t, dir, "set", "note.md", "id", "--json", "9007199254740993")

	yamlOut := testGit(t, dir, "show", "HEAD:config.yaml")
	for _, want := range []string{"9007199254740992", "9007199254740993", "id: 9007199254740993"} {
		if !strings.Contains(yamlOut, want) {
			t.Fatalf("YAML output missing %q:\n%s", want, yamlOut)
		}
	}

	mdOut := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(mdOut, "id: 9007199254740993") {
		t.Fatalf("frontmatter output:\n%s", mdOut)
	}
}

func TestYAMLSetHexdumpLikeStringDoesNotParseJSONPrefix(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "doc.yml", "body: value\n")
	commitAll(t, dir, "initial")

	hexdump := "00000000  aa bb cc dd  |....|\n00000010  00 11 22 33  |..\"3|\n"
	runOK(t, dir, "set", "doc.yml", "body", hexdump)

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

	runOK(t, dir, "set", "config.yaml", "status", "draft")

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

	runOK(t, dir, "set", "config.yaml", "defaults.status", "closed")

	got := testGit(t, dir, "show", "HEAD:config.yaml")
	if !strings.Contains(got, "defaults: &defaults\n  status: closed") || !strings.Contains(got, "copy: *defaults") {
		t.Fatalf("YAML anchor/alias output:\n%s", got)
	}
}

func TestYAMLSelectorBelowAliasFails(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "config.yaml", "defaults: &defaults\n  status: open\ncopy: *defaults\n")
	head := commitAll(t, dir, "initial")

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"set", "config.yaml", "copy.status", "closed"}, &out, &errb)
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

	runOK(t, dir, "set", "note.md", "status", "draft")

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
			args:   []string{"append", "config.yaml", "items", "--json", `"b"`},
			want:   []string{"items:", "# keep", "- a", "- b"},
		},
		{
			name:      "add semantic object no-op",
			before:    "items:\n  - b: 2\n    a: 1\n",
			args:      []string{"add", "config.yaml", "items", "--json", `{"a":1,"b":2}`},
			want:      []string{"items:", "b: 2", "a: 1"},
			unchanged: true,
		},
		{
			name:   "remove all semantic matches",
			before: "items:\n  - x\n  - y\n  - x\n",
			args:   []string{"remove", "config.yaml", "items", "--json", `"x"`},
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
			args:   []string{"append", "config.yaml", "matrix[0]", "--json", `"b"`},
			want:   []string{"matrix:", "- - a", "  - b"},
		},
		{
			name:   "remove nested array selected by index",
			before: "matrix:\n  - - x\n    - y\n    - x\n",
			args:   []string{"remove", "config.yaml", "matrix[0]", "--json", `"x"`},
			want:   []string{"matrix:", "- - y"},
			absent: []string{"- x"},
		},
		{
			name:   "create nested containers",
			before: "# top\n",
			args:   []string{"set", "config.yaml", "parent.child.value", "--json", "1"},
			want:   []string{"# top", "parent:", "  child:", "    value: 1"},
		},
		{
			name:   "root set",
			before: "old: value\n",
			args:   []string{"yaml", "set", "config.yaml", "$", "--json", `{"next":true}`},
			want:   []string{"next: true"},
			absent: []string{"old: value"},
		},
		{
			name:   "root append",
			before: "- a\n",
			args:   []string{"yaml", "append", "config.yaml", "$", "--json", `"b"`},
			want:   []string{"- a", "- b"},
		},
		{
			name:   "flow style replacement",
			before: "flow: [old]\n",
			args:   []string{"set", "config.yaml", "flow[0]", "--json", `{"a":"one"}`},
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
			args:   []string{"append", "config.yaml", "items", "--json", `"b"`},
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

			var out, errb bytes.Buffer
			code, err := runCLIAt(dir, tc.args, &out, &errb)
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
			args:   []string{"append", "config.yaml", "items[0]", "--json", `"b"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := initRepo(t)
			writeFile(t, dir, "config.yaml", tc.before)
			head := commitAll(t, dir, "initial")

			var out, errb bytes.Buffer
			code, err := runCLIAt(dir, tc.args, &out, &errb)
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

	runOK(t, dir, "delete", "note.md", "title")
	runOK(t, dir, "append", "note.md", "tags", "--json", `"b"`)
	runOK(t, dir, "remove", "note.md", "remove", "--json", `"x"`)

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

	runOK(t, dir, "set", "note.md", "body", "line one\nline two\n")

	mdOut := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(mdOut, "body: |") || !strings.Contains(mdOut, "  line one\n  line two\n") {
		t.Fatalf("frontmatter did not use literal block scalar:\n%s", mdOut)
	}
	if strings.Contains(mdOut, `line one\nline two`) {
		t.Fatalf("frontmatter escaped newlines instead of using block content:\n%s", mdOut)
	}
}

func TestMarkdownPorcelainRejectsOldFrontmatterSelector(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "# Note\n")
	head := commitAll(t, dir, "initial")

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"set", "note.md", "frontmatter.status", "draft"}, &out, &errb)
	if err == nil || code != exitUsage {
		t.Fatalf("old frontmatter selector succeeded code=%d err=%v stdout=%s stderr=%s", code, err, out.String(), errb.String())
	}
	if !strings.Contains(err.Error(), "frontmatter selectors are bare") {
		t.Fatalf("error = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed frontmatter selector moved HEAD to %s", got)
	}
}

func TestMarkdownInlineFieldCommandsCommitAndMaterialize(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "# Note\n\n- [ ] Send follow-up\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "set", "note.md", "done", "2026-05-02", "--task", "Send follow-up")

	got := testGit(t, dir, "show", "HEAD:note.md")
	want := "# Note\n\n- [ ] Send follow-up [done:: 2026-05-02]\n"
	if got != want {
		t.Fatalf("markdown inline field output = %q, want %q", got, want)
	}
	wt, err := os.ReadFile(filepath.Join(dir, "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(wt) != got {
		t.Fatalf("worktree not materialized:\nwt=%s\nhead=%s", wt, got)
	}
	subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s"))
	if subject != `etch set note.md done --task 'Send follow-up' "2026-05-02"` {
		t.Fatalf("subject = %q", subject)
	}

	runOK(t, dir, "delete", "note.md", "done", "--task", "Send follow-up")
	got = testGit(t, dir, "show", "HEAD:note.md")
	if got != "# Note\n\n- [ ] Send follow-up\n" {
		t.Fatalf("markdown inline field delete = %q", got)
	}
}

func TestMarkdownTaskListCommandsCommitAndMaterialize(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "# Note\n\n## Actions\n- [ ] Send follow-up\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "task", "close", "note.md", "Send follow-up", "--section", "Actions")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if got != "# Note\n\n## Actions\n- [x] Send follow-up\n" {
		t.Fatalf("task close output = %q", got)
	}
	subject := stringsTrim(testGit(t, dir, "log", "-1", "--format=%s"))
	if subject != `etch task close note.md --section Actions "Send follow-up"` {
		t.Fatalf("task close subject = %q", subject)
	}

	runOK(t, dir, "task", "open", "note.md", "Review draft", "--section", "Actions")
	got = testGit(t, dir, "show", "HEAD:note.md")
	if got != "# Note\n\n## Actions\n- [x] Send follow-up\n- [ ] Review draft\n" {
		t.Fatalf("task open create output = %q", got)
	}
	wt, err := os.ReadFile(filepath.Join(dir, "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(wt) != got {
		t.Fatalf("worktree not materialized:\nwt=%s\nhead=%s", wt, got)
	}
}

func TestEvalMarkdownSectionReplace(t *testing.T) {
	tests := []struct {
		name    string
		heading string
		input   string
		content string
		want    string
	}{
		{
			name:    "replaces body until next peer heading",
			heading: "## Notes",
			input:   "# Title\n\n## Notes\nold\n\n## Next\nkeep\n",
			content: "new\nbody\n",
			want:    "# Title\n\n## Notes\nnew\nbody\n## Next\nkeep\n",
		},
		{
			name:    "ignores fenced headings",
			heading: "## Notes",
			input:   "# Title\n\n```md\n## Notes\nold\n```\n\n## Notes\nreal\n\n## Next\nkeep\n",
			content: "new\n",
			want:    "# Title\n\n```md\n## Notes\nold\n```\n\n## Notes\nnew\n## Next\nkeep\n",
		},
		{
			name:    "ignores HTML block headings",
			heading: "## Notes",
			input:   "# Title\n\n<div>\n## Notes\nold\n</div>\n\n## Notes\nreal\n\n## Next\nkeep\n",
			content: "new\n",
			want:    "# Title\n\n<div>\n## Notes\nold\n</div>\n\n## Notes\nnew\n## Next\nkeep\n",
		},
		{
			name:    "stops at setext heading",
			heading: "## Notes",
			input:   "# Title\n\n## Notes\nold\n\nNext\n----\nkeep\n",
			content: "new\n",
			want:    "# Title\n\n## Notes\nnew\nNext\n----\nkeep\n",
		},
		{
			name:    "matches closing ATX markers",
			heading: "## Notes",
			input:   "# Title\n\n## Notes ##\nold\n\n## Next\nkeep\n",
			content: "new\n",
			want:    "# Title\n\n## Notes ##\nnew\n## Next\nkeep\n",
		},
		{
			name:    "matches title-only heading",
			heading: "Notes",
			input:   "# Title\n\n## Notes\nold\n\n## Next\nkeep\n",
			content: "new\n",
			want:    "# Title\n\n## Notes\nnew\n## Next\nkeep\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := evalMarkdownSection("note.md", "section replace", tc.heading, tc.content, []byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("section replace was not marked changed")
			}
			if string(got) != tc.want {
				t.Fatalf("section replace output:\n--- got ---\n%q\n--- want ---\n%q", got, tc.want)
			}
		})
	}
}

func TestEvalMarkdownSectionAppendPrepend(t *testing.T) {
	tests := []struct {
		name string
		verb string
		in   string
		body string
		want string
	}{
		{
			name: "append non-empty section",
			verb: "section append",
			in:   "# Title\n\n## Notes\nold\n\n\n## Next\nkeep\n",
			body: "\n\nnew\n\n",
			want: "# Title\n\n## Notes\nold\n\nnew\n## Next\nkeep\n",
		},
		{
			name: "prepend non-empty section",
			verb: "section prepend",
			in:   "# Title\n\n## Notes\n\nold\n\n## Next\nkeep\n",
			body: "\nnew\n\n",
			want: "# Title\n\n## Notes\nnew\n\nold\n\n## Next\nkeep\n",
		},
		{
			name: "append empty section",
			verb: "section append",
			in:   "# Title\n\n## Notes\n## Next\nkeep\n",
			body: "\nnew\n",
			want: "# Title\n\n## Notes\nnew\n## Next\nkeep\n",
		},
		{
			name: "prepend heading without trailing newline",
			verb: "section prepend",
			in:   "## Notes",
			body: "new",
			want: "## Notes\nnew\n",
		},
		{
			name: "append uses file newline style",
			verb: "section append",
			in:   "## Notes\r\nold\r\n",
			body: "new\nline",
			want: "## Notes\r\nold\r\n\r\nnew\r\nline\r\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := evalMarkdownSection("note.md", tc.verb, "Notes", tc.body, []byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if !changed {
				t.Fatal("section insertion was not marked changed")
			}
			if string(got) != tc.want {
				t.Fatalf("section output:\n--- got ---\n%q\n--- want ---\n%q", got, tc.want)
			}
		})
	}
}

func TestEvalMarkdownSectionAppendRejectsBlankFragment(t *testing.T) {
	_, _, err := evalMarkdownSection("note.md", "section append", "Notes", "\n \n", []byte("## Notes\nold\n"))
	if err == nil || !strings.Contains(err.Error(), "section fragment must not be blank") {
		t.Fatalf("err = %v", err)
	}
}

func TestMarkdownSectionCommandCommitsAndMaterializes(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Notes\nold\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "md", "section", "append", "note.md", "## Notes", "new")

	got := testGit(t, dir, "show", "HEAD:note.md")
	want := "## Notes\nold\n\nnew\n"
	if got != want {
		t.Fatalf("section command output:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
	subject := testGit(t, dir, "log", "-1", "--pretty=%s")
	if stringsTrim(subject) != `etch section append note.md '## Notes' "new"` {
		t.Fatalf("commit subject = %q", subject)
	}
}

func TestMarkdownSectionAmbiguousTitleOnlyHeading(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "# Title\n\n## Notes\nold\n\n### Notes\nnested\n")
	head := commitAll(t, dir, "initial")

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"section", "replace", "note.md", "Notes", "new\n"}, &out, &errb)
	if err == nil || code == exitOK {
		t.Fatalf("ambiguous section replace succeeded stdout=%s stderr=%s", out.String(), errb.String())
	}
	if !strings.Contains(err.Error(), `heading "Notes" is ambiguous`) {
		t.Fatalf("err = %v", err)
	}
	if got := stringsTrim(testGit(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("failed section mutation moved HEAD to %s", got)
	}
}

func TestCSVTableVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "data.csv", "id,status\n1,open\n2,open\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "table", "set", "data.csv", "all,status", "done")
	runOK(t, dir, "table", "row", "append", "data.csv", `{"id":"3","status":"open"}`)
	runOK(t, dir, "table", "column", "add", "data.csv", "owner", "--default", "Brandon")
	runOK(t, dir, "table", "column", "rename", "data.csv", "owner", "assignee")
	runOK(t, dir, "table", "row", "delete", "data.csv", "id=2")
	got := testGit(t, dir, "show", "HEAD:data.csv")
	if !strings.Contains(got, "id,status,assignee") || strings.Contains(got, "2,") || !strings.Contains(got, "Brandon") {
		t.Fatalf("CSV output:\n%s", got)
	}
}

func TestMarkdownTableVerbs(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n| sku | status |\n| --- | --- |\n| A1 | open |\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "table", "set", "note.md", "## Inventory", "sku=A1,status", "done")
	runOK(t, dir, "table", "row", "append", "note.md", "## Inventory", `{"sku":"B2","status":"open"}`)
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "| A1 | done |") || !strings.Contains(got, "| B2 | open |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestMarkdownTableVerbsIgnoreIndentedCodeTables(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n    | sku | status |\n    | --- | --- |\n    | A1 | open |\n\n| sku | status |\n| --- | --- |\n| A1 | open |\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "table", "set", "note.md", "## Inventory", "sku=A1,status", "done")
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

	runOK(t, dir, "table", "set", "note.md", "## Inventory", "sku=A1,status", "done")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "intro\n| sku | status |") || !strings.Contains(got, "| A1 | done |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestMarkdownTableVerbsReadEscapedPipes(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n| sku | status |\n| --- | --- |\n| A\\|1 | open |\n")
	commitAll(t, dir, "initial")

	runOK(t, dir, "table", "set", "note.md", "## Inventory", "sku=A|1,status", "done")
	got := testGit(t, dir, "show", "HEAD:note.md")
	if !strings.Contains(got, "| A\\|1 | done |") {
		t.Fatalf("Markdown table output:\n%s", got)
	}
}

func TestMarkdownTableScopeRejectsAmbiguousHeading(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, "note.md", "## Inventory\n\n| sku | status |\n| --- | --- |\n| A1 | open |\n\n## Inventory\n\n| sku | status |\n| --- | --- |\n| B2 | open |\n")
	head := commitAll(t, dir, "initial")

	var out, errb bytes.Buffer
	code, err := runCLIAt(dir, []string{"table", "set", "note.md", "## Inventory", "sku=A1,status", "done"}, &out, &errb)
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
	runOK(t, dir, "set", "state.json", "b", "2")
	got := []byte(testGit(t, dir, "show", "HEAD:state.json"))
	if len(got) < 3 || got[0] != 0xef || got[1] != 0xbb || got[2] != 0xbf {
		t.Fatalf("BOM not preserved: %v", got[:min(3, len(got))])
	}
}

func TestBOMPreservedForYAMLMarkdownAndCSV(t *testing.T) {
	dir := initRepo(t)
	bom := []byte{0xef, 0xbb, 0xbf}
	for path, content := range map[string]string{
		"config.yaml": "status: open\n",
		"note.md":     "# Title\n\n## Notes\nold\n",
		"data.csv":    "id,status\n1,open\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, path), append(append([]byte{}, bom...), []byte(content)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	commitAll(t, dir, "initial")

	runOK(t, dir, "set", "config.yaml", "status", "complete")
	runOK(t, dir, "section", "replace", "note.md", "## Notes", "new\n")
	runOK(t, dir, "table", "set", "data.csv", "all,status", "done")

	for _, path := range []string{"config.yaml", "note.md", "data.csv"} {
		got := []byte(testGit(t, dir, "show", "HEAD:"+path))
		if len(got) < len(bom) || !bytes.Equal(got[:len(bom)], bom) {
			t.Fatalf("%s BOM not preserved: %v", path, got[:min(len(bom), len(got))])
		}
	}
}

func TestInvalidUTF8RefusedForStructuredFormats(t *testing.T) {
	invalid := []byte{0xff, '\n'}
	tableOp, err := DecodeOperation(Statement{Tokens: []string{"table", "set", "data.csv", "all,status", "done"}})
	if err != nil {
		t.Fatal(err)
	}
	mdTableOp, err := DecodeOperation(Statement{Tokens: []string{"table", "set", "note.md", "doc", "all,status", "done"}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "json",
			run: func() error {
				_, _, err := evalJSON("a", "set", "1", ValueModeJSON, invalid)
				return err
			},
			want: "invalid UTF-8 in JSON input",
		},
		{
			name: "jsonl",
			run: func() error {
				_, _, err := evalJSONLAppend(`{"ok":true}`, invalid)
				return err
			},
			want: "invalid UTF-8 in JSONL input",
		},
		{
			name: "yaml",
			run: func() error {
				_, _, err := evalYAML("a", "set", jsonx.Number("1"), invalid)
				return err
			},
			want: "invalid UTF-8 in YAML input",
		},
		{
			name: "frontmatter",
			run: func() error {
				_, _, err := evalFrontmatter("note.md", "status", "set", "complete", invalid)
				return err
			},
			want: "invalid UTF-8 in Markdown input",
		},
		{
			name: "markdown section",
			run: func() error {
				_, _, err := evalMarkdownSection("note.md", "section replace", "## Notes", "new\n", invalid)
				return err
			},
			want: "invalid UTF-8 in Markdown input",
		},
		{
			name: "markdown table",
			run: func() error {
				_, _, err := evalTable("note.md", mdTableOp, invalid)
				return err
			},
			want: "invalid UTF-8 in Markdown input",
		},
		{
			name: "csv",
			run: func() error {
				_, _, err := evalTable("data.csv", tableOp, invalid)
				return err
			},
			want: "invalid UTF-8 in CSV input",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
