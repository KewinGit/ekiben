package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

// confirm helpers (real bodies in Task 17)
func (m *Model) handleConfirmKey(tea.KeyMsg) (tea.Model, tea.Cmd) { m.confirm = false; return m, nil }
func (m *Model) confirmBar() string                               { return "" }

// other-view stubs (real bodies in Tasks 18-20)
func (m *Model) viewFocus() string    { return "focus" }
func (m *Model) viewLogs() string     { return "logs" }
func (m *Model) viewSettings() string { return "settings" }
