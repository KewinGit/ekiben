package ui

import (
	"fmt"
	"strings"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/KewinGit/ekiben/internal/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
		if m.memHistory[id] == nil {
			m.memHistory[id] = newHistory()
		}
		memPct := 0.0
		if st.MemLimit > 0 {
			memPct = float64(st.MemUsage) / float64(st.MemLimit) * 100
		}
		m.memHistory[id].Push(memPct)
	}
}

func (m *Model) viewCurrent() string {
	switch m.mode {
	case viewFocus:
		return m.viewFocus()
	case viewSettings:
		return m.viewSettings()
	default:
		return m.viewHome()
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

	// The grid lives inside a rounded panel border (1 line/col on each side).
	m.gridContentW = m.width - 2
	if m.gridContentW < MinCardWidth {
		m.gridContentW = MinCardWidth
	}
	// Screen layout: header(headerH) + errBanner(errH) + panel[ top(1) + content(availH) + bottom(1) ]
	//                + separator(1) + footer(footerH)
	availH := m.height - headerH - errH - footerH - 3
	if availH < 1 {
		availH = 1
	}
	// content first row sits below header, error banner, and the panel's top border
	m.bodyTop = headerH + errH + 1
	m.bodyLeft = 1 // panel left border
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
	for len(windowLines) < availH { // pad to avoid ghosting
		windowLines = append(windowLines, "")
	}

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Width(m.gridContentW).
		Render(strings.Join(windowLines, "\n"))

	// --- assemble ---
	var b strings.Builder
	b.WriteString(header + "\n")
	for _, l := range errBannerLines {
		b.WriteString(l + "\n")
	}
	b.WriteString(panel)
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
	sel := m.SelectedID()

	var lines []string
	var rects []cardRect
	m.groupRects = m.groupRects[:0]

	for _, g := range m.groups {
		// group header line (record its body row for click-to-collapse)
		m.groupRects = append(m.groupRects, groupRect{name: g.Name, y: len(lines)})
		lines = append(lines, m.groupHeader(g))

		if m.collapsed[g.Name] {
			continue
		}

		// per-group layout: width adapts so the longest name in THIS compose fits
		cols, cardW := m.groupLayout(g)

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

		// lay out in rows of cols (computed per group above)
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
	tab := m.tabBar()
	stats := lipgloss.PlaceHorizontal(w, lipgloss.Center,
		lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
			fmt.Sprintf("%d containers · %d healthy · %d down", total, healthy, down)))
	return tab + "\n" + stats
}

// homeTabNames are the labels of the top-level tabs (shared by render + hit-test).
var homeTabNames = []string{"Containers", "Images", "Volumes", "Networks", "Info"}

const tabBarLeading = 1 // leading space before the first tab label
const tabBarSep = 3     // width of the " │ " separator between labels

// tabAt maps a column x on the tab-bar row to a tab, if any.
func tabAt(x int) (homeTab, bool) {
	pos := tabBarLeading
	for i, name := range homeTabNames {
		w := lipgloss.Width(name)
		if x >= pos && x < pos+w {
			return homeTab(i), true
		}
		pos += w + tabBarSep
	}
	return 0, false
}

// tabBar renders the top-level home tab bar.
func (m *Model) tabBar() string {
	names := homeTabNames
	var b strings.Builder
	b.WriteString(" ")
	for i, name := range names {
		var s string
		if homeTab(i) == m.homeTab {
			s = lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).Render(name)
		} else {
			s = lipgloss.NewStyle().Foreground(m.theme.Dim).Render(name)
		}
		b.WriteString(s)
		if i < len(names)-1 {
			b.WriteString(lipgloss.NewStyle().Foreground(m.theme.Dim).Render(" │ "))
		}
	}
	return b.String()
}

// viewHome dispatches to the correct home tab view.
func (m *Model) viewHome() string {
	switch m.homeTab {
	case homeImages:
		return m.viewImages()
	case homeVolumes:
		return m.viewVolumes()
	case homeNetworks:
		return m.viewNetworks()
	case homeInfo:
		return m.viewInfo()
	default:
		return m.viewGrid()
	}
}

