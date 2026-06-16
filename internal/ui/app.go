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
	viewLogs
	viewSettings
)

type Model struct {
	client  docker.Client
	cfg     config.Config
	theme   Theme
	eventCh <-chan docker.Event

	groups   []model.Group
	stats    map[string]docker.Stats
	history  map[string]*model.RingBuffer
	order    []string // flattened visible container IDs, in display order
	selected int      // index into order

	collapsed    map[string]bool
	cols         int
	width        int
	height       int
	mode         viewMode
	focusInspect bool

	// scrollable grid state
	scrollY    int        // vertical scroll offset in body lines
	cardRects  []cardRect // hit-test rects from last render
	bodyTop    int        // screen line where body starts (= header height)
	gridAvailH int        // visible body height from last render
	gridBodyH  int        // total body height from last render

	// confirm modal
	confirm    bool
	confirmFor string // action name
	confirmID  string

	// focus view live logs
	focusLogs string

	// logs view
	logsVP        viewport.Model
	logsID        string
	logsReady     bool
	logsRaw       string // full unfiltered content
	logsQuery     string // current search query
	logsSearching bool   // true while typing a query
	logsFollow    bool   // true when follow mode is active

	// settings view
	settingsTab    settingsTab
	settingsSel    int
	settingsGroups []string
	lastContainers []docker.Container

	lastErr error
}

func New(client docker.Client, cfg config.Config) *Model {
	return &Model{
		client:    client,
		cfg:       cfg,
		theme:     ThemeByName(cfg.Theme),
		stats:     map[string]docker.Stats{},
		history:   map[string]*model.RingBuffer{},
		collapsed: cloneBoolMap(cfg.GroupCollapsed),
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
	return tea.Batch(m.refreshCmd(), m.pollCmd(), m.waitForEvent())
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

func (m *Model) recomputeLayout() { m.cols = Columns(m.width) }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.recomputeLayout()
		if m.logsReady {
			m.logsVP.Width = msg.Width
			m.logsVP.Height = msg.Height - 3
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
	case eventMsg:
		return m, tea.Batch(m.refreshCmd(), m.waitForEvent())
	case actionResultMsg:
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, m.refreshCmd()
	case logsMsg:
		m.handleLogsMsg(msg)
		return m, nil
	case logsTickMsg:
		if m.logsFollow && m.mode == viewLogs {
			return m, tea.Batch(m.loadLogsCmd(), m.logsTickCmd())
		}
		return m, nil
	case focusLogsMsg:
		if msg.id == m.SelectedID() {
			m.focusLogs = msg.content
		}
		return m, nil
	case focusTickMsg:
		if m.mode == viewFocus {
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
	if k.String() == "q" && !(m.mode == viewLogs && m.logsSearching) {
		return m, tea.Quit
	}
	if m.mode != viewGrid {
		if k.String() == "esc" {
			m.mode = viewGrid
			m.focusInspect = false
			return m, nil
		}
	}
	if m.mode == viewSettings {
		return m, m.updateSettings(k)
	}
	if m.mode == viewLogs {
		if m.logsSearching {
			switch k.Type {
			case tea.KeyEsc, tea.KeyEnter:
				m.logsSearching = false
			case tea.KeyBackspace, tea.KeyDelete:
				if len(m.logsQuery) > 0 {
					runes := []rune(m.logsQuery)
					m.logsQuery = string(runes[:len(runes)-1])
				}
			case tea.KeyRunes:
				m.logsQuery += k.String()
			}
			if m.logsReady {
				m.logsVP.SetContent(filterLines(m.logsRaw, m.logsQuery))
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
				m.logsVP.SetContent(filterLines(m.logsRaw, m.logsQuery))
			}
			return m, nil
		case "f":
			m.logsFollow = !m.logsFollow
			if m.logsFollow {
				return m, tea.Batch(m.loadLogsCmd(), m.logsTickCmd())
			}
			return m, nil
		}
		return m, m.updateLogs(k)
	}
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
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
	case "enter":
		m.focusLogs = ""
		m.mode = viewFocus
		return m, tea.Batch(m.loadFocusLogsCmd(), m.focusTickCmd())
	case "l":
		m.mode = viewLogs
		m.logsFollow = false
		m.logsSearching = false
		m.logsQuery = ""
		return m, m.loadLogsCmd()
	case "i":
		m.focusLogs = ""
		m.mode = viewFocus
		m.focusInspect = true
		return m, tea.Batch(m.loadFocusLogsCmd(), m.focusTickCmd())
	case "c":
		m.mode = viewSettings
		m.enterSettings()
	}
	return m, nil
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
		bodyY := msg.Y - m.bodyTop + m.scrollY
		id, ok := cardAt(m.cardRects, msg.X, bodyY)
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
	name := m.selectedGroupName()
	if name == "" {
		return
	}
	m.collapsed[name] = !m.collapsed[name]
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
