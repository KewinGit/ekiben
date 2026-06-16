package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// filterLines returns only the lines in content that contain query
// (case-insensitive). If query is empty, content is returned unchanged.
func filterLines(content, query string) string {
	if query == "" {
		return content
	}
	lq := strings.ToLower(query)
	var out []string
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(strings.ToLower(line), lq) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// setLogsContent word-wraps the content to the viewport width so long log lines
// always wrap instead of being cut off, then sets it on the viewport.
func (m *Model) setLogsContent(s string) {
	w := m.logsVP.Width
	if w < 1 {
		if w = m.width; w < 1 {
			w = 80
		}
	}
	m.logsVP.SetContent(lipgloss.NewStyle().Width(w).Render(s))
}

// updateLogs forwards a key to the logs viewport (scrolling) and returns its cmd.
func (m *Model) updateLogs(k tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.logsVP, cmd = m.logsVP.Update(k)
	return cmd
}
