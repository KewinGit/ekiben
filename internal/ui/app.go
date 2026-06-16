package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// cardRect is a hit-test rectangle for a card in the grid body.
// y is the line index within the FULL (pre-scroll) body; x is the column offset.
type cardRect struct {
	id   string
	x, y int
	w, h int
}

// groupRect marks the body line of a group header (for click-to-collapse).
type groupRect struct {
	name string
	y    int
}

// cardAt returns the id of the rect that contains (x, y) or ("", false).
// x/y are in pre-scroll body coordinates.
func cardAt(rects []cardRect, x, y int) (string, bool) {
	for _, r := range rects {
		if x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h {
			return r.id, true
		}
	}
	return "", false
}

type retryMsg struct{}

type modalKind int

const (
	modalNone modalKind = iota
	modalConfirm
	modalBlocked
)

// modalState drives the centered confirmation / blocking popup.
type modalState struct {
	kind   modalKind
	title  string
	msg    string
	danger bool // red styling for dangerous actions
	steps  int  // confirmations required (1 = single, 2 = double)
	stage  int  // confirmations given so far
	action func() tea.Cmd
}

type viewMode int

const (
	viewGrid viewMode = iota
	viewFocus
	viewSettings
)

// homeTab identifies which top-level tab is active in the grid view.
type homeTab int

const (
	homeContainers homeTab = iota
	homeImages
	homeVolumes
	homeNetworks
	homeInfo
)

const homeTabCount = 5

type Model struct {
	client  docker.Client
	cfg     config.Config
	theme   Theme
	eventCh <-chan docker.Event

	groups     []model.Group
	stats      map[string]docker.Stats
	history    map[string]*model.RingBuffer
	memHistory map[string]*model.RingBuffer
	order      []string // flattened visible container IDs, in display order
	selected   int      // index into order

	collapsed map[string]bool
	cols      int
	width     int
	height    int
	mode      viewMode

	// scrollable grid state
	scrollY      int         // vertical scroll offset in body lines
	cardRects    []cardRect  // hit-test rects from last render
	groupRects   []groupRect // group-header hit-test rows from last render
	bodyTop      int         // screen line where body content starts
	bodyLeft     int         // screen column where body content starts
	gridAvailH   int         // visible body height from last render
	gridBodyH    int         // total body height from last render
	gridContentW int         // body content width (inside the panel border)

	// confirmation / blocking modal
	modal modalState

	// focus view live logs
	focusInit        bool // pending initial scroll-to-bottom after opening focus
	focusInspectInfo docker.InspectInfo

	// logs view (viewport reused by the focus detail view)
	logsVP         viewport.Model
	logsID         string
	logsReady      bool
	logsRaw        string // full unfiltered content
	logsQuery      string // current search query
	logsSearching  bool   // true while typing a query
	logsFollow     bool   // true when follow mode is active
	logsTotalLines int    // wrapped line count of current logs content

	// settings view
	settingsTab    settingsTab
	settingsSel    int
	settingsGroups []string
	lastContainers []docker.Container

	// home tab state
	homeTab  homeTab
	images   []docker.Image
	volumes  []docker.Volume
	networks []docker.Network
	imgSel   int // selected row in the Images tab
	netSel   int // selected row in the Networks tab
	volSel   int // selected row in the Volumes tab

	// hit-testing for the Networks/Volumes list (set during render)
	listTop     int // screen Y of the first data row
	listStart   int // index of the first visible data row
	listVisible int // number of visible data rows

	// streaming compose output pane (Containers tab)
	composeRunning   bool
	composeDone      bool // finished; pane stays until a key is pressed
	composeTitle     string
	composeLines     []string
	composeCh        chan composeEvent
	composePendingUp *composeRef // restart: run `up` after `down` completes

	lastErr error
	cfgPath string // where to persist config; empty disables saving (tests)
}

