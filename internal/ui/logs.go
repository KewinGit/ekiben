package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// logsTickMsg is sent by logsTickCmd to drive the follow loop.
type logsTickMsg struct{}

// logsTickCmd schedules a logsTickMsg after 1 second.
func (m *Model) logsTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return logsTickMsg{}
	})
}

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

func (m *Model) handleLogsMsg(msg logsMsg) {
	if !m.logsReady {
		m.logsVP = viewport.New(m.width, m.height-3)
		m.logsReady = true
	}
	m.logsID = msg.id
	m.logsRaw = msg.content
	m.logsVP.SetContent(filterLines(m.logsRaw, m.logsQuery))
	m.logsVP.GotoBottom()
}

// updateLogs handles key input while in logs mode (scroll only).
func (m *Model) updateLogs(k tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.logsVP, cmd = m.logsVP.Update(k)
	return cmd
}

func (m *Model) viewLogs() string {
	t := m.theme
	head := lipgloss.NewStyle().Foreground(t.Header).Bold(true).
		Render(fmt.Sprintf("logs: %s", m.logsID))

	var statusParts []string
	if m.logsFollow {
		statusParts = append(statusParts, "follow ON")
	} else {
		statusParts = append(statusParts, "follow OFF")
	}
	if m.logsSearching {
		statusParts = append(statusParts, fmt.Sprintf("search: %s_", m.logsQuery))
	} else if m.logsQuery != "" {
		statusParts = append(statusParts, fmt.Sprintf("filter: %s", m.logsQuery))
	}
	status := ""
	if len(statusParts) > 0 {
		status = "  [" + strings.Join(statusParts, "  ") + "]"
	}

	help := lipgloss.NewStyle().Foreground(t.Dim).
		Render("[↑↓] scroll  [PgUp/PgDn] page  [g/G] top/bottom  [f] follow  [/] search  [esc] back" + status)

	content := ""
	if m.logsReady {
		content = m.logsVP.View()
	}
	return head + "\n" + content + "\n" + help
}
