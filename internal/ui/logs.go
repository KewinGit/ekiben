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
	wrapped := lipgloss.NewStyle().Width(w).Render(s)
	m.logsTotalLines = strings.Count(wrapped, "\n") + 1
	m.logsVP.SetContent(wrapped)
}

// scrollbar renders a vertical scroll indicator of the given height. total is the
// content line count, offset the top visible line, visible the viewport height.
func scrollbar(height, total, offset, visible int, t Theme) string {
	if height <= 0 {
		return ""
	}
	track := lipgloss.NewStyle().Foreground(t.Border)
	thumbS := lipgloss.NewStyle().Foreground(t.Accent)
	lines := make([]string, height)
	if total <= visible || total <= 0 {
		for i := range lines {
			lines[i] = track.Render("│")
		}
		return strings.Join(lines, "\n")
	}
	thumb := height * visible / total
	if thumb < 1 {
		thumb = 1
	}
	maxOffset := total - visible
	pos := 0
	if maxOffset > 0 {
		pos = (height - thumb) * offset / maxOffset
	}
	for i := 0; i < height; i++ {
		if i >= pos && i < pos+thumb {
			lines[i] = thumbS.Render("█")
		} else {
			lines[i] = track.Render("░")
		}
	}
	return strings.Join(lines, "\n")
}

// updateLogs forwards a key to the logs viewport (scrolling) and returns its cmd.
func (m *Model) updateLogs(k tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.logsVP, cmd = m.logsVP.Update(k)
	return cmd
}