func New(client docker.Client, cfg config.Config) *Model {
	return &Model{
		client:     client,
		cfg:        cfg,
		cfgPath:    config.Path(),
		theme:      ThemeByName(cfg.Theme),
		stats:      map[string]docker.Stats{},
		history:    map[string]*model.RingBuffer{},
		memHistory: map[string]*model.RingBuffer{},
		collapsed:  cloneBoolMap(cfg.GroupCollapsed),
	}
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (m *Model) Init() tea.Cmd {
	m.eventCh, _ = m.client.Events(context.Background())
	return tea.Batch(m.refreshCmd(), m.pollCmd(), m.waitForEvent(), m.loadImagesCmd(), m.loadVolumesCmd(), m.loadNetworksCmd())
}

// applyContainers rebuilds groups + the flattened navigation order.
func (m *Model) applyContainers(cs []docker.Container) {
	m.lastContainers = cs
	if !m.cfg.ShowStopped {
		filtered := make([]docker.Container, 0, len(cs))
		for _, c := range cs {
			if c.Running() {
				filtered = append(filtered, c)
			}
		}
		cs = filtered
	}
	m.groups = model.GroupContainers(cs, m.cfg.GroupOrder)
	for i := range m.groups {
		model.SortContainers(m.groups[i].Containers, m.stats, m.cfg.SortWithinGroup)
	}
	m.rebuildOrder()
	m.lastErr = nil
}

func (m *Model) rebuildOrder() {
	prev := m.SelectedID()
	m.order = m.order[:0]
	for _, g := range m.groups {
		if m.collapsed[g.Name] {
			continue
		}
		for _, c := range g.Containers {
			m.order = append(m.order, c.ID)
		}
	}
	// keep selection on the same container if still present
	m.selected = 0
	for i, id := range m.order {
		if id == prev {
			m.selected = i
			break
		}
	}
	if m.selected >= len(m.order) {
		m.selected = max(0, len(m.order)-1)
	}
}

func (m *Model) SelectedID() string {
	if m.selected < 0 || m.selected >= len(m.order) {
		return ""
	}
	return m.order[m.selected]
}

func (m *Model) recomputeLayout() {
	m.gridContentW = m.width - 2
	if m.gridContentW < MinCardWidth {
		m.gridContentW = MinCardWidth
	}
	m.cols = Columns(m.gridContentW)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.recomputeLayout()
		if m.logsReady {
			m.logsVP.Width = m.focusLogsWidth()
			m.setLogsContent(filterLines(m.logsRaw, m.logsQuery)) // re-wrap to new width
		}
		// Clear the screen so a smaller frame doesn't leave stale rows behind.
		return m, tea.ClearScreen
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case statsMsg:
		m.ingestStats(msg)
		return m, m.pollCmd()
	case containersMsg:
		m.applyContainers(msg)
		return m, nil
	case imagesMsg:
		m.images = []docker.Image(msg)
		return m, nil
	case volumesMsg:
		m.volumes = []docker.Volume(msg)
		return m, nil
	case networksMsg:
		m.networks = []docker.Network(msg)
		return m, nil
	case eventMsg:
		return m, tea.Batch(m.refreshCmd(), m.waitForEvent())
	case actionResultMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		// reload everything: an action may have changed containers/images/volumes/networks
		return m, tea.Batch(m.refreshCmd(), m.loadImagesCmd(), m.loadVolumesCmd(), m.loadNetworksCmd())
	case execDoneMsg:
		// exec released the terminal: re-enable mouse, force a clean repaint, reload.
		return m, tea.Batch(tea.EnableMouseCellMotion, tea.ClearScreen,
			m.refreshCmd(), m.loadImagesCmd(), m.loadVolumesCmd(), m.loadNetworksCmd())
	case composeEvent:
		if msg.done {
			m.composeCh = nil
			if msg.err != nil {
				m.lastErr = msg.err
			}
			// restart: chain `up` after a successful `down`
			if m.composePendingUp != nil {
				g := *m.composePendingUp
				m.composePendingUp = nil
				return m, m.startComposeCmd("compose up: "+g.project, composeArgs(g, "up", "-d"))
			}
			// keep the pane open until the user presses a key
			m.composeDone = true
			return m, tea.Batch(m.refreshCmd(), m.loadImagesCmd(), m.loadVolumesCmd(), m.loadNetworksCmd())
		}
		m.composeLines = append(m.composeLines, msg.line)
		if len(m.composeLines) > 500 {
			m.composeLines = m.composeLines[len(m.composeLines)-500:]
		}
		return m, readComposeCmd(m.composeCh)
	case focusLogsMsg:
		if msg.id == m.SelectedID() {
			m.logsRaw = msg.content
			if !m.logsReady {
				m.logsVP = viewport.New(m.focusLogsWidth(), 10)
				m.logsVP.MouseWheelEnabled = true
				m.logsReady = true
			}
			m.logsVP.Width = m.focusLogsWidth()
			m.setLogsContent(filterLines(m.logsRaw, m.logsQuery))
			if m.focusInit {
				m.logsVP.GotoBottom()
				m.focusInit = false
			} else if m.logsFollow {
				m.logsVP.GotoBottom()
			}
		}
		return m, nil
	case focusInspectMsg:
		if msg.id == m.SelectedID() {
			m.focusInspectInfo = msg.info
		}
		return m, nil
	case focusTickMsg:
		if m.mode == viewFocus && m.logsFollow {
			return m, tea.Batch(m.loadFocusLogsCmd(), m.focusTickCmd())
		}
		return m, nil
	case errMsg:
		m.lastErr = msg.err
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return retryMsg{} })
	case retryMsg:
		return m, m.refreshCmd()
	}
	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal.kind != modalNone {
		return m.handleModalKey(k)
	}
	// Global quit: ctrl+c always; 'q' everywhere except while typing a log search.
	if k.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	// A finished compose pane stays until any key dismisses it.
	if m.composeDone {
		m.composeRunning = false
		m.composeDone = false
		m.composeLines = nil
		return m, nil
	}
	if k.String() == "q" && !(m.mode == viewFocus && m.logsSearching) {
		return m, tea.Quit
	}
	if m.mode == viewSettings {
		if k.String() == "esc" {
			m.mode = viewGrid
			return m, nil
		}
		return m, m.updateSettings(k)
	}
	if m.mode == viewFocus {
		return m.handleFocusKey(k)
	}
	// Home tab switching — active in all grid sub-tabs
	switch k.String() {
	case "tab":
		m.homeTab = (m.homeTab + 1) % homeTabCount
		return m, m.homeTabSwitchCmd()
	case "shift+tab":
		m.homeTab = (m.homeTab - 1 + homeTabCount) % homeTabCount
		return m, m.homeTabSwitchCmd()
	case "1":
		m.homeTab = homeContainers
		return m, m.homeTabSwitchCmd()
	case "2":
		m.homeTab = homeImages
		return m, m.homeTabSwitchCmd()
	case "3":
		m.homeTab = homeVolumes
		return m, m.homeTabSwitchCmd()
	case "4":
		m.homeTab = homeNetworks
		return m, m.homeTabSwitchCmd()
	case "5":
		m.homeTab = homeInfo
		return m, m.homeTabSwitchCmd()
	case "c":
		m.mode = viewSettings
		m.enterSettings()
		return m, nil
	}

	// Images / Networks / Volumes tabs: up/down move the list selection, d deletes.
	if sel, n := m.listSelPtr(); sel != nil {
		switch k.String() {
		case "up", "k":
			if *sel > 0 {
				*sel--
			}
		case "down", "j":
			if *sel < n-1 {
				*sel++
			}
		case "d":
			m.requestListDelete()
		}
		return m, nil
	}

	// Container-specific keys only apply on the Containers tab
	if m.homeTab != homeContainers {
		return m, nil
	}

	switch k.String() {
	case "right":
		m.move(1)
	case "left", "h":
		m.move(-1)
	case "down", "j":
		m.move(m.selectedGroupCols())
	case "up", "k":
		m.move(-m.selectedGroupCols())
	case " ", "space":
		m.toggleCollapse()
	case "s":
		return m, m.requestAction("stop")
	case "r":
		return m, m.requestAction("restart")
	case "p":
		return m, m.requestAction("pause")
	case "d":
		return m, m.requestAction("delete")
	case "a":
		return m, m.requestAction("start")
	case "u":
		return m, m.requestAction("unpause")
	case "e":
		if id := m.SelectedID(); id != "" {
			return m, m.execShellCmd(id)
		}
	case "S":
		if g, ok := m.selectedComposeRef(); ok {
			return m, m.runCompose(g, "up")
		}
	case "X":
		return m, m.requestCompose("down")
	case "R":
		return m, m.requestCompose("restart")
	case "enter", "l", "i":
		m.openFocus()
		return m, tea.Batch(m.loadFocusLogsCmd(), m.loadFocusInspectCmd())
	}
	return m, nil
}

