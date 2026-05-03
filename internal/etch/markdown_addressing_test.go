package etch

import (
	"strings"
	"testing"
)

func TestMarkdownBodyRangeSkipsFrontmatter(t *testing.T) {
	raw := []byte("---\ntitle: Hi\n---\n# Body\n")
	got, err := markdownBodyRange(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[got.Start:got.End]) != "# Body\n" {
		t.Fatalf("body range = %q", raw[got.Start:got.End])
	}

	got, err = markdownBodyRange([]byte("# Body\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got != (markdownRange{Start: 0, End: len("# Body\n")}) {
		t.Fatalf("body range without frontmatter = %#v", got)
	}
}

func TestResolveMarkdownSectionAddress(t *testing.T) {
	raw := []byte("# Title\n\n## Status ##\nopen\n\n### Status\nnested\n")

	section, err := resolveMarkdownSection(raw, "note.md", "## Status")
	if err != nil {
		t.Fatal(err)
	}
	if section.Heading.Level != 2 || section.Heading.Content != "Status" || string(raw[section.Heading.BodyStart:section.BodyEnd]) != "open\n\n### Status\nnested\n" {
		t.Fatalf("section = %#v body=%q", section, raw[section.Heading.BodyStart:section.BodyEnd])
	}

	if _, err := resolveMarkdownSection(raw, "note.md", "Status"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("title-only ambiguous err = %v", err)
	}
	if _, err := resolveMarkdownSection(raw, "note.md", "Missing"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing err = %v", err)
	}
}

func TestMarkdownPlacementFromFlags(t *testing.T) {
	got, err := markdownPlacementFromFlags(false, true, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != markdownPlacementTail {
		t.Fatalf("placement = %#v", got)
	}

	got, err = markdownPlacementFromFlags(false, false, "anchor", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != markdownPlacementBefore || got.Anchor != "anchor" {
		t.Fatalf("placement = %#v", got)
	}

	if _, err := markdownPlacementFromFlags(true, false, "", "anchor"); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("conflicting placement err = %v", err)
	}
}

func TestResolveMarkdownPlacementPoint(t *testing.T) {
	raw := []byte("# Title\n\n## Notes\nfirst anchor\nsecond anchor\n\n## Other\nanchor\n")
	section, err := resolveMarkdownSection(raw, "note.md", "Notes")
	if err != nil {
		t.Fatal(err)
	}
	scope := markdownRange{Start: section.Heading.BodyStart, End: section.BodyEnd}

	point, err := resolveMarkdownPlacementPoint(raw, "note.md", scope, markdownPlacement{Kind: markdownPlacementHead})
	if err != nil {
		t.Fatal(err)
	}
	if point != scope.Start {
		t.Fatalf("head point = %d, want %d", point, scope.Start)
	}

	point, err = resolveMarkdownPlacementPoint(raw, "note.md", scope, markdownPlacement{Kind: markdownPlacementTail})
	if err != nil {
		t.Fatal(err)
	}
	if point != scope.End {
		t.Fatalf("tail point = %d, want %d", point, scope.End)
	}

	point, err = resolveMarkdownPlacementPoint(raw, "note.md", scope, markdownPlacement{Kind: markdownPlacementBefore, Anchor: "second anchor"})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[point:point+len("second anchor")]) != "second anchor" {
		t.Fatalf("before point landed at %q", raw[point:point+len("second anchor")])
	}

	point, err = resolveMarkdownPlacementPoint(raw, "note.md", scope, markdownPlacement{Kind: markdownPlacementAfter, Anchor: "first anchor"})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[point:point+len("\nsecond")]) != "\nsecond" {
		t.Fatalf("after point landed at %q", raw[point:point+len("\nsecond")])
	}

	if _, err := resolveMarkdownPlacementPoint(raw, "note.md", scope, markdownPlacement{Kind: markdownPlacementBefore, Anchor: "anchor"}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous anchor err = %v", err)
	}
	if _, err := resolveMarkdownPlacementPoint(raw, "note.md", scope, markdownPlacement{Kind: markdownPlacementAfter, Anchor: "Other"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("out-of-scope anchor err = %v", err)
	}
}

func TestMarkdownItemTypeConstraints(t *testing.T) {
	taskNumbered, err := markdownItemTypeConstraintsFromArgs([]string{"task", "numbered"})
	if err != nil {
		t.Fatal(err)
	}
	if !taskNumbered.matches(markdownListItem{Task: true, Numbered: true}) {
		t.Fatal("task+numbered did not match numbered task")
	}
	if taskNumbered.matches(markdownListItem{Task: true, Numbered: false}) {
		t.Fatal("task+numbered matched bullet task")
	}

	for _, types := range [][]string{
		{"task", "plain"},
		{"numbered", "bullet"},
	} {
		if _, err := markdownItemTypeConstraintsFromArgs(types); err == nil || !strings.Contains(err.Error(), "contradictory") {
			t.Fatalf("types %v err = %v", types, err)
		}
	}
	if _, err := markdownItemTypeConstraintsFromArgs([]string{"checked"}); err == nil || !strings.Contains(err.Error(), "unknown item type") {
		t.Fatalf("unknown type err = %v", err)
	}
}

func TestMarkdownListItemsNormalizeAndClassify(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"- [ ] Bullet task [done:: 2026-05-02] [[62]](https://notes.granola.ai/d/57b65c7f-note)",
		"1. [x] Numbered task",
		"- Plain **bold** item",
		"2) Plain numbered",
		"",
	}, "\n"))

	items := markdownListItems(raw)
	if len(items) != 4 {
		t.Fatalf("items = %#v", items)
	}
	wants := []struct {
		text     string
		task     bool
		numbered bool
	}{
		{text: "Bullet task", task: true},
		{text: "Numbered task", task: true, numbered: true},
		{text: "Plain bold item"},
		{text: "Plain numbered", numbered: true},
	}
	for i, want := range wants {
		if items[i].Normalized != want.text || items[i].Task != want.task || items[i].Numbered != want.numbered || items[i].Complex {
			t.Fatalf("item %d = %#v, want %#v", i, items[i], want)
		}
	}
}