// viewImages renders the images listing panel.
func (m *Model) viewImages() string {
	w := m.width
	if w < 1 {
		w = 80
	}
	availH := m.height - lipgloss.Height(m.tabBar()) - 4 // 4 = border top+bottom + some margin
	if availH < 1 {
		availH = 1
	}

	var lines []string
	hdr := lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).Render(
		fmt.Sprintf("%-40s  %8s  %s", "REPOSITORY:TAG", "SIZE", "ID"))
	lines = append(lines, hdr)

	var totalSize int64
	for _, img := range m.images {
		totalSize += img.Size
		repoTag := img.Repo + ":" + img.Tag
		shortID := img.ID
		if len(shortID) > 19 {
			shortID = shortID[7:19] // strip sha256: + 12 chars
		}
		line := fmt.Sprintf("%-40s  %8s  %s",
			repoTag,
			HumanBytes(uint64(img.Size)),
			shortID,
		)
		lines = append(lines, line)
	}

	totalLine := lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
		fmt.Sprintf("TOTAL  %s", HumanBytes(uint64(totalSize))))

	truncated := 0
	// Reserve 1 line for total footer; availH-1 for content
	maxContent := availH - 1
	if maxContent < 1 {
		maxContent = 1
	}
	if len(lines) > maxContent {
		truncated = len(lines) - maxContent
		lines = lines[:maxContent]
	}
	if truncated > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
			fmt.Sprintf("… (+%d more)", truncated)))
	}
	lines = append(lines, totalLine)

	contentW := w - 4
	if contentW < 20 {
		contentW = 20
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Width(contentW).
		Render(strings.Join(lines, "\n"))

	return m.tabBar() + "\n" + panel
}

// viewVolumes renders the volumes listing panel.
func (m *Model) viewVolumes() string {
	w := m.width
	if w < 1 {
		w = 80
	}
	availH := m.height - lipgloss.Height(m.tabBar()) - 4
	if availH < 1 {
		availH = 1
	}

	var lines []string
	hdr := lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).Render(
		fmt.Sprintf("%-30s  %-10s  %8s  %s", "NAME", "DRIVER", "SIZE", "MOUNTPOINT"))
	lines = append(lines, hdr)

	mountW := w - 58 // leave room for name+driver+size columns + borders
	if mountW < 20 {
		mountW = 20
	}
	var totalSize int64
	for _, vol := range m.volumes {
		totalSize += vol.Size
		mp := ansi.Truncate(vol.Mountpoint, mountW, "…")
		line := fmt.Sprintf("%-30s  %-10s  %8s  %s",
			vol.Name, vol.Driver, HumanBytes(uint64(vol.Size)), mp)
		lines = append(lines, line)
	}

	totalLine := lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
		fmt.Sprintf("TOTAL  %s", HumanBytes(uint64(totalSize))))

	truncated := 0
	maxContent := availH - 1
	if maxContent < 1 {
		maxContent = 1
	}
	if len(lines) > maxContent {
		truncated = len(lines) - maxContent
		lines = lines[:maxContent]
	}
	if truncated > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
			fmt.Sprintf("… (+%d more)", truncated)))
	}
	lines = append(lines, totalLine)

	contentW := w - 4
	if contentW < 20 {
		contentW = 20
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Width(contentW).
		Render(strings.Join(lines, "\n"))

	return m.tabBar() + "\n" + panel
}

// viewNetworks renders the networks listing panel.
func (m *Model) viewNetworks() string {
	w := m.width
	if w < 1 {
		w = 80
	}
	availH := m.height - lipgloss.Height(m.tabBar()) - 4
	if availH < 1 {
		availH = 1
	}

	var lines []string
	hdr := lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).Render(
		fmt.Sprintf("%-30s  %-10s  %-10s  %s", "NAME", "DRIVER", "SCOPE", "ID"))
	lines = append(lines, hdr)

	for _, net := range m.networks {
		name := ansi.Truncate(net.Name, 30, "…")
		driver := ansi.Truncate(net.Driver, 10, "…")
		scope := ansi.Truncate(net.Scope, 10, "…")
		id := net.ID
		if len(id) > 12 {
			id = id[:12]
		}
		line := fmt.Sprintf("%-30s  %-10s  %-10s  %s", name, driver, scope, id)
		lines = append(lines, line)
	}

	truncated := 0
	if len(lines) > availH {
		truncated = len(lines) - availH
		lines = lines[:availH]
	}
	if truncated > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(m.theme.Dim).Render(
			fmt.Sprintf("… (+%d more)", truncated)))
	}

	contentW := w - 4
	if contentW < 20 {
		contentW = 20
	}
	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Border).
		Width(contentW).
		Render(strings.Join(lines, "\n"))

	return m.tabBar() + "\n" + panel
}

