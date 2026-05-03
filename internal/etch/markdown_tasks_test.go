package etch

import (
	"strings"
	"testing"
)

func markdownTaskListOp(verb, text string, address markdownAddress) Operation {
	op := Operation{
		Verb:      verb,
		Kind:      "md-task-list",
		Class:     ClassIdempotent,
		Path:      "note.md",
		Target:    PlanTarget{Path: "note.md", Part: "body"},
		Value:     text,
		ValueMode: ValueModeString,
		Markdown:  address,
	}
	if verb == "list add" || verb == "task add" {
		op.Class = ClassNonIdempotent
	}
	fillDescriptor(&op)
	return op
}

func TestEvalMarkdownTaskOpenClose(t *testing.T) {
	before := []byte(strings.Join([]string{
		"# Note",
		"",
		"## Actions",
		"- [ ] Send follow-up",
		"- [x] File report",
		"- [X] Archive thread",
		"",
	}, "\n"))

	got, changed, err := evalMarkdownTaskList("note.md", markdownTaskListOp("task close", "Send follow-up", markdownAddress{Section: "Actions"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !strings.Contains(string(got), "- [x] Send follow-up") {
		t.Fatalf("task close output changed=%v:\n%s", changed, got)
	}

	got, changed, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task open", "File report", markdownAddress{Section: "Actions"}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !strings.Contains(string(got), "- [ ] File report") {
		t.Fatalf("task open output changed=%v:\n%s", changed, got)
	}

	got, changed, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task open", "Archive thread", markdownAddress{Section: "Actions"}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || !strings.Contains(string(got), "- [ ] Archive thread") {
		t.Fatalf("task open uppercase output changed=%v:\n%s", changed, got)
	}

	_, changed, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task open", "Archive thread", markdownAddress{Section: "Actions"}), got)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("task open of already-open task changed the file")
	}
}

func TestEvalMarkdownTaskOpenCreatesOnlyWithDestination(t *testing.T) {
	before := []byte("# Note\n\n## Actions\n")

	got, changed, err := evalMarkdownTaskList("note.md", markdownTaskListOp("task open", "Send follow-up", markdownAddress{Section: "Actions"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "# Note\n\n## Actions\n- [ ] Send follow-up\n" {
		t.Fatalf("task open create changed=%v got=%q", changed, got)
	}

	_, _, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task open", "Missing", markdownAddress{}), before)
	if err == nil || !strings.Contains(err.Error(), "requires a destination address") {
		t.Fatalf("bare missing task open err = %v", err)
	}
}

func TestEvalMarkdownTaskCustomStatusFails(t *testing.T) {
	before := []byte("- [>] Waiting\n")
	_, _, err := evalMarkdownTaskList("note.md", markdownTaskListOp("task close", "Waiting", markdownAddress{}), before)
	if err == nil || !strings.Contains(err.Error(), "unsupported checkbox status") {
		t.Fatalf("custom close err = %v", err)
	}
	_, _, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task open", "Waiting", markdownAddress{}), before)
	if err == nil || !strings.Contains(err.Error(), "unsupported checkbox status") {
		t.Fatalf("custom open err = %v", err)
	}
}

func TestEvalMarkdownTaskAnchorsUseListItems(t *testing.T) {
	before := []byte(strings.Join([]string{
		"## Actions",
		"Prose anchor",
		"- [ ] Anchor",
		"- [ ] Send follow-up",
		"",
	}, "\n"))

	got, changed, err := evalMarkdownTaskList("note.md", markdownTaskListOp("task close", "Send follow-up", markdownAddress{Section: "Actions", After: "Anchor"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "## Actions\nProse anchor\n- [ ] Anchor\n- [x] Send follow-up\n" {
		t.Fatalf("task close after item changed=%v got=%q", changed, got)
	}

	_, _, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task close", "Send follow-up", markdownAddress{Section: "Actions", After: "Prose anchor"}), before)
	if err == nil || !strings.Contains(err.Error(), "item \"Prose anchor\" not found") {
		t.Fatalf("prose anchor err = %v", err)
	}
}

func TestEvalMarkdownListAndTaskAdd(t *testing.T) {
	before := []byte(strings.Join([]string{
		"## Actions",
		"- Existing",
		"",
		"## Later",
		"keep",
		"",
	}, "\n"))

	got, changed, err := evalMarkdownTaskList("note.md", markdownTaskListOp("list add", "Next", markdownAddress{Section: "Actions"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "## Actions\n- Existing\n- Next\n\n## Later\nkeep\n" {
		t.Fatalf("list add changed=%v got=%q", changed, got)
	}

	got, changed, err = evalMarkdownTaskList("note.md", markdownTaskListOp("task add", "Before next", markdownAddress{Section: "Actions", Before: "Next"}), got)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "## Actions\n- Existing\n- [ ] Before next\n- Next\n\n## Later\nkeep\n" {
		t.Fatalf("task add before changed=%v got=%q", changed, got)
	}
}

func TestEvalMarkdownListAddWithoutDestinationRejectsAmbiguousTargets(t *testing.T) {
	before := []byte(strings.Join([]string{
		"## Actions",
		"- Existing",
		"",
		"## Later",
		"- Other",
		"",
	}, "\n"))

	_, _, err := evalMarkdownTaskList("note.md", markdownTaskListOp("list add", "Next", markdownAddress{}), before)
	if err == nil || !strings.Contains(err.Error(), "ambiguous without --section") {
		t.Fatalf("ambiguous list add err = %v", err)
	}
}

func TestEvalMarkdownListAddWithoutDestinationAllowsSingleObviousList(t *testing.T) {
	before := []byte(strings.Join([]string{
		"## Actions",
		"- Existing",
		"",
	}, "\n"))

	got, changed, err := evalMarkdownTaskList("note.md", markdownTaskListOp("list add", "Next", markdownAddress{}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "## Actions\n- Existing\n- Next\n" {
		t.Fatalf("list add changed=%v got=%q", changed, got)
	}
}

func TestEvalMarkdownListAddNumberedAndValidation(t *testing.T) {
	before := []byte("## Steps\n1. First\n")
	got, changed, err := evalMarkdownTaskList("note.md", markdownTaskListOp("task add", "Second", markdownAddress{Section: "Steps"}), before)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || string(got) != "## Steps\n1. First\n2. [ ] Second\n" {
		t.Fatalf("numbered task add changed=%v got=%q", changed, got)
	}

	for _, text := range []string{"- Already sourced", "line one\nline two", " \t "} {
		_, _, err := evalMarkdownTaskList("note.md", markdownTaskListOp("list add", text, markdownAddress{Section: "Steps"}), before)
		if err == nil {
			t.Fatalf("list add accepted invalid text %q", text)
		}
	}
}
