package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// refreshCmd lists containers once.
func (m *Model) refreshCmd() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		cs, err := client.List(context.Background(), true)
		if err != nil {
			return errMsg{err}
		}
		return containersMsg(cs)
	}
}

// pollCmd waits one interval, then fetches stats for all running containers.
func (m *Model) pollCmd() tea.Cmd {
	client := m.client
	interval := m.cfg.Interval()
	ids := []string{}
	for _, g := range m.groups {
		for _, c := range g.Containers {
			if c.Running() {
				ids = append(ids, c.ID)
			}
		}
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
		out := statsMsg{}
		for _, id := range ids {
			if s, err := client.Stats(context.Background(), id); err == nil {
				out[id] = s
			}
		}
		return out
	})
}

// listenEvents turns the docker event channel into tea.Msgs.
func (m *Model) listenEvents() tea.Cmd {
	evCh, _ := m.client.Events(context.Background())
	return func() tea.Msg {
		ev, ok := <-evCh
		if !ok {
			return nil
		}
		return eventMsg(ev)
	}
}

// confirm handling
func (m *Model) handleConfirmKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "y", "Y":
		action, id := m.confirmFor, m.confirmID
		m.confirm = false
		m.doAction(action, id)
		return m, m.refreshCmd()
	default: // anything else cancels
		m.confirm = false
	}
	return m, nil
}

func (m *Model) confirmBar() string {
	return lipglossDim(m, "Confirm "+m.confirmFor+" "+shortID(m.confirmID)+"?  [y/N]")
}

func (m *Model) doAction(action, id string) {
	ctx := context.Background()
	switch action {
	case "stop":
		_ = m.client.Stop(ctx, id)
	case "restart":
		_ = m.client.Restart(ctx, id)
	case "pause":
		_ = m.client.Pause(ctx, id)
	case "unpause":
		_ = m.client.Unpause(ctx, id)
	case "start":
		_ = m.client.Start(ctx, id)
	case "delete":
		_ = m.client.Remove(ctx, id)
	}
}

func lipglossDim(m *Model, s string) string {
	return lipgloss.NewStyle().Foreground(m.theme.Dim).Render(s)
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// loadLogsCmd stub (real body in Task 19)
func (m *Model) loadLogsCmd() tea.Cmd { return nil }

// view stubs (real bodies in Tasks 19-20)
func (m *Model) viewLogs() string     { return "logs" }
func (m *Model) viewSettings() string { return "settings" }
