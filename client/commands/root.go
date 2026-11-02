package commands

import (
	"context"
	"errors"

	"github.com/ben-anass/DouzTach/client/terminal"
	"github.com/spf13/cobra"
)

// Execute builds the root command and runs it with the process arguments.
func Execute(ctx context.Context, version string) error {
	app := NewApp(version)
	return NewRootCommand(app).ExecuteContext(ctx)
}

// NewRootCommand builds the full command tree. It is exported so tests can
// execute commands with isolated streams and a temporary config directory.
func NewRootCommand(app *App) *cobra.Command {
	flags := &GlobalFlags{}
	root := &cobra.Command{
		Use:   "douztech",
		Short: "DouzTach project management from the terminal",
		Long: `DouzTach is a project-management client for teams.

Running 'douztech' without a command opens the interactive dashboard. Use the
subcommands for scriptable access to the same data. The client only talks to
the backend endpoints published by the DouzTach API contract; when a contract
is missing the command fails clearly instead of simulating data.`,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return app.Load(*flags)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.runTUI(cmd.Context())
		},
	}
	root.PersistentFlags().StringVar(&flags.ConfigDir, "config-dir", "",
		"configuration directory (default: the OS user config directory)")
	root.PersistentFlags().StringVar(&flags.APIURL, "api-url", "",
		"override the backend API base URL for this run")
	root.PersistentFlags().DurationVar(&flags.Timeout, "timeout", 0,
		"override the HTTP request timeout for this run (for example 30s)")
	root.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false,
		"disable ANSI colours")

	root.AddCommand(
		newLoginCommand(app),
		newRegisterCommand(app),
		newLogoutCommand(app),
		newDashboardCommand(app),
		newWorkspaceCommand(app),
		newTaskCommand(app),
		newBoardCommand(app),
		newSprintCommand(app),
		newEpicCommand(app),
		newNotificationCommand(app),
		newActivityCommand(app),
		newChatCommand(app),
		newProfileCommand(app),
		newSettingsCommand(app),
		newSetupCommand(app),
		newDoctorCommand(app),
		newVersionCommand(app),
	)
	return root
}

// runTUI launches the interactive Bubble Tea application.
func (a *App) runTUI(ctx context.Context) error {
	if !a.Interactive {
		return errors.New("the interactive dashboard needs a terminal; run 'douztech --help' for commands or 'douztech dashboard' for a summary")
	}
	if !a.stdoutIsTerminal() {
		return errors.New("the interactive dashboard needs a terminal on standard output; use 'douztech dashboard' when redirecting output")
	}

	return terminal.Run(ctx, terminal.Options{
		Version:  a.Version,
		Styles:   a.Styles,
		Services: a.Services,
		Manager:  a.Manager,
		Config:   a.Config,
		Session:  a.Session,
		Now:      a.now,
	})
}