// viewInfo renders program info (moved from settings Info tab).
func (m *Model) viewInfo() string {
	t := m.theme
	lbl := lipgloss.NewStyle().Foreground(t.Label)
	dim := lipgloss.NewStyle().Foreground(t.Dim)
	accent := lipgloss.NewStyle().Foreground(t.Accent)

	total := 0
	for _, g := range m.groups {
		total += len(g.Containers)
	}

	banner := lipgloss.NewStyle().Foreground(t.Header).Bold(true).Render(ekibenBanner)

	var body strings.Builder
	body.WriteString(banner + "\n\n")
	body.WriteString(dim.Render("A terminal UI to monitor and manage Docker containers,") + "\n")
	body.WriteString(dim.Render("shown as cards grouped by Compose project.") + "\n\n")
	body.WriteString(lbl.Render("version  ") + version.String() + "\n")
	body.WriteString(lbl.Render("license  ") + "MIT" + "\n")
	body.WriteString(lbl.Render("author   ") + "Kevin Corso" + "\n")
	body.WriteString(lbl.Render("github   ") + accent.Render("https://github.com/KewinGit/ekiben") + "\n")
	body.WriteString(lbl.Render("config   ") + config.Path() + "\n\n")
	body.WriteString(lbl.Render("monitoring  ") +
		fmt.Sprintf("%d containers · %d images · %d volumes · %d networks", total, len(m.images), len(m.volumes), len(m.networks)) + "\n\n")
	body.WriteString(dim.Render("keys  tab / 1-5  switch tab     c  settings     q  quit") + "\n")
	body.WriteString(dim.Render("      ↑↓←→ navigate · click select · wheel scroll") + "\n")
	body.WriteString(dim.Render("      enter focus · l logs · s/r/p/a/u/d actions · space collapse"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(1, 3).
		Render(body.String())

	w := m.width
	h := m.height
	if w > 0 && h > 0 {
		tabH := lipgloss.Height(m.tabBar())
		remaining := h - tabH
		if remaining < 1 {
			remaining = 1
		}
		return m.tabBar() + "\n" + lipgloss.Place(w, remaining, lipgloss.Center, lipgloss.Center, box)
	}
	return m.tabBar() + "\n" + box
}

func (m *Model) groupHeader(g groupLike) string {
	name := g.GroupName()
	arrow := "▾"
	if m.collapsed[name] {
		arrow = "▸"
	}
	containers := g.GetContainers()
	title := lipgloss.NewStyle().Foreground(m.theme.Header).Bold(true).
		Render(fmt.Sprintf("%s %s", arrow, name))

	// Aggregate sums, but only for fields currently visible on the cards.
	var cpu float64
	var mem, rx, tx, pids uint64
	for _, c := range containers {
		s := m.stats[c.ID]
		cpu += s.CPUPerc
		mem += s.MemUsage
		rx += s.NetRx
		tx += s.NetTx
		pids += s.PIDs
	}
	parts := []string{fmt.Sprintf("· %d", len(containers))}
	if contains(m.cfg.CardFields, "cpu") {
		parts = append(parts, fmt.Sprintf("cpu %.1f%%", cpu))
	}
	if contains(m.cfg.CardFields, "mem") {
		parts = append(parts, fmt.Sprintf("mem %s", HumanBytes(mem)))
	}
	if contains(m.cfg.CardFields, "net") {
		parts = append(parts, fmt.Sprintf("net ↓%s ↑%s", HumanBytes(rx), HumanBytes(tx)))
	}
	if contains(m.cfg.CardFields, "pids") {
		parts = append(parts, fmt.Sprintf("pids %d", pids))
	}
	return title + lipgloss.NewStyle().Foreground(m.theme.Dim).Render("  "+strings.Join(parts, " · "))
}

// groupLayout computes, for a single compose group, how many columns fit and how
// wide each card is, sized so the longest container name in THIS group is readable.
func (m *Model) groupLayout(g model.Group) (cols, cardW int) {
	// Width = just enough to fit the longest name in THIS group (+ chrome).
	// We do NOT stretch cards to fill the row; the extra space is used by
	// packing MORE columns, not by widening each card.
	cardW = MinCardWidth
	for _, c := range g.Containers {
		// name + chrome: status dot (1) + space (1) + " ►" marker (2) + borders (2)
		if w := lipgloss.Width(c.Name) + 6; w > cardW {
			cardW = w
		}
	}
	avail := m.gridContentW
	if avail < MinCardWidth {
		avail = MinCardWidth
	}
	if cardW > avail {
		cardW = avail
	}
	cols = (avail + CardGap) / (cardW + CardGap)
	if cols < 1 {
		cols = 1
	}
	return cols, cardW
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
