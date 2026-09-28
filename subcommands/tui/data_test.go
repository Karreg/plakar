package tui

import (
	"bytes"
	"io"
	"testing"

	ptesting "github.com/PlakarKorp/plakar/testing"
	"github.com/stretchr/testify/require"
)

func TestDashboardAndSnapshotBrowsing(t *testing.T) {
	t.Parallel()

	repo, _ := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("z-folder"),
		ptesting.NewMockFile("z-folder/hello.txt", 0644, "hello from snapshot"),
		ptesting.NewMockFile("alpha.txt", 0644, "alpha"),
	})
	require.NoError(t, snap.Close())

	dashboard, err := LoadDashboard(repo)
	require.NoError(t, err)
	require.Equal(t, 1, dashboard.Total)
	require.NotEmpty(t, dashboard.Location)
	require.Positive(t, dashboard.LogicalSize)
	require.Len(t, dashboard.SnapshotsPerDay, 30)
	require.Equal(t, 1, sumInts(dashboard.SnapshotsPerDay))

	snapshots, err := ListSnapshots(repo)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, snap.Header.Identifier, snapshots[0].ID)
	require.Equal(t, "/", snapshots[0].Importer)
	require.Positive(t, snapshots[0].Size)

	rootEntries, err := ListDir(repo, snapshots[0].ID, "")
	require.NoError(t, err)
	require.Len(t, rootEntries, 2)
	require.Equal(t, "z-folder", rootEntries[0].Name)
	require.True(t, rootEntries[0].IsDir)
	require.Equal(t, "alpha.txt", rootEntries[1].Name)

	children, err := ListDir(repo, snapshots[0].ID, rootEntries[0].Path)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, "hello.txt", children[0].Name)

	file, info, err := OpenFile(repo, snapshots[0].ID, children[0].Path)
	require.NoError(t, err)
	defer file.Close()
	require.Equal(t, "hello.txt", info.Name)
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "hello from snapshot", string(content))
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}
