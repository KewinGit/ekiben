package ui

import (
	"strings"
	"testing"
)

func TestLogsViewShowsContent(t *testing.T) {
	m := newTestModel()
	m.mode = viewLogs
	m.Update(logsMsg{id: "1", content: "alpha\nbeta\n"})
	out := m.View()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("logs view missing content:\n%s", out)
	}
}

func TestFilterLines(t *testing.T) {
	content := "alpha\nBeta\ngamma\nALPHA beta\n"

	// empty query returns content unchanged
	got := filterLines(content, "")
	if got != content {
		t.Fatalf("empty query: expected content unchanged, got %q", got)
	}

	// non-empty query returns only matching lines, case-insensitive
	got = filterLines(content, "alpha")
	lines := strings.Split(got, "\n")
	if len(lines) != 2 || lines[0] != "alpha" || lines[1] != "ALPHA beta" {
		t.Fatalf("query 'alpha': expected [alpha, ALPHA beta], got %v", lines)
	}

	// query matching a different case
	got = filterLines(content, "BETA")
	lines = strings.Split(got, "\n")
	if len(lines) != 2 || lines[0] != "Beta" || lines[1] != "ALPHA beta" {
		t.Fatalf("query 'BETA': expected [Beta, ALPHA beta], got %v", lines)
	}

	// query with no match returns empty string
	got = filterLines(content, "zzzz")
	if got != "" {
		t.Fatalf("no-match query: expected empty string, got %q", got)
	}
}
