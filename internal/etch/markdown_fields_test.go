package etch

import (
	"strings"
	"testing"
)

func markdownFieldOp(verb, field, value string, address markdownAddress) Operation {
	op := Operation{
		Verb:      verb,
		Kind:      "md-field",
		Class:     ClassIdempotent,
		Path:      "note.md",
		Target:    PlanTarget{Path: "note.md", Part: "inline-field", Selector: field},
		Value:     value,
		ValueMode: ValueModeString,
		Markdown:  address,
	}
	fillDescriptor(&op)
	return op
}

func TestDataviewFieldNameNormalization(t *testing.T) {
	tests := map[string]string{
		"test":             "test",
		"property":         "property",
		"test thing":       "test-thing",
		"This     is test": "this-is-test",
		"test thing 3":     "test-thing-3",
		"This is a Test.":  "this-is-a-test",
		"Yes-sir":          "yes-sir",
		"📷":                "📷",
		"Статус":           "статус",
		"Last Run":         "last-run",
		"last_run":         "last_run",
		"**Last** Run!":    "last-run",
		"Created (date)":   "created-date",
	}
	for input, want := range tests {
		if got := dataviewFieldName(input); got != want {
			t.Fatalf("dataviewFieldName(%q) = %q, want %q", input, got, want)
		}
	}
	if dataviewFieldName("last_run") == dataviewFieldName("last run") {
		t.Fatal("underscore normalized as whitespace alias")
	}
}

func TestMarkdownInlineFieldsParseFormsAndIgnoreCode(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"Key:: Value",
		"Paragraph [done:: yes] and (trace:: hidden).",
		"`[skip:: inline code]`",
		"```",
		"code:: no",
		"```",
		"    indented:: no",
		"",
	}, "\n"))
	scope := markdownRange{Start: 0, End: len(raw)}
	fields := markdownInlineFields(raw, scope)
	if len(fields) != 3 {
		t.Fatalf("fields = %#v", fields)
	}
	wants := []struct {
		name string
		form markdownInlineFieldForm
		val  string
	}{
		{"Key", markdownInlineFieldFullLine, "Value"},
		{"done", markdownInlineFieldBracket, "yes"},
		{"trace", markdownInlineFieldParen, "hidden"},
	}
	for i, want := range wants {
		if fields[i].RawName != want.name || fields[i].Form != want.form || string(raw[fields[i].ValueStart:fields[i].ValueEnd]) != want.val {
			t.Fatalf("field %d = %#v value=%q, want %#v", i, fields[i], raw[fields[i].ValueStart:fields[i].ValueEnd], want)
		}
	}
}

func TestEvalMarkdownFieldUpdatesPreserveSourceForm(t *testing.T) {
	before := []byte("Last Run ::  yesterday\n\n- [ ] Send [done:: no]\n\n(trace:: old)\n")
	got, changed, err := evalMarkdownField("note.md", markdownFieldOp("set", "last-run", "today", markdownAddress{Body: true}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("field update reported no change")
	}
	want := "Last Run ::  today\n\n- [ ] Send [done:: no]\n\n(trace:: old)\n"
	if string(got) != want {
		t.Fatalf("full-line update:\n%q\nwant:\n%q", got, want)
	}

	got, changed, err = evalMarkdownField("note.md", markdownFieldOp("set", "done", "yes", markdownAddress{Task: "Send"}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "Last Run ::  today\n\n- [ ] Send [done:: yes]\n\n(trace:: old)\n" {
		t.Fatalf("item update changed=%v got=%q", changed, got)
	}
}

func TestEvalMarkdownFieldExactMatchBeatsNormalizedMatch(t *testing.T) {
	before := []byte("last run:: normalized\nlast-run:: exact\n")
	got, _, err := evalMarkdownField("note.md", markdownFieldOp("set", "last-run", "updated", markdownAddress{Body: true}), before)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "last run:: normalized\nlast-run:: updated\n" {
		t.Fatalf("exact match output = %q", got)
	}

	if _, _, err := evalMarkdownField("note.md", markdownFieldOp("set", "last", "updated", markdownAddress{Body: true}), []byte("Last!:: a\nlast?:: b\n")); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("normalized collision err = %v", err)
	}
}

func TestEvalMarkdownFieldCreatesBodySectionTaskAndHiddenFields(t *testing.T) {
	before := []byte("# Title\n\n## Status\nold\n\n- [ ] Send follow-up\n")
	got, _, err := evalMarkdownField("note.md", markdownFieldOp("set", "last", "2026-05-02", markdownAddress{Body: true}), before)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# Title\n\n## Status\nold\n\n- [ ] Send follow-up\n\nlast:: 2026-05-02\n" {
		t.Fatalf("body create = %q", got)
	}
	got, _, err = evalMarkdownField("note.md", markdownFieldOp("set", "snooze", "2026-05-06", markdownAddress{Section: "Status", Head: true}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "## Status\nsnooze:: 2026-05-06\n\nold\n") {
		t.Fatalf("section create = %q", got)
	}
	got, _, err = evalMarkdownField("note.md", markdownFieldOp("set", "trace-id", "abc123", markdownAddress{Task: "Send follow-up", Hidden: true}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "- [ ] Send follow-up (trace-id:: abc123)\n") {
		t.Fatalf("task hidden create = %q", got)
	}
}

func TestEvalMarkdownFieldUsesAnchorsForCreationAndMatching(t *testing.T) {
	before := []byte("first:: old\nanchor\nlast:: old\n")
	got, _, err := evalMarkdownField("note.md", markdownFieldOp("set", "last", "new", markdownAddress{Body: true, After: "anchor"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first:: old\nanchor\nlast:: new\n" {
		t.Fatalf("anchored update = %q", got)
	}

	got, _, err = evalMarkdownField("note.md", markdownFieldOp("set", "inserted", "value", markdownAddress{Body: true, Before: "anchor"}), got)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first:: old\n\ninserted:: value\n\nanchor\nlast:: new\n" {
		t.Fatalf("anchored create = %q", got)
	}
}

func TestEvalMarkdownFieldDeleteFullLineAndInline(t *testing.T) {
	before := []byte("done:: yes\n- [ ] Send [done:: yes]\n")
	got, changed, err := evalMarkdownField("note.md", markdownFieldOp("delete", "done", "", markdownAddress{Task: "Send"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "done:: yes\n- [ ] Send\n" {
		t.Fatalf("inline delete changed=%v got=%q", changed, got)
	}
	got, changed, err = evalMarkdownField("note.md", markdownFieldOp("delete", "done", "", markdownAddress{Body: true}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "- [ ] Send\n" {
		t.Fatalf("full-line delete changed=%v got=%q", changed, got)
	}
}
