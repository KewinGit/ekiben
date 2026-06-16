package ui

import (
	"sort"

	"github.com/KewinGit/ekiben/internal/config"
	"github.com/KewinGit/ekiben/internal/docker"
	"github.com/KewinGit/ekiben/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

type viewMode int

const (
	viewGrid viewMode = iota
	viewFocus
	viewLogs
	viewSettings
)

type Model struct {
	client docker.Client
	cfg    config.Config
	theme  Theme

	groups   []model.Group
	stats    map[string]docker.Stats
	history  map[string]*model.RingBuffer
	order    []string // flattened visible container IDs, in display order
	selected int      // index into order

	collapsed map[string]bool
	cols      int
	width     int
	height    int
	mode      viewMode

	// confirm modal
	confirm    bool
	confirmFor string // action name
	confirmID  string
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
	return tea.Batch(m.refreshCmd(), m.pollCmd(), m.listenEvents())
}

// applyContainers rebuilds groups + the flattened navigation order.
func (m *Model) applyContainers(cs []docker.Container) {
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
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case statsMsg:
		m.ingestStats(msg)
		return m, m.pollCmd()
	case containersMsg:
		m.applyContainers(msg)
		return m, nil
	case eventMsg:
		return m, tea.Batch(m.refreshCmd(), m.listenEvents())
	}
	return m, nil
}

func (m *Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirm {
		return m.handleConfirmKey(k)
	}
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "right", "l":
		m.move(1)
	case "left", "h":
		m.move(-1)
	case "down", "j":
		m.move(m.cols)
	case "up", "k":
		m.move(-m.cols)
	}
	return m, nil
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
}

// View is implemented in grid.go (and dispatches to other views later).
func (m *Model) View() string { return m.viewCurrent() }

// sortedGroupNames is used by settings later.
func (m *Model) sortedGroupNames() []string {
	out := []string{}
	for _, g := range m.groups {
		out = append(out, g.Name)
	}
	sort.Strings(out)
	return out
}
