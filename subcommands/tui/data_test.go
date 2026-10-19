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
	}, ptesting.WithPerimeter("production"), ptesting.WithTags([]string{"daily"}))
	require.NoError(t, snap.Close())

	dashboard, err := LoadDashboard(repo)
	require.NoError(t, err)
	require.Equal(t, 1, dashboard.Total)
	require.NotEmpty(t, dashboard.Location)
	require.Positive(t, dashboard.LogicalSize)

	snapshots, err := ListSnapshots(repo)
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, snap.Header.Identifier, snapshots[0].ID)
	require.Equal(t, "/", snapshots[0].Importer)
	require.Positive(t, snapshots[0].Size)
	require.Equal(t, "production", snapshots[0].Perimeter)
	require.Equal(t, []string{"daily"}, snapshots[0].Tags)

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
	require.NotEmpty(t, info.ContentType)
	require.Positive(t, info.Chunks)
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "hello from snapshot", string(content))
}

func TestFilePreview(t *testing.T) {
	t.Parallel()

	repo, ctx := ptesting.GenerateRepository(t, bytes.NewBuffer(nil), bytes.NewBuffer(nil), nil)
	t.Cleanup(func() { ctx.Close() })
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	exactLimit := bytes.Repeat([]byte("x"), maxFileBytes)
	overLimit := append(bytes.Clone(exactLimit), 'y')
	snap := ptesting.GenerateSnapshot(t, repo, []ptesting.MockFile{
		ptesting.NewMockDir("folder"),
		ptesting.NewMockFile("empty.txt", 0644, ""),
		ptesting.NewMockFile("text.txt", 0644, "hello"),
		ptesting.NewMockFile("binary.bin", 0644, "hello\x00world"),
		ptesting.NewMockFile("limit.txt", 0644, string(exactLimit)),
		ptesting.NewMockFile("large.txt", 0644, string(overLimit)),
	})
	require.NoError(t, snap.Close())

	for _, test := range []struct {
		name    string
		content string
		warning string
		wantErr string
	}{
		{name: "empty.txt"},
		{name: "text.txt", content: "hello"},
		{name: "binary.bin", warning: "Binary file; content is not displayed"},
		{name: "limit.txt", content: string(exactLimit)},
		{name: "large.txt", content: string(exactLimit), warning: "Showing the first 1 MiB"},
		{name: "folder", wantErr: `"/folder" is not a regular file`},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := newModel(repo, true)
			model.snapshotID = snap.Header.Identifier
			result, ok := model.loadFile("/" + test.name)().(fileMsg)
			require.True(t, ok)
			if test.wantErr != "" {
				require.EqualError(t, result.err, test.wantErr)
				return
			}
			require.NoError(t, result.err)
			require.NotNil(t, result.info)
			require.Equal(t, test.name, result.info.Name)
			require.Equal(t, test.content, result.content)
			require.Equal(t, test.warning, result.warning)
		})
	}
}
