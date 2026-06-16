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
