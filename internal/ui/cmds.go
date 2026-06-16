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

// waitForEvent reads the next event from the already-open event channel.
// It must NOT call m.client.Events again — the channel is opened once in Init.
func (m *Model) waitForEvent() tea.Cmd {
	ch := m.eventCh
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(ev)
	}
}

// actionResultMsg carries the result of an async container action.
type actionResultMsg struct{ err error }

// doActionCmd runs a container action asynchronously and returns actionResultMsg.
func (m *Model) doActionCmd(action, id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		var err error
		ctx := context.Background()
		switch action {
		case "stop":
			err = client.Stop(ctx, id)
		case "restart":
			err = client.Restart(ctx, id)
		case "pause":
			err = client.Pause(ctx, id)
		case "unpause":
			err = client.Unpause(ctx, id)
		case "start":
			err = client.Start(ctx, id)
		case "delete":
			err = client.Remove(ctx, id)
		}
		return actionResultMsg{err}
	}
}

// confirm handling
func (m *Model) handleConfirmKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "y", "Y":
		action, id := m.confirmFor, m.confirmID
		m.confirm = false
		return m, m.doActionCmd(action, id)
	default: // anything else cancels
		m.confirm = false
	}
	return m, nil
}

func (m *Model) confirmBar() string {
	return lipglossDim(m, "Confirm "+m.confirmFor+" "+shortID(m.confirmID)+"?  [y/N]")
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

type logsMsg struct {
	id      string
	content string
}

func (m *Model) loadLogsCmd() tea.Cmd {
	client := m.client
	id := m.SelectedID()
	return func() tea.Msg {
		rc, err := client.Logs(context.Background(), id, false, 1000)
		if err != nil {
			return errMsg{err}
		}
		defer rc.Close()
		b, _ := readAllDemux(rc)
		return logsMsg{id: id, content: string(b)}
	}
}
