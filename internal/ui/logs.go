package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) handleLogsMsg(msg logsMsg) {
	if !m.logsReady {
		m.logsVP = viewport.New(m.width, m.height-3)
		m.logsReady = true
	}
	m.logsID = msg.id
	m.logsVP.SetContent(msg.content)
	m.logsVP.GotoBottom()
}

// updateLogs handles key input while in logs mode.
func (m *Model) updateLogs(k tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	m.logsVP, cmd = m.logsVP.Update(k)
	return cmd
}

func (m *Model) viewLogs() string {
	t := m.theme
	head := lipgloss.NewStyle().Foreground(t.Header).Bold(true).
		Render(fmt.Sprintf("logs: %s", m.logsID))
	help := lipgloss.NewStyle().Foreground(t.Dim).
		Render("[↑↓] scroll  [PgUp/PgDn] page  [g/G] top/bottom  [esc] back")
	content := ""
	if m.logsReady {
		content = m.logsVP.View()
	}
	return head + "\n" + content + "\n" + help
}
