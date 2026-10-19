package tui

import (
	"flag"
	"fmt"

	"github.com/PlakarKorp/kloset/locate"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/subcommands"
	tea "github.com/charmbracelet/bubbletea"
)

type Tui struct {
	subcommands.SubcommandBase
	SnapshotPath string
	NoHighlight  bool
}

func init() {
	subcommands.Register(func() subcommands.Subcommand { return &Tui{} }, 0, "tui")
}

func (cmd *Tui) Parse(ctx *appcontext.AppContext, args []string) error {
	flags := flag.NewFlagSet("tui", flag.ExitOnError)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "Usage: %s [-no-highlight] [SNAPSHOT[:PATH]]\n", flags.Name())
		flags.PrintDefaults()
	}
	flags.BoolVar(&cmd.NoHighlight, "no-highlight", false, "disable syntax highlighting")
	flags.Parse(args)

	if flags.NArg() > 1 {
		return fmt.Errorf("too many arguments")
	}
	if flags.NArg() == 1 {
		cmd.SnapshotPath = flags.Arg(0)
	}
	cmd.RepositorySecret = ctx.GetSecret()
	return nil
}

func (cmd *Tui) Execute(ctx *appcontext.AppContext, repo *repository.Repository) (int, error) {
	model := newModel(repo, cmd.NoHighlight)
	if cmd.SnapshotPath != "" {
		snap, dirPath, err := locate.OpenSnapshotByPath(repo, cmd.SnapshotPath)
		if err != nil {
			return 1, fmt.Errorf("open snapshot: %w", err)
		}
		model.snapshotID = snap.Header.Identifier
		model.rootPath = snap.Header.GetSource(0).Importer.Directory
		model.dirPath = dirPath
		if err := snap.Close(); err != nil {
			return 1, fmt.Errorf("close snapshot: %w", err)
		}
		model.view = viewBrowser
	}

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		return 1, fmt.Errorf("run tui: %w", err)
	}
	return 0, nil
}
