package tui

import (
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

func TestSnapshotFilters(t *testing.T) {
	t.Parallel()

	items := []SnapshotInfo{
		{ShortID: "one", Perimeter: "prod", Tags: []string{"daily", "important"}},
		{ShortID: "two", Perimeter: "prod", Tags: []string{"weekly"}},
		{ShortID: "three", Perimeter: "staging", Tags: []string{"daily"}},
	}
	for _, test := range []struct {
		name   string
		filter snapshotFilter
		want   []string
	}{
		{name: "no filter", want: []string{"one", "two", "three"}},
		{name: "perimeter", filter: snapshotFilter{perimeter: "prod"}, want: []string{"one", "two"}},
		{name: "tag", filter: snapshotFilter{tag: "daily"}, want: []string{"one", "three"}},
		{name: "both", filter: snapshotFilter{perimeter: "prod", tag: "daily"}, want: []string{"one"}},
		{name: "exact perimeter", filter: snapshotFilter{perimeter: "pro"}},
		{name: "exact tag", filter: snapshotFilter{tag: "day"}},
		{name: "case sensitive", filter: snapshotFilter{tag: "Daily"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := newModel(nil, true)
			model.filter = test.filter
			model = sendMsg(t, model, snapshotsMsg{items: items})
			var got []string
			for _, item := range model.snapshots {
				got = append(got, item.ShortID)
			}
			require.Equal(t, test.want, got)
			require.Zero(t, model.page)
			require.Zero(t, model.cursor)
		})
	}
}

func TestSnapshotFilterOptions(t *testing.T) {
	t.Parallel()

	model := newModel(nil, true)
	model.filter = snapshotFilter{perimeter: "prod"}
	model = sendMsg(t, model, snapshotsMsg{items: []SnapshotInfo{
		{Perimeter: "prod", Tags: []string{"weekly", "daily", ""}},
		{Perimeter: "staging", Tags: []string{"daily"}},
		{Perimeter: "", Tags: []string{"weekly"}},
	}})
	require.Equal(t, []string{"", "prod", "staging"}, model.filterOptions[0])
	require.Equal(t, []string{"", "daily", "weekly"}, model.filterOptions[1])
	require.Len(t, model.snapshots, 1)

	model = sendMsg(t, model, snapshotsMsg{})
	require.Equal(t, [2][]string{{""}, {""}}, model.filterOptions)
}

func TestSnapshotFilterMenuAfterRefresh(t *testing.T) {
	t.Parallel()

	model := newModel(nil, true)
	model.view = viewSnapshots
	model.width, model.height = 32, 12
	model.filter = snapshotFilter{perimeter: "old"}
	model = sendMsg(t, model, snapshotsMsg{items: []SnapshotInfo{{Perimeter: "new", Tags: []string{"daily"}}}})
	require.Equal(t, snapshotFilter{perimeter: "old"}, model.filter)
	require.Empty(t, model.snapshots)
	model = sendKey(t, model, "f")
	require.Contains(t, renderSnapshots(model), "old (unavailable)")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "enter")
	require.Equal(t, "new", model.filter.perimeter)
	require.Len(t, model.snapshots, 1)
	model = sendMsg(t, model, snapshotsMsg{})
	require.Equal(t, [2][]string{{""}, {""}}, model.filterOptions)
	model = sendKey(t, model, "f")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "enter")
	require.Empty(t, model.filter.perimeter)
	require.Contains(t, renderSnapshots(model), "No snapshots found")
}

func TestSnapshotFilterMenuScroll(t *testing.T) {
	t.Parallel()

	items := make([]SnapshotInfo, 15)
	for index := range items {
		items[index].Perimeter = string(rune('a' + index))
	}
	model := newModel(nil, true)
	model.view = viewSnapshots
	model.width, model.height = 24, 12
	model = sendMsg(t, model, snapshotsMsg{items: items})
	model = sendKey(t, model, "f")
	for range len(items) + 2 {
		model = sendKey(t, model, "down")
	}
	require.Equal(t, len(items), model.filterInputs[0])
	require.Contains(t, renderSnapshots(model), "Choices 14-16/16")
	model = sendKey(t, model, "tab")
	require.Contains(t, renderSnapshots(model), "Choices 1-1/1")
	model = sendKey(t, model, "shift+tab")
	require.Equal(t, 0, model.filterField)
	model = sendKey(t, model, "enter")
	require.Equal(t, "o", model.filter.perimeter)
	require.Len(t, model.snapshots, 1)
}

