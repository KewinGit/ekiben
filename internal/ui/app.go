package ui

import (
	"context"
	"time"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

	collapsed    map[string]bool
	cols         int
	width        int
	height       int
	mode         viewMode
	focusInspect bool

	// scrollable grid state
	scrollY      int         // vertical scroll offset in body lines
	cardRects    []cardRect  // hit-test rects from last render
	groupRects   []groupRect // group-header hit-test rows from last render
	bodyTop      int         // screen line where body content starts
	bodyLeft     int         // screen column where body content starts
	gridAvailH   int         // visible body height from last render
	gridBodyH    int         // total body height from last render
	gridContentW int         // body content width (inside the panel border)

	// confirm modal
	confirm    bool
	confirmFor string // action name
	confirmID  string

	// focus view live logs
	focusInit bool // pending initial scroll-to-bottom after opening focus

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

	lastErr error
}

func New(client docker.Client, cfg config.Config) *Model {
	return &Model{
		client:     client,
		cfg:        cfg,
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
		return m, m.refreshCmd()
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
	if m.confirm {
		return m.handleConfirmKey(k)
	}
	// Global quit: ctrl+c always; 'q' everywhere except while typing a log search.
	if k.Type == tea.KeyCtrlC {
		return m, tea.Quit
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
	case "enter", "l":
		m.openFocus()
		return m, m.loadFocusLogsCmd()
	case "i":
		m.openFocus()
		m.focusInspect = true
		return m, m.loadFocusLogsCmd()
	}
	return m, nil
}

// openFocus enters the detail view for the selected container, resetting log state.
func (m *Model) openFocus() {
	m.mode = viewFocus
	m.logsFollow = false
	m.logsSearching = false
	m.logsQuery = ""
	m.focusInspect = false
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
		m.focusInspect = false
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
		m.scrollY -= 3
		if m.scrollY < 0 {
			m.scrollY = 0
		}
	case tea.MouseButtonWheelDown:
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
func (m *Model) View() string { return m.viewCurrent() }

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
	_ = m.cfg.Save(config.Path())
	m.rebuildOrder()
}

func (m *Model) requestAction(action string) tea.Cmd {
	id := m.SelectedID()
	if id == "" {
		return nil
	}
	if m.cfg.ConfirmDestructive {
		m.confirm = true
		m.confirmFor = action
		m.confirmID = id
		return nil
	}
	return m.doActionCmd(action, id)
}
