package etch

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpTopicsSnapshotSmoke(t *testing.T) {
	for _, topic := range []string{"", "model", "selectors", "values", "plans", "security", "conflicts", "table", "csv"} {
		var out bytes.Buffer
		if err := printHelp(&out, topic); err != nil {
			t.Fatalf("help %q: %v", topic, err)
		}
		if out.Len() == 0 {
			t.Fatalf("help %q produced no output", topic)
		}
	}
}

func TestShortHelpMentionsCoreFlags(t *testing.T) {
	for _, want := range []string{"--plan", "--dry-run", "--no-checkout", "--untracked", "--allow-empty"} {
		if !strings.Contains(shortHelp, want) {
			t.Fatalf("short help missing %s", want)
		}
	}
}
