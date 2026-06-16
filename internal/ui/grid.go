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
	// --- header ---
	header := m.header()
	var errBannerLines []string
	if m.lastErr != nil {
		errBannerLines = []string{
			lipgloss.NewStyle().Foreground(m.theme.Problem).
				Render("⚠ Docker error: " + m.lastErr.Error() + " — retrying…"),
		}
	}

	// footer
	var footer string
	if m.confirm {
		footer = m.confirmBar()
	} else {
		footer = m.actionBar()
	}

	// --- guard: no dimensions yet ---
	if m.height <= 0 || m.width <= 0 {
		var b strings.Builder
		b.WriteString(header + "\n")
		for _, l := range errBannerLines {
			b.WriteString(l + "\n")
		}
		b.WriteString(m.buildBodyFull() + "\n")
		b.WriteString(footer)
		return b.String()
	}

	// --- compute layout heights ---
	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	errH := len(errBannerLines)
	bodyTop := headerH + 1 // +1 for the "\n" separator after header
	// availH = total screen minus header row (with separator), error banner, footer
	availH := m.height - bodyTop - errH - footerH - 1 // -1 for separator before footer
	if availH < 1 {
		availH = 1
	}
	m.bodyTop = bodyTop + errH
	m.gridAvailH = availH

	// --- build full body lines + rects ---
	bodyLines, rects := m.buildBodyLines()
	m.cardRects = rects
	totalH := len(bodyLines)
	m.gridBodyH = totalH

	// --- clamp scroll ---
	maxScroll := max(0, totalH-availH)
	if m.scrollY < 0 {
		m.scrollY = 0
	}
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}

	// --- window ---
	end := m.scrollY + availH
	if end > totalH {
		end = totalH
	}
	windowLines := bodyLines[m.scrollY:end]

	// pad to availH to prevent ghosting
	for len(windowLines) < availH {
		windowLines = append(windowLines, "")
	}

	// --- assemble ---
	var b strings.Builder
	b.WriteString(header + "\n")
	for _, l := range errBannerLines {
		b.WriteString(l + "\n")
	}
	b.WriteString(strings.Join(windowLines, "\n"))
	b.WriteString("\n" + footer)
	return b.String()
}

// buildBodyFull builds the body as a single string (no windowing, for uninitialized state).
func (m *Model) buildBodyFull() string {
	lines, _ := m.buildBodyLines()
	return strings.Join(lines, "\n")
}

// buildBodyLines builds the grid body as a slice of lines and records card rects.
// y coordinates in rects are offsets within the full body slice.
func (m *Model) buildBodyLines() ([]string, []cardRect) {
	cardW := CardWidth(m.width, m.cols)
	sel := m.SelectedID()

	var lines []string
	var rects []cardRect

	for _, g := range m.groups {
		// group header line
		lines = append(lines, m.groupHeader(g))

		if m.collapsed[g.Name] {
			continue
		}

		// build rendered cards for this group
		var cards []string
		var ids []string
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
			ids = append(ids, c.ID)
		}

		// lay out in rows of m.cols
		cols := m.cols
		if cols < 1 {
			cols = 1
		}
		for rowStart := 0; rowStart < len(cards); rowStart += cols {
			rowEnd := rowStart + cols
			if rowEnd > len(cards) {
				rowEnd = len(cards)
			}
			rowCards := cards[rowStart:rowEnd]
			rowIDs := ids[rowStart:rowEnd]

			// rendered row (for display)
			rowStr := lipgloss.JoinHorizontal(lipgloss.Top, withGaps(rowCards)...)
			rowLines := strings.Split(rowStr, "\n")
			rowH := len(rowLines)

			// compute rects for each card in the row
			xCursor := 0
			for colIdx, card := range rowCards {
				cw := lipgloss.Width(card)
				rects = append(rects, cardRect{
					id: rowIDs[colIdx],
					x:  xCursor,
					y:  len(lines),
					w:  cw,
					h:  rowH,
				})
				xCursor += cw + CardGap // +1 for the gap " " inserted by withGaps
			}

			lines = append(lines, rowLines...)
		}
	}

	return lines, rects
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

const ekibenBanner = `        _    _ _
   ___ | | _(_) |__   ___ _ __
  / _ \| |/ / | '_ \ / _ \ '_ \
 |  __/|   <| | |_) |  __/ | | |
  \___||_|\_\_|_.__/ \___|_| |_|`

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
	w := m.width
	if w < 1 {
		w = 80
	}
	banner := lipgloss.PlaceHorizontal(w, lipgloss.Center,
		lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).Render(ekibenBanner))
	stats := lipgloss.PlaceHorizontal(w, lipgloss.Center,
		lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
			fmt.Sprintf("%d containers · %d healthy · %d down", total, healthy, down)))
	return banner + "\n" + stats
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
		"[↑↓←→] navigate  [enter] focus  [l] logs  [s] stop  [r] restart  [p] pause  [a] start  [u] unpause  [i] inspect  [d] delete  [c] settings  [q] quit")
}
