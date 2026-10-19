package tui

import (
	"os"
	"os/exec"
	"testing"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		args        []string
		path        string
		noHighlight bool
		wantErr     string
	}{
		{name: "dashboard"},
		{name: "no highlight", args: []string{"-no-highlight"}, noHighlight: true},
		{name: "snapshot", args: []string{"abcdef12"}, path: "abcdef12"},
		{name: "directory", args: []string{"abcdef12:/backup"}, path: "abcdef12:/backup"},
		{name: "no highlight directory", args: []string{"-no-highlight", "abcdef12:/backup"}, path: "abcdef12:/backup", noHighlight: true},
		{name: "explicit false", args: []string{"-no-highlight=false", "abcdef12"}, path: "abcdef12"},
		{name: "too many arguments", args: []string{"abcdef12", "/backup"}, wantErr: "too many arguments"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := appcontext.NewAppContext()
			t.Cleanup(func() { ctx.Close() })
			ctx.SetSecret([]byte("test secret"))

			cmd := &Tui{}
			err := cmd.Parse(ctx, test.args)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.path, cmd.SnapshotPath)
			require.Equal(t, test.noHighlight, cmd.NoHighlight)
			require.Equal(t, ctx.GetSecret(), cmd.GetRepositorySecret())
		})
	}
}

func TestParseFlagExit(t *testing.T) {
	const helperEnv = "PLAKAR_TEST_TUI_FLAG"
	if arg := os.Getenv(helperEnv); arg != "" {
		ctx := appcontext.NewAppContext()
		defer ctx.Close()
		require.NoError(t, (&Tui{}).Parse(ctx, []string{arg}))
		t.Fatal("flag parsing should exit")
	}

	executable, err := os.Executable()
	require.NoError(t, err)
	for _, test := range []struct {
		name string
		arg  string
		code int
		want string
	}{
		{name: "unknown flag", arg: "-unknown", code: 2, want: "flag provided but not defined: -unknown"},
		{name: "invalid boolean", arg: "-no-highlight=invalid", code: 2, want: "invalid boolean value"},
		{name: "help", arg: "-h", want: "Usage: tui [-no-highlight] [SNAPSHOT[:PATH]]"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(helperEnv, test.arg)
			cmd := exec.Command(executable, "-test.run=^TestParseFlagExit$")
			output, err := cmd.CombinedOutput()
			if test.code == 0 {
				require.NoError(t, err, "%s", output)
			} else {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				require.Equal(t, test.code, exitErr.ExitCode(), "%s", output)
			}
			require.Contains(t, string(output), test.want)
		})
	}
}

func TestCommandRegistration(t *testing.T) {
	t.Parallel()

	cmd, matched, args := subcommands.Lookup([]string{"tui", "-no-highlight"})
	require.IsType(t, &Tui{}, cmd)
	require.Equal(t, []string{"tui"}, matched)
	require.Equal(t, []string{"-no-highlight"}, args)
	require.Zero(t, cmd.GetFlags())
}
