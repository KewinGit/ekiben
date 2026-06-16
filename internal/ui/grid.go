package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/KewinGit/ekiben/internal/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// deleteStatusCol renders a fixed-width STATUS column describing deletability.
func deleteStatusCol(deps int, locked bool, t Theme) string {
	txt, col := "", t.Dim
	switch {
	case locked:
		txt, col = "locked", t.Dim
	case deps == 0:
		txt, col = "safe delete", t.Healthy
	default:
		txt, col = fmt.Sprintf("in use (%d)", deps), t.Warn
	}
	return lipgloss.NewStyle().Foreground(col).Render(fmt.Sprintf("%-12s", txt))
}

// windowSlice returns [start,end) of length<=height centered on sel.
func windowSlice(n, sel, height int) (int, int) {
	if height < 1 {
		height = 1
	}
	if n <= height {
		return 0, n
	}
	start := sel - height/2
	if start < 0 {
		start = 0
	}
	if start > n-height {
		start = n - height
	}
	return start, start + height
}

// containersInNetwork returns the names of containers attached to a network.
func (m *Model) containersInNetwork(name string) []string {
	var out []string
	for _, c := range m.lastContainers {
		for _, n := range c.Networks {
			if n == name {
				out = append(out, c.Name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// containersUsingVolume returns the names of containers mounting a volume.
func (m *Model) containersUsingVolume(name string) []string {
	var out []string
	for _, c := range m.lastContainers {
		for _, mt := range c.Mounts {
			if mt == name {
				out = append(out, c.Name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

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
	footer := m.actionBar()

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

// hyperlink wraps text in an OSC 8 terminal hyperlink so supporting terminals
// render it clickable (usually ctrl/cmd+click while mouse reporting is on).
func hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
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

// containersUsingImage returns names of containers created from the image ref.
func (m *Model) containersUsingImage(ref string) []string {
	var out []string
	for _, c := range m.lastContainers {
		if c.Image == ref {
			out = append(out, c.Name)
		}
	}
	sort.Strings(out)
	return out
}

// viewImages renders a selectable image list + the containers created from it.
func (m *Model) viewImages() string {
	t := m.theme
	w := m.width
	if w < 1 {
		w = 80
	}
	tab := m.tabBar()
	avail := m.height - lipgloss.Height(tab) - 1
	if avail < 8 {
		avail = 8
	}
	listH := (avail-1)/2 + 1
	detailH := avail - 1 - listH // -1 reserves a hint line below the list

	if m.imgSel >= len(m.images) {
		m.imgSel = max(0, len(m.images)-1)
	}
	bold := lipgloss.NewStyle().Foreground(t.Header).Bold(true)
	dim := lipgloss.NewStyle().Foreground(t.Dim)
	accent := lipgloss.NewStyle().Foreground(t.Accent)

	var totalSize int64
	for _, img := range m.images {
		totalSize += img.Size
	}
	rows := []string{bold.Render(fmt.Sprintf("  %-40s %8s %-14s %s", "REPOSITORY:TAG", "SIZE", "ID", "STATUS")) +
		dim.Render(fmt.Sprintf("   (%d, %s)", len(m.images), HumanBytes(uint64(totalSize))))}
	innerH := listH - 3
	start, end := windowSlice(len(m.images), m.imgSel, innerH)
	m.listTop = lipgloss.Height(tab) + 2
	m.listStart = start
	m.listVisible = end - start
	for i := start; i < end; i++ {
		img := m.images[i]
		cursor := "  "
		if i == m.imgSel {
			cursor = accent.Render("► ")
		}
		id := img.ID
		if len(id) > 19 {
			id = id[7:19]
		}
		deps := len(m.containersUsingImage(img.Repo + ":" + img.Tag))
		rows = append(rows, cursor+fmt.Sprintf("%-40s %8s %-14s ",
			ansi.Truncate(img.Repo+":"+img.Tag, 40, "…"), HumanBytes(uint64(img.Size)), id)+deleteStatusCol(deps, false, t))
	}
	listPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).
		Width(w - 2).Height(listH - 2).Render(strings.Join(rows, "\n"))

	var det []string
	if len(m.images) > 0 {
		img := m.images[m.imgSel]
		ref := img.Repo + ":" + img.Tag
		det = append(det, bold.Render("containers from ")+accent.Render(ref))
		cs := m.containersUsingImage(ref)
		if len(cs) == 0 {
			det = append(det, dim.Render("  (none)"))
		} else {
			for _, n := range cs {
				det = append(det, "  "+n)
			}
		}
	}
	detailPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).
		Width(w - 2).Height(detailH - 2).Render(strings.Join(det, "\n"))

	hint := dim.Render("  ↑↓/click select · d delete · tab/1-5 switch tab")
	return tab + "\n" + listPanel + "\n" + hint + "\n" + detailPanel
}

// viewVolumes renders the volumes listing panel.
func (m *Model) viewVolumes() string {
	t := m.theme
	w := m.width
	if w < 1 {
		w = 80
	}
	tab := m.tabBar()
	avail := m.height - lipgloss.Height(tab) - 1
	if avail < 8 {
		avail = 8
	}
	listH := (avail-1)/2 + 1
	detailH := avail - 1 - listH

	if m.volSel >= len(m.volumes) {
		m.volSel = max(0, len(m.volumes)-1)
	}
	bold := lipgloss.NewStyle().Foreground(t.Header).Bold(true)
	dim := lipgloss.NewStyle().Foreground(t.Dim)
	accent := lipgloss.NewStyle().Foreground(t.Accent)

	// list panel
	var totalSize int64
	for _, v := range m.volumes {
		totalSize += v.Size
	}
	rows := []string{bold.Render(fmt.Sprintf("  %-30s %-8s %8s  %s", "NAME", "DRIVER", "SIZE", "STATUS")) +
		dim.Render(fmt.Sprintf("   (%d, %s)", len(m.volumes), HumanBytes(uint64(totalSize))))}
	innerH := listH - 3
	start, end := windowSlice(len(m.volumes), m.volSel, innerH)
	m.listTop = lipgloss.Height(tab) + 2 // tab + panel top border + header row
	m.listStart = start
	m.listVisible = end - start
	for i := start; i < end; i++ {
		v := m.volumes[i]
		cursor := "  "
		if i == m.volSel {
			cursor = accent.Render("► ")
		}
		deps := len(m.containersUsingVolume(v.Name))
		rows = append(rows, cursor+fmt.Sprintf("%-30s %-8s %8s  ",
			ansi.Truncate(v.Name, 30, "…"), ansi.Truncate(v.Driver, 8, "…"), HumanBytes(uint64(v.Size)))+deleteStatusCol(deps, false, t))
	}
	listPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).
		Width(w - 2).Height(listH - 2).Render(strings.Join(rows, "\n"))

	// detail panel: containers using the selected volume
	var det []string
	if len(m.volumes) > 0 {
		v := m.volumes[m.volSel]
		det = append(det, bold.Render("containers using ")+accent.Render(v.Name))
		det = append(det, dim.Render(ansi.Truncate(v.Mountpoint, w-6, "…")))
		cs := m.containersUsingVolume(v.Name)
		if len(cs) == 0 {
			det = append(det, dim.Render("  (none)"))
		} else {
			for _, n := range cs {
				det = append(det, "  "+n)
			}
		}
	}
	detailPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).
		Width(w - 2).Height(detailH - 2).Render(strings.Join(det, "\n"))

	hint := dim.Render("  ↑↓/click select · d delete · tab/1-5 switch tab")
	return tab + "\n" + listPanel + "\n" + hint + "\n" + detailPanel
}

// viewNetworks renders a selectable network list + the containers in the selection.
func (m *Model) viewNetworks() string {
	t := m.theme
	w := m.width
	if w < 1 {
		w = 80
	}
	tab := m.tabBar()
	avail := m.height - lipgloss.Height(tab) - 1
	if avail < 8 {
		avail = 8
	}
	listH := (avail-1)/2 + 1
	detailH := avail - 1 - listH

	if m.netSel >= len(m.networks) {
		m.netSel = max(0, len(m.networks)-1)
	}
	bold := lipgloss.NewStyle().Foreground(t.Header).Bold(true)
	dim := lipgloss.NewStyle().Foreground(t.Dim)
	accent := lipgloss.NewStyle().Foreground(t.Accent)

	rows := []string{bold.Render(fmt.Sprintf("  %-24s %-9s %-6s %-14s %s", "NAME", "DRIVER", "SCOPE", "ID", "STATUS")) +
		dim.Render(fmt.Sprintf("   (%d)", len(m.networks)))}
	innerH := listH - 3
	start, end := windowSlice(len(m.networks), m.netSel, innerH)
	m.listTop = lipgloss.Height(tab) + 2 // tab + panel top border + header row
	m.listStart = start
	m.listVisible = end - start
	for i := start; i < end; i++ {
		net := m.networks[i]
		cursor := "  "
		if i == m.netSel {
			cursor = accent.Render("► ")
		}
		id := net.ID
		if len(id) > 12 {
			id = id[:12]
		}
		locked := net.Name == "bridge" || net.Name == "host" || net.Name == "none"
		deps := len(m.containersInNetwork(net.Name))
		rows = append(rows, cursor+fmt.Sprintf("%-24s %-9s %-6s %-14s ",
			ansi.Truncate(net.Name, 24, "…"), ansi.Truncate(net.Driver, 9, "…"), ansi.Truncate(net.Scope, 6, "…"), id)+deleteStatusCol(deps, locked, t))
	}
	listPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).
		Width(w - 2).Height(listH - 2).Render(strings.Join(rows, "\n"))

	var det []string
	if len(m.networks) > 0 {
		net := m.networks[m.netSel]
		det = append(det, bold.Render("containers in ")+accent.Render(net.Name))
		cs := m.containersInNetwork(net.Name)
		if len(cs) == 0 {
			det = append(det, dim.Render("  (none)"))
		} else {
			for _, n := range cs {
				det = append(det, "  "+n)
			}
		}
	}
	detailPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.Border).
		Width(w - 2).Height(detailH - 2).Render(strings.Join(det, "\n"))

	hint := dim.Render("  ↑↓/click select · d delete · tab/1-5 switch tab")
	return tab + "\n" + listPanel + "\n" + hint + "\n" + detailPanel
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
	ghURL := "https://github.com/KewinGit/ekiben"
	body.WriteString(lbl.Render("github   ") + hyperlink(ghURL, accent.Underline(true).Render(ghURL)) + "\n")
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
	hints := []string{
		"↑↓←→ navigate", "enter focus", "l logs", "s stop", "r restart", "p pause",
		"a start", "u unpause", "i inspect", "d delete", "c settings", "q quit",
	}
	return wrapHints(hints, m.width, m.theme.Dim)
}

// wrapHints joins hint tokens with " · ", wrapping onto extra lines only when
// the line would exceed width.
func wrapHints(tokens []string, width int, color lipgloss.Color) string {
	const sep = " · "
	var lines []string
	cur := ""
	for _, tok := range tokens {
		cand := tok
		if cur != "" {
			cand = cur + sep + tok
		}
		if width > 0 && lipgloss.Width(cand) > width && cur != "" {
			lines = append(lines, cur)
			cur = tok
		} else {
			cur = cand
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lipgloss.NewStyle().Foreground(color).Render(strings.Join(lines, "\n"))
}
