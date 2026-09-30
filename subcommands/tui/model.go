package tui

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/PlakarKorp/kloset/objects"
	"github.com/PlakarKorp/kloset/repository"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	viewDashboard = iota
	viewSnapshots
	viewBrowser
	viewFile
	snapshotsPerPage = 50
	maxFileBytes     = 1 << 20
)

type dashboardMsg struct {
	data *DashboardData
	err  error
}

type snapshotsMsg struct {
	items []SnapshotInfo
	err   error
}

type snapshotFilter struct {
	perimeter string
	tag       string
}

type entriesMsg struct {
	items []EntryInfo
	err   error
}

type fileMsg struct {
	info    *EntryInfo
	content string
	warning string
	err     error
}

type tuiModel struct {
	repo          *repository.Repository
	noHighlight   bool
	view          int
	width         int
	height        int
	loading       bool
	err           error
	dashboard     *DashboardData
	allSnapshots  []SnapshotInfo
	snapshots     []SnapshotInfo
	filter        snapshotFilter
	filterOptions [2][]string
	filterInputs  [2]int
	filterField   int
	editingFilter bool
	page          int
	cursor        int
	snapshotID    objects.MAC
	rootPath      string
	dirPath       string
	entries       []EntryInfo
	fileInfo      *EntryInfo
	fileContent   string
	fileWarning   string
	fileOffset    int
}

func newModel(repo *repository.Repository, noHighlight bool) *tuiModel {
	return &tuiModel{repo: repo, noHighlight: noHighlight, view: viewDashboard}
}

func (m *tuiModel) Init() tea.Cmd {
	if m.view == viewBrowser {
		return m.loadEntries()
	}
	return m.loadDashboard()
}

func (m *tuiModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case dashboardMsg:
		m.loading = false
		m.dashboard, m.err = msg.data, msg.err
	case snapshotsMsg:
		m.loading = false
		m.allSnapshots, m.err = msg.items, msg.err
		m.filterOptions = snapshotFilterOptions(m.allSnapshots)
		m.applySnapshotFilter()
	case entriesMsg:
		m.loading = false
		m.entries, m.err = msg.items, msg.err
		m.cursor = 0
	case fileMsg:
		m.loading = false
		m.fileInfo, m.fileContent, m.fileWarning, m.err = msg.info, msg.content, msg.warning, msg.err
		m.fileOffset = 0
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m *tuiModel) updateKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.editingFilter {
		return m.updateFilterKey(key)
	}
	if key.String() == "q" {
		return m, tea.Quit
	}
	if m.loading {
		return m, nil
	}
	if m.err != nil && key.String() == "r" {
		m.err = nil
		return m, m.refresh()
	}

	switch m.view {
	case viewDashboard:
		switch key.String() {
		case "s", "enter":
			m.view = viewSnapshots
			return m, m.loadSnapshots()
		case "r":
			return m, m.loadDashboard()
		}
	case viewSnapshots:
		switch key.String() {
		case "f":
			for index, value := range []string{m.filter.perimeter, m.filter.tag} {
				m.filterInputs[index] = slices.Index(m.filterChoices(index), value)
			}
			m.editingFilter = true
			m.filterField = 0
			return m, nil
		case "esc", "backspace", "h":
			m.view = viewDashboard
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else if m.page > 0 {
				m.page--
				m.cursor = min(snapshotsPerPage-1, m.snapshotPageCount()-1)
			}
		case "down", "j":
			if m.cursor+1 < m.snapshotPageCount() {
				m.cursor++
			} else if (m.page+1)*snapshotsPerPage < len(m.snapshots) {
				m.page++
				m.cursor = 0
			}
		case "left", "pgup":
			if m.page > 0 {
				m.page--
				m.cursor = 0
			}
		case "right", "pgdown":
			if (m.page+1)*snapshotsPerPage < len(m.snapshots) {
				m.page++
				m.cursor = 0
			}
		case "enter":
			if info, ok := m.selectedSnapshot(); ok {
				m.snapshotID = info.ID
				m.rootPath = info.Importer
				m.dirPath = info.Importer
				m.view = viewBrowser
				return m, m.loadEntries()
			}
		case "r":
			return m, m.loadSnapshots()
		}
	case viewBrowser:
		switch key.String() {
		case "esc":
			m.view = viewSnapshots
			return m, nil
		case "backspace", "h":
			if path.Clean(m.dirPath) == path.Clean(m.rootPath) {
				m.view = viewSnapshots
				return m, nil
			}
			parent := path.Dir(m.dirPath)
			if !withinRoot(parent, m.rootPath) {
				parent = m.rootPath
			}
			m.dirPath = parent
			return m, m.loadEntries()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor+1 < len(m.entries) {
				m.cursor++
			}
		case "enter", "l":
			if m.cursor >= 0 && m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				if entry.IsDir {
					m.dirPath = entry.Path
					return m, m.loadEntries()
				}
				m.view = viewFile
				return m, m.loadFile(entry.Path)
			}
		case "r":
			return m, m.loadEntries()
		}
	case viewFile:
		switch key.String() {
		case "esc", "backspace", "h":
			m.view = viewBrowser
		case "up", "k":
			m.fileOffset = max(0, m.fileOffset-1)
		case "down", "j":
			m.fileOffset++
		case "pgup":
			m.fileOffset = max(0, m.fileOffset-max(1, m.height-6))
		case "pgdown":
			m.fileOffset += max(1, m.height-6)
		}
	}
	return m, nil
}