// selectedComposeRef returns the compose project of the selected container.
func (m *Model) selectedComposeRef() (composeRef, bool) {
	c, ok := m.selectedContainer()
	if !ok || c.Project == "" {
		return composeRef{}, false
	}
	return composeRef{project: c.Project, workdir: c.ComposeWorkdir, files: c.ComposeFiles}, true
}

// runCompose starts a streamed `docker compose` action for a project.
func (m *Model) runCompose(g composeRef, action string) tea.Cmd {
	switch action {
	case "up":
		return m.startComposeCmd("compose up: "+g.project, composeArgs(g, "up", "-d"))
	case "down":
		return m.startComposeCmd("compose down: "+g.project, composeArgs(g, "down"))
	case "restart":
		gg := g
		m.composePendingUp = &gg
		return m.startComposeCmd("compose down: "+g.project, composeArgs(g, "down"))
	}
	return nil
}

// requestCompose runs a compose action (down / restart) on the selected project,
// gated by a danger confirm when confirm_destructive is on.
func (m *Model) requestCompose(action string) tea.Cmd {
	g, ok := m.selectedComposeRef()
	if !ok {
		return nil
	}
	if !m.cfg.ConfirmDestructive {
		return m.runCompose(g, action)
	}
	msg := g.project
	if action == "restart" {
		msg = g.project + "  (down + up)"
	}
	act := func() tea.Cmd { return m.runCompose(g, action) }
	m.modal = modalState{kind: modalConfirm, title: "compose " + action, msg: msg, danger: true, steps: 1, action: act}
	return nil
}