func TestSnapshotFilterEditor(t *testing.T) {
	t.Parallel()

	model := newModel(nil, true)
	model.view = viewSnapshots
	model.width, model.height = 80, 20
	model = sendMsg(t, model, snapshotsMsg{items: []SnapshotInfo{
		{ShortID: "one", Perimeter: "prod", Tags: []string{"daily"}},
		{ShortID: "two", Perimeter: "staging", Tags: []string{"weekly"}},
	}})
	model = sendKey(t, model, "f")
	require.True(t, model.editingFilter)
	model = sendKey(t, model, "down")
	model = sendKey(t, model, "tab")
	model = sendKey(t, model, "down")
	model = sendKey(t, model, "enter")
	require.False(t, model.editingFilter)
	require.Equal(t, snapshotFilter{perimeter: "prod", tag: "daily"}, model.filter)
	require.Len(t, model.snapshots, 1)
	require.Equal(t, "one", model.snapshots[0].ShortID)

	model = sendKey(t, model, "f")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "esc")
	require.Equal(t, "prod", model.filter.perimeter)
	model = sendKey(t, model, "f")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "enter")
	require.Empty(t, model.filter.perimeter)
	require.Len(t, model.snapshots, 1)
	model = sendKey(t, model, "f")
	model = sendKey(t, model, "tab")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "enter")
	require.Empty(t, model.filter.tag)
	require.Len(t, model.snapshots, 2)
}

func TestSnapshotFilterNavigationAndRefresh(t *testing.T) {
	t.Parallel()

	items := []SnapshotInfo{{ID: objects.MAC{1}, Perimeter: "other"}}
	for index := range snapshotsPerPage + 1 {
		items = append(items, SnapshotInfo{
			ID:        objects.MAC{byte(index + 2)},
			Perimeter: "prod",
			Importer:  "/backup",
		})
	}
	model := newModel(nil, true)
	model.view = viewSnapshots
	model.filter = snapshotFilter{perimeter: "prod"}
	model = sendMsg(t, model, snapshotsMsg{items: items})
	for range snapshotsPerPage {
		model = sendKey(t, model, "down")
	}
	require.Equal(t, 1, model.page)
	model = sendKey(t, model, "enter")
	require.Equal(t, items[len(items)-1].ID, model.snapshotID)
	model = sendMsg(t, model, entriesMsg{})
	model = sendKey(t, model, "esc")
	require.Equal(t, viewSnapshots, model.view)
	require.Equal(t, 1, model.page)
	model.width = 80
	model = sendKey(t, model, "f")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "up")
	model = sendKey(t, model, "enter")
	require.Empty(t, model.filter.perimeter)
	require.Zero(t, model.page)
	require.Zero(t, model.cursor)
	require.Len(t, model.snapshots, len(items))
	model = sendKey(t, model, "f")
	model = sendKey(t, model, "down")
	model = sendKey(t, model, "down")
	model = sendKey(t, model, "enter")

	model = sendKey(t, model, "r")
	require.True(t, model.loading)
	model = sendMsg(t, model, snapshotsMsg{items: []SnapshotInfo{{Perimeter: "other"}}})
	require.Equal(t, snapshotFilter{perimeter: "prod"}, model.filter)
	require.Zero(t, model.page)
	require.Zero(t, model.cursor)
	require.Empty(t, model.snapshots)
	require.Contains(t, renderSnapshots(model), "No snapshots match filters")
	model = sendKey(t, model, "enter")
	require.Equal(t, viewSnapshots, model.view)
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

func TestDashboardRender(t *testing.T) {
	t.Parallel()

	model := newModel(nil, true)
	model.dashboard = &DashboardData{
		Location:    "fs:///repo",
		Total:       1,
		StorageSize: 20,
		LogicalSize: 40,
		Efficiency:  50,
	}
	model.width = 100
	view := renderDashboard(model)
	require.Contains(t, view, "repository dashboard")
	require.Contains(t, view, "50.0%")
	require.NotContains(t, view, "Snapshots per day")
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