func (m *tuiModel) updateFilterKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.editingFilter = false
	case "enter":
		m.filter = snapshotFilter{
			perimeter: m.filterChoices(0)[m.filterInputs[0]],
			tag:       m.filterChoices(1)[m.filterInputs[1]],
		}
		m.editingFilter = false
		m.applySnapshotFilter()
	case "tab", "shift+tab":
		m.filterField = 1 - m.filterField
	case "up", "k":
		m.filterInputs[m.filterField] = max(0, m.filterInputs[m.filterField]-1)
	case "down", "j":
		m.filterInputs[m.filterField] = min(len(m.filterChoices(m.filterField))-1, m.filterInputs[m.filterField]+1)
	}
	return m, nil
}

func (m *tuiModel) View() string {
	return renderView(m)
}

func (m *tuiModel) refresh() tea.Cmd {
	switch m.view {
	case viewDashboard:
		return m.loadDashboard()
	case viewSnapshots:
		return m.loadSnapshots()
	case viewBrowser:
		return m.loadEntries()
	case viewFile:
		if m.fileInfo != nil {
			return m.loadFile(m.fileInfo.Path)
		}
	}
	return nil
}

func (m *tuiModel) loadDashboard() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		data, err := LoadDashboard(m.repo)
		return dashboardMsg{data: data, err: err}
	}
}

func (m *tuiModel) loadSnapshots() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		items, err := ListSnapshots(m.repo)
		return snapshotsMsg{items: items, err: err}
	}
}

func (m *tuiModel) applySnapshotFilter() {
	m.snapshots = nil
	for _, item := range m.allSnapshots {
		if m.filter.perimeter != "" && item.Perimeter != m.filter.perimeter {
			continue
		}
		if m.filter.tag != "" && !slices.Contains(item.Tags, m.filter.tag) {
			continue
		}
		m.snapshots = append(m.snapshots, item)
	}
	m.page, m.cursor = 0, 0
}

func snapshotFilterOptions(items []SnapshotInfo) [2][]string {
	var options [2][]string
	perimeters := make(map[string]struct{})
	tags := make(map[string]struct{})
	for _, item := range items {
		if item.Perimeter != "" {
			perimeters[item.Perimeter] = struct{}{}
		}
		for _, tag := range item.Tags {
			if tag != "" {
				tags[tag] = struct{}{}
			}
		}
	}
	for perimeter := range perimeters {
		options[0] = append(options[0], perimeter)
	}
	for tag := range tags {
		options[1] = append(options[1], tag)
	}
	for index := range options {
		slices.Sort(options[index])
		options[index] = append([]string{""}, options[index]...)
	}
	return options
}

func (m *tuiModel) filterChoices(index int) []string {
	choices := m.filterOptions[index]
	if len(choices) == 0 {
		choices = []string{""}
	}
	value := m.filter.perimeter
	if index == 1 {
		value = m.filter.tag
	}
	if value != "" && !slices.Contains(choices, value) {
		choices = append(slices.Clone(choices), value)
	}
	return choices
}

func (m *tuiModel) loadEntries() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		items, err := ListDir(m.repo, m.snapshotID, m.dirPath)
		return entriesMsg{items: items, err: err}
	}
}

func (m *tuiModel) loadFile(filePath string) tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		file, info, err := OpenFile(m.repo, m.snapshotID, filePath)
		if err != nil {
			return fileMsg{err: err}
		}
		defer file.Close()
		content, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
		if err != nil {
			return fileMsg{err: fmt.Errorf("read file %q: %w", filePath, err)}
		}
		warning := ""
		if len(content) > maxFileBytes {
			content = content[:maxFileBytes]
			warning = "Showing the first 1 MiB"
		}
		if bytes.IndexByte(content, 0) >= 0 {
			return fileMsg{info: info, warning: "Binary file; content is not displayed"}
		}
		text := string(content)
		if !m.noHighlight {
			text = highlightFile(info.Name, text)
		}
		return fileMsg{info: info, content: text, warning: warning}
	}
}

func (m *tuiModel) selectedSnapshot() (SnapshotInfo, bool) {
	index := m.page*snapshotsPerPage + m.cursor
	if index < 0 || index >= len(m.snapshots) {
		return SnapshotInfo{}, false
	}
	return m.snapshots[index], true
}

func (m *tuiModel) snapshotPageCount() int {
	start := m.page * snapshotsPerPage
	return min(snapshotsPerPage, len(m.snapshots)-start)
}

func withinRoot(candidate, root string) bool {
	root = path.Clean(root)
	candidate = path.Clean(candidate)
	return candidate == root || strings.HasPrefix(candidate, strings.TrimSuffix(root, "/")+"/")
}