// openFocus enters the detail view for the selected container, resetting log state.
func (m *Model) openFocus() {
	m.mode = viewFocus
	m.logsFollow = false
	m.logsSearching = false
	m.logsQuery = ""
	m.focusInspectInfo = docker.InspectInfo{}
	m.focusInit = true
}

// handleFocusKey handles keys in the detail view (log scroll/follow/search + actions).
func (m *Model) handleFocusKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.logsSearching {
		switch k.Type {
		case tea.KeyEsc, tea.KeyEnter:
			m.logsSearching = false
		case tea.KeyBackspace, tea.KeyDelete:
			if len(m.logsQuery) > 0 {
				r := []rune(m.logsQuery)
				m.logsQuery = string(r[:len(r)-1])
			}
		case tea.KeyRunes:
			m.logsQuery += k.String()
		}
		if m.logsReady {
			m.setLogsContent(filterLines(m.logsRaw, m.logsQuery))
		}
		return m, nil
	}
	switch k.String() {
	case "esc":
		m.mode = viewGrid
		m.logsFollow = false
		m.logsSearching = false
		m.logsQuery = ""
		return m, nil
	case "/":
		m.logsSearching = true
		m.logsQuery = ""
		if m.logsReady {
			m.setLogsContent(filterLines(m.logsRaw, m.logsQuery))
		}
		return m, nil
	case "f":
		m.logsFollow = !m.logsFollow
		if m.logsFollow {
			return m, tea.Batch(m.loadFocusLogsCmd(), m.focusTickCmd())
		}
		return m, nil
	case "s":
		return m, m.requestAction("stop")
	case "r":
		return m, m.requestAction("restart")
	case "p":
		return m, m.requestAction("pause")
	case "a":
		return m, m.requestAction("start")
	case "u":
		return m, m.requestAction("unpause")
	case "d":
		return m, m.requestAction("delete")
	case "e":
		if id := m.SelectedID(); id != "" {
			return m, m.execShellCmd(id)
		}
	}
	return m, m.updateLogs(k)
}

// homeTabSwitchCmd returns a load cmd when switching to Images, Volumes, or Networks tabs.
func (m *Model) homeTabSwitchCmd() tea.Cmd {
	switch m.homeTab {
	case homeImages:
		return m.loadImagesCmd()
	case homeVolumes:
		return m.loadVolumesCmd()
	case homeNetworks:
		return m.loadNetworksCmd()
	}
	return nil
}

// selectedGroupCols returns the column count of the group holding the selection,
// used so vertical navigation steps by the right amount per (variable-width) group.
func (m *Model) selectedGroupCols() int {
	id := m.SelectedID()
	for _, g := range m.groups {
		for _, c := range g.Containers {
			if c.ID == id {
				cols, _ := m.groupLayout(g)
				return cols
			}
		}
	}
	if m.cols < 1 {
		return 1
	}
	return m.cols
}

func (m *Model) move(delta int) {
	if len(m.order) == 0 {
		return
	}
	n := m.selected + delta
	if n < 0 {
		n = 0
	}
	if n >= len(m.order) {
		n = len(m.order) - 1
	}
	m.selected = n
	m.ensureSelectedVisible()
}

