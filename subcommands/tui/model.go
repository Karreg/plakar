package tui

import (
	"bytes"
	"fmt"
	"io"
	"path"
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
	repo        *repository.Repository
	noHighlight bool
	view        int
	width       int
	height      int
	loading     bool
	err         error
	dashboard   *DashboardData
	snapshots   []SnapshotInfo
	page        int
	cursor      int
	snapshotID  objects.MAC
	rootPath    string
	dirPath     string
	entries     []EntryInfo
	fileInfo    *EntryInfo
	fileContent string
	fileWarning string
	fileOffset  int
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
		m.snapshots, m.err = msg.items, msg.err
		m.page, m.cursor = 0, 0
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
	if key.String() == "ctrl+c" || key.String() == "q" {
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