func TestResolveMarkdownItemIgnoresTrailingReferenceAnnotationLinks(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"- [ ] Send follow-up [[62]](https://notes.granola.ai/d/57b65c7f-note) [63](https://notes.granola.ai/d/other)",
		"- [ ] Read [docs](https://example.com)",
		"",
	}, "\n"))

	item, err := resolveMarkdownTask(raw, "note.md", "Send follow-up", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[item.LineStart:item.LineEnd]) != "- [ ] Send follow-up [[62]](https://notes.granola.ai/d/57b65c7f-note) [63](https://notes.granola.ai/d/other)\n" {
		t.Fatalf("citation task range = %q", raw[item.LineStart:item.LineEnd])
	}

	item, err = resolveMarkdownTask(raw, "note.md", "Read docs", nil)
	if err != nil {
		t.Fatal(err)
	}
	if item.Normalized != "Read docs" {
		t.Fatalf("ordinary link normalized to %q", item.Normalized)
	}

	if _, err := resolveMarkdownTask(raw, "note.md", "Read [docs](https://example.com)", nil); err != nil {
		t.Fatalf("source-style link selector err = %v", err)
	}
	if _, err := resolveMarkdownTask(raw, "note.md", "Read", nil); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ordinary trailing link err = %v", err)
	}
}

func TestResolveMarkdownItemAddress(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"- [ ] Buy milk",
		"1. [ ] Buy milk",
		"- Plain item",
		"",
	}, "\n"))

	if _, err := resolveMarkdownItem(raw, "note.md", "Buy milk", nil); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous item err = %v", err)
	}

	item, err := resolveMarkdownItem(raw, "note.md", "- [ ] Buy milk", []string{"bullet", "task"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Numbered || !item.Task || item.Normalized != "Buy milk" {
		t.Fatalf("bullet task = %#v", item)
	}
	if string(raw[item.LineStart:item.LineEnd]) != "- [ ] Buy milk\n" {
		t.Fatalf("bullet task range = %q", raw[item.LineStart:item.LineEnd])
	}

	item, err = resolveMarkdownTask(raw, "note.md", "Buy milk", []string{"numbered"})
	if err != nil {
		t.Fatal(err)
	}
	if !item.Numbered || !item.Task {
		t.Fatalf("numbered task = %#v", item)
	}
	if string(raw[item.LineStart:item.LineEnd]) != "1. [ ] Buy milk\n" {
		t.Fatalf("numbered task range = %q", raw[item.LineStart:item.LineEnd])
	}

	if _, err := resolveMarkdownItem(raw, "note.md", "Plain item", []string{"task"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("absent valid combination err = %v", err)
	}
}

func TestResolveMarkdownItemRejectsComplexMatches(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "nested item",
			raw:  "- Parent\n  - Child\n",
			want: "Child",
		},
		{
			name: "multiline item",
			raw:  "- first line\n  second line\n",
			want: "first line",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolveMarkdownItem([]byte(tc.raw), "note.md", tc.want, nil); err == nil || !strings.Contains(err.Error(), "structurally complex") {
				t.Fatalf("complex item err = %v", err)
			}
		})
	}
}