// ensureSelectedVisible adjusts scrollY so the selected card's rect is visible.
func (m *Model) ensureSelectedVisible() {
	if m.gridAvailH <= 0 {
		return
	}
	id := m.SelectedID()
	for _, r := range m.cardRects {
		if r.id != id {
			continue
		}
		maxScroll := max(0, m.gridBodyH-m.gridAvailH)
		// scroll up if card top is above viewport
		if r.y < m.scrollY {
			m.scrollY = r.y
		}
		// scroll down if card bottom is below viewport
		if r.y+r.h > m.scrollY+m.gridAvailH {
			m.scrollY = r.y + r.h - m.gridAvailH
		}
		if m.scrollY < 0 {
			m.scrollY = 0
		}
		if m.scrollY > maxScroll {
			m.scrollY = maxScroll
		}
		return
	}
}

// listSelPtr returns a pointer to the selection index and the item count for the
// active list-style tab (Images/Networks/Volumes), or (nil, 0) otherwise.
func (m *Model) listSelPtr() (*int, int) {
	switch m.homeTab {
	case homeImages:
		return &m.imgSel, len(m.images)
	case homeNetworks:
		return &m.netSel, len(m.networks)
	case homeVolumes:
		return &m.volSel, len(m.volumes)
	}
	return nil, 0
}

// handleMouse handles mouse events (only when in grid view).
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// In the detail view, forward wheel events to the logs viewport for scrolling.
	if m.mode == viewFocus {
		if m.logsReady {
			var cmd tea.Cmd
			m.logsVP, cmd = m.logsVP.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	if m.mode != viewGrid {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if sel, _ := m.listSelPtr(); sel != nil {
			if *sel > 0 {
				*sel--
			}
			return m, nil
		}
		m.scrollY -= 3
		if m.scrollY < 0 {
			m.scrollY = 0
		}
	case tea.MouseButtonWheelDown:
		if sel, n := m.listSelPtr(); sel != nil {
			if *sel < n-1 {
				*sel++
			}
			return m, nil
		}
		maxScroll := max(0, m.gridBodyH-m.gridAvailH)
		m.scrollY += 3
		if m.scrollY > maxScroll {
			m.scrollY = maxScroll
		}
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// Tab bar is always the first row: a click there switches tab.
		if msg.Y == 0 {
			if t, ok := tabAt(msg.X); ok {
				m.homeTab = t
				return m, m.homeTabSwitchCmd()
			}
			return m, nil
		}
		// Images / Networks / Volumes: click a list row to select it.
		if sel, n := m.listSelPtr(); sel != nil {
			if row := msg.Y - m.listTop; row >= 0 && row < m.listVisible {
				if i := m.listStart + row; i < n {
					*sel = i
				}
			}
			return m, nil
		}
		// Card selection / group collapse only matter on the Containers tab.
		if m.homeTab != homeContainers {
			return m, nil
		}
		bodyX := msg.X - m.bodyLeft
		bodyY := msg.Y - m.bodyTop + m.scrollY
		// click on a group header row toggles its collapse
		for _, gr := range m.groupRects {
			if gr.y == bodyY {
				m.toggleGroupCollapsed(gr.name)
				return m, nil
			}
		}
		id, ok := cardAt(m.cardRects, bodyX, bodyY)
		if ok {
			for i, oid := range m.order {
				if oid == id {
					m.selected = i
					break
				}
			}
		}
	}
	return m, nil
}

// View is implemented in grid.go (and dispatches to other views later).
func (m *Model) View() string {
	if m.modal.kind != modalNone {
		return m.viewModal()
	}
	return m.viewCurrent()
}

// handleModalKey processes keys while a modal is open.
func (m *Model) handleModalKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modal.kind == modalBlocked {
		m.modal = modalState{} // any key dismisses
		return m, nil
	}
	// modalConfirm
	if s := k.String(); s == "y" || s == "Y" {
		m.modal.stage++
		if m.modal.stage >= m.modal.steps {
			act := m.modal.action
			m.modal = modalState{}
			if act != nil {
				return m, act()
			}
		}
		return m, nil
	}
	m.modal = modalState{} // any other key cancels
	return m, nil
}

