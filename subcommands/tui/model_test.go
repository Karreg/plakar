package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/PlakarKorp/kloset/objects"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestSnapshotNavigationAndPaging(t *testing.T) {
	t.Parallel()

	model := newModel(nil, true)
	model.view = viewSnapshots
	model.width, model.height = 100, 20
	model.snapshots = make([]SnapshotInfo, snapshotsPerPage+1)
	for index := range model.snapshots {
		model.snapshots[index] = SnapshotInfo{
			ID:        objects.MAC{byte(index + 1)},
			ShortID:   "snapshot",
			Timestamp: time.Unix(int64(index), 0),
			Importer:  "/backup",
		}
	}

	for range snapshotsPerPage - 1 {
		model = sendKey(t, model, "down")
	}
	require.Equal(t, snapshotsPerPage-1, model.cursor)
	model = sendKey(t, model, "down")
	require.Equal(t, 1, model.page)
	require.Zero(t, model.cursor)

	model = sendKey(t, model, "enter")
	require.Equal(t, viewBrowser, model.view)
	require.Equal(t, model.snapshots[snapshotsPerPage].ID, model.snapshotID)
	require.Equal(t, "/backup", model.rootPath)
}

func TestBrowserBackNavigation(t *testing.T) {
	t.Parallel()

	model := newModel(nil, true)
	model.view = viewBrowser
	model.rootPath = "/backup"
	model.dirPath = "/backup/folder"
	model = sendKey(t, model, "backspace")
	require.Equal(t, "/backup", model.dirPath)
	require.Equal(t, viewBrowser, model.view)
	// The backspace triggers a directory reload; complete it before the next key.
	model = sendMsg(t, model, entriesMsg{items: []EntryInfo{}})

	model = sendKey(t, model, "backspace")
	require.Equal(t, viewSnapshots, model.view)
	model = sendKey(t, model, "esc")
	require.Equal(t, viewDashboard, model.view)
}

func TestHistogramAndDashboardRender(t *testing.T) {
	t.Parallel()

	values := make([]int, 30)
	values[14] = 10 // max: fills all 5 levels
	values[15] = 2  // partial: fills only the lowest level
	histogram := renderHistogram(values)
	require.Equal(t, 6, strings.Count(histogram, "#"))
	require.Contains(t, histogram, "1")

	model := newModel(nil, true)
	model.dashboard = &DashboardData{
		Location:        "fs:///repo",
		Total:           1,
		StorageSize:     20,
		LogicalSize:     40,
		Efficiency:      50,
		SnapshotsPerDay: values,
	}
	model.width = 100
	view := renderDashboard(model)
	require.Contains(t, view, "repository dashboard")
	require.Contains(t, view, "50.0%")
	require.Contains(t, view, "Snapshots per day")
}

func sendMsg(t *testing.T, model *tuiModel, message tea.Msg) *tuiModel {
	t.Helper()
	next, _ := model.Update(message)
	updated, ok := next.(*tuiModel)
	require.True(t, ok)
	return updated
}

func sendKey(t *testing.T, model *tuiModel, key string) *tuiModel {
	t.Helper()
	var message tea.Msg
	switch key {
	case "up":
		message = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		message = tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		message = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		message = tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		message = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		message = tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		message = tea.KeyMsg{Type: tea.KeyBackspace}
	default:
		message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	next, _ := model.Update(message)
	updated, ok := next.(*tuiModel)
	require.True(t, ok)
	return updated
}
