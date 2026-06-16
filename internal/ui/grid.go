package ui

import (
	"fmt"
	"strings"

	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// --- messages produced by background commands (filled in Task 16) ---

type statsMsg map[string]docker.Stats
type containersMsg []docker.Container
type eventMsg docker.Event
type errMsg struct{ err error }

func newHistory() *model.RingBuffer { return model.NewRingBuffer(30) }

func (m *Model) ingestStats(s statsMsg) {
	for id, st := range s {
		m.stats[id] = st
		if m.history[id] == nil {
			m.history[id] = newHistory()
		}
		m.history[id].Push(st.CPUPerc)
	}
}

func (m *Model) viewCurrent() string {
	switch m.mode {
	case viewFocus:
		return m.viewFocus()
	case viewLogs:
		return m.viewLogs()
	case viewSettings:
		return m.viewSettings()
	default:
		return m.viewGrid()
	}
}

func (m *Model) viewGrid() string {
	var b strings.Builder
	if m.lastErr != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Problem).
			Render("⚠ Docker error: "+m.lastErr.Error()+" — retrying…") + "\n\n")
	}
	b.WriteString(m.header() + "\n\n")
	cardW := CardWidth(m.width, m.cols)
	sel := m.SelectedID()

	for _, g := range m.groups {
		b.WriteString(m.groupHeader(g) + "\n")
		if m.collapsed[g.Name] {
			continue
		}
		var cards []string
		for _, c := range g.Containers {
			hist := []float64{}
			if rb := m.history[c.ID]; rb != nil {
				hist = rb.Values()
			}
			cards = append(cards, RenderCard(CardInput{
				Container: c, Stats: m.stats[c.ID], History: hist,
				Fields: m.cfg.CardFields, Width: cardW,
				Selected: c.ID == sel, Theme: m.theme,
			}))
		}
		b.WriteString(joinCards(cards, m.cols) + "\n")
	}
	if m.confirm {
		b.WriteString("\n" + m.confirmBar())
	} else {
		b.WriteString("\n" + m.actionBar())
	}
	return b.String()
}

// joinCards lays out card strings into rows of `cols`.
func joinCards(cards []string, cols int) string {
	if cols < 1 {
		cols = 1
	}
	var rows []string
	for i := 0; i < len(cards); i += cols {
		end := i + cols
		if end > len(cards) {
			end = len(cards)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, withGaps(cards[i:end])...))
	}
	return strings.Join(rows, "\n")
}

func withGaps(cards []string) []string {
	out := []string{}
	for i, c := range cards {
		if i > 0 {
			out = append(out, " ")
		}
		out = append(out, c)
	}
	return out
}

func (m *Model) header() string {
	total := 0
	healthy, down := 0, 0
	for _, g := range m.groups {
		for _, c := range g.Containers {
			total++
			if c.Health == docker.HealthHealthy {
				healthy++
			}
			if !c.Running() {
				down++
			}
		}
	}
	h := lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).Render("ekiben")
	return fmt.Sprintf("%s  %d containers · %d healthy · %d down", h, total, healthy, down)
}

func (m *Model) groupHeader(g groupLike) string {
	arrow := "▾"
	if m.collapsed[g.GroupName()] {
		arrow = "▸"
	}
	style := lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true)
	return style.Render(fmt.Sprintf("%s %s", arrow, g.GroupName())) +
		lipgloss.NewStyle().Foreground(m.theme.Dim).Render(fmt.Sprintf("  · %d", len(g.GetContainers())))
}

// groupLike lets header code accept model.Group without import cycle friction.
type groupLike interface {
	GroupName() string
	GetContainers() []docker.Container
}

func (m *Model) actionBar() string {
	return lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
		"[↑↓←→] navigate  [enter] focus  [l] logs  [s] stop  [r] restart  [p] pause  [i] inspect  [d] delete  [c] settings  [q] quit")
}