// viewModal renders the centered confirmation / blocking popup.
func (m *Model) viewModal() string {
	t := m.theme
	border := t.Border
	titleStyle := lipgloss.NewStyle().Foreground(t.Header).Bold(true)
	if m.modal.danger {
		border = t.Problem
		titleStyle = lipgloss.NewStyle().Foreground(t.Problem).Bold(true)
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.modal.title) + "\n\n")
	b.WriteString(m.modal.msg + "\n\n")
	dim := lipgloss.NewStyle().Foreground(t.Dim)
	switch m.modal.kind {
	case modalBlocked:
		b.WriteString(dim.Render("[any key] close"))
	case modalConfirm:
		if m.modal.steps > 1 {
			rem := m.modal.steps - m.modal.stage
			b.WriteString(lipgloss.NewStyle().Foreground(t.Problem).Bold(true).
				Render(fmt.Sprintf("DANGER — press y %d more time(s); any other key cancels", rem)))
		} else {
			b.WriteString(dim.Render("[y] confirm    [n] cancel"))
		}
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(1, 3).Render(b.String())
	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) selectedGroupName() string {
	id := m.SelectedID()
	for _, g := range m.groups {
		for _, c := range g.Containers {
			if c.ID == id {
				return g.Name
			}
		}
	}
	return ""
}

func (m *Model) toggleCollapse() {
	m.toggleGroupCollapsed(m.selectedGroupName())
}

// toggleGroupCollapsed flips a group's collapse state, persists it to the config
// (so it's remembered next launch), and rebuilds the navigation order.
func (m *Model) toggleGroupCollapsed(name string) {
	if name == "" {
		return
	}
	m.collapsed[name] = !m.collapsed[name]
	m.cfg.GroupCollapsed = cloneBoolMap(m.collapsed)
	m.saveConfig()
	m.rebuildOrder()
}

// saveConfig persists the config to cfgPath. A blank cfgPath disables saving so
// tests never clobber the user's real ~/.config/ekiben/config.yml.
func (m *Model) saveConfig() {
	if m.cfgPath == "" {
		return
	}
	_ = m.cfg.Save(m.cfgPath)
}

func (m *Model) requestAction(action string) tea.Cmd {
	c, ok := m.selectedContainer()
	if !ok {
		return nil
	}
	if !m.cfg.ConfirmDestructive {
		return m.doActionCmd(action, c.ID)
	}
	danger := action == "delete"
	steps := 1
	if action == "delete" && c.Running() {
		steps = 2 // deleting a running container is doubly confirmed
	}
	id := c.ID
	m.modal = modalState{
		kind:   modalConfirm,
		title:  action + " container",
		msg:    c.Name,
		danger: danger,
		steps:  steps,
		action: func() tea.Cmd { return m.doActionCmd(action, id) },
	}
	return nil
}

// requestListDelete builds the delete modal for the active Images/Networks/Volumes tab.
func (m *Model) requestListDelete() {
	switch m.homeTab {
	case homeImages:
		if len(m.images) == 0 {
			return
		}
		img := m.images[m.imgSel]
		ref := img.Repo + ":" + img.Tag
		if deps := m.containersUsingImage(ref); len(deps) > 0 {
			m.modal = blockedModal("Cannot remove image", ref+"\n\nin use by "+strings.Join(deps, ", "))
			return
		}
		id := img.ID
		m.modal = modalState{kind: modalConfirm, title: "remove image", msg: ref, danger: true, steps: 1,
			action: func() tea.Cmd { return m.removeImageCmd(id) }}
	case homeVolumes:
		if len(m.volumes) == 0 {
			return
		}
		v := m.volumes[m.volSel]
		if deps := m.containersUsingVolume(v.Name); len(deps) > 0 {
			m.modal = blockedModal("Cannot remove volume", v.Name+"\n\nin use by "+strings.Join(deps, ", "))
			return
		}
		name := v.Name
		m.modal = modalState{kind: modalConfirm, title: "remove volume", msg: name, danger: true, steps: 1,
			action: func() tea.Cmd { return m.removeVolumeCmd(name) }}
	case homeNetworks:
		if len(m.networks) == 0 {
			return
		}
		net := m.networks[m.netSel]
		if net.Name == "bridge" || net.Name == "host" || net.Name == "none" {
			m.modal = blockedModal("Cannot remove network", net.Name+" is a predefined Docker network")
			return
		}
		if deps := m.containersInNetwork(net.Name); len(deps) > 0 {
			m.modal = blockedModal("Cannot remove network", net.Name+"\n\nin use by "+strings.Join(deps, ", "))
			return
		}
		id := net.ID
		m.modal = modalState{kind: modalConfirm, title: "remove network", msg: net.Name, danger: true, steps: 1,
			action: func() tea.Cmd { return m.removeNetworkCmd(id) }}
	}
}

func blockedModal(title, msg string) modalState {
	return modalState{kind: modalBlocked, title: title, msg: msg, danger: true}
}
