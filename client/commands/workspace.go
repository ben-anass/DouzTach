package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

func newWorkspaceCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "List, create, and switch workspaces",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if app.Config.WorkspaceID == "" {
				app.muted("No workspace selected. Use 'douztech workspace list' and 'douztech workspace switch <id>'.")
			} else {
				app.muted("Current workspace: %s", app.Config.WorkspaceID)
			}
			return nil
		},
	}
	cmd.AddCommand(
		newWorkspaceListCommand(app),
		newWorkspaceCreateCommand(app),
		newWorkspaceSwitchCommand(app),
		newWorkspaceMembersCommand(app),
	)
	return cmd
}

func newWorkspaceListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspaces available to your account",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Workspaces"); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			workspaces, err := app.Services.Workspaces.List(ctx)
			if err != nil {
				return err
			}
			if len(workspaces) == 0 {
				app.empty("No workspaces were returned for this account.")
				return nil
			}
			rows := make([][]string, 0, len(workspaces))
			for _, workspace := range workspaces {
				marker := " "
				if workspace.ID == app.Config.WorkspaceID {
					marker = "*"
				}
				rows = append(rows, []string{
					marker,
					cell(workspace.ID, 24),
					cell(workspace.Name, 32),
					dash(workspace.Role),
					fmt.Sprintf("%d", workspace.MemberCount),
				})
			}
			app.table([]string{"", "ID", "NAME", "ROLE", "MEMBERS"}, rows)
			app.spacer()
			app.muted("* = selected workspace. Switch with 'douztech workspace switch <id>'.")
			return nil
		},
	}
}

func newWorkspaceCreateCommand(app *App) *cobra.Command {
	var (
		name        string
		description string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Workspace creation"); err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				if !app.Interactive {
					return errors.New("workspace name is required; pass --name")
				}
				values, err := prompts.RunForm(cmd.Context(), prompts.WorkspaceForm(app.Styles))
				if err != nil {
					return err
				}
				name = values["name"]
				if description == "" {
					description = values["description"]
				}
			}
			if err := validation.WorkspaceName("workspace name", name); err != nil {
				return err
			}
			if err := validation.Description("description", description); err != nil {
				return err
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			workspace, err := app.Services.Workspaces.Create(ctx, api.CreateWorkspaceRequest{
				Name:        strings.TrimSpace(name),
				Description: strings.TrimSpace(description),
			})
			if err != nil {
				return err
			}
			if workspace == nil || strings.TrimSpace(workspace.ID) == "" {
				return errors.New("the backend returned a workspace without an ID; the selection was not changed")
			}
			app.Config.WorkspaceID = workspace.ID
			if err := app.saveConfig(); err != nil {
				return err
			}
			app.success("Created workspace %s (%s) and selected it.", workspace.Name, workspace.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "workspace name")
	cmd.Flags().StringVar(&description, "description", "", "workspace description")
	return cmd
}

func newWorkspaceMembersCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "members [workspace-id]",
		Short: "List the members of a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Workspace members"); err != nil {
				return err
			}
			workspaceID := ""
			if len(args) == 1 {
				workspaceID = strings.TrimSpace(args[0])
				if err := validation.ID("workspace ID", workspaceID); err != nil {
					return err
				}
			} else {
				resolved, err := app.workspaceID()
				if err != nil {
					return err
				}
				workspaceID = resolved
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			members, err := app.Services.Workspaces.Members(ctx, workspaceID)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				app.empty("No members were returned for this workspace.")
				return nil
			}
			rows := make([][]string, 0, len(members))
			for _, member := range members {
				rows = append(rows, []string{
					cell(member.User.DisplayName(), 24),
					cell(dash(member.User.Username), 18),
					cell(dash(member.User.Email), 30),
					cell(dash(member.Role), 12),
					formatTimestamp(member.JoinedAt),
				})
			}
			app.table([]string{"NAME", "USERNAME", "EMAIL", "ROLE", "JOINED"}, rows)
			return nil
		},
	}
}

func newWorkspaceSwitchCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "switch [workspace-id]",
		Short: "Select the workspace used by subsequent commands",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID := ""
			if len(args) == 1 {
				workspaceID = strings.TrimSpace(args[0])
				if err := validation.ID("workspace ID", workspaceID); err != nil {
					return err
				}
			}

			if workspaceID == "" {
				if err := app.requireReady("Workspaces"); err != nil {
					return err
				}
				if !app.Interactive {
					return errors.New("workspace switch needs a workspace ID when not running in a terminal")
				}
				ctx, cancel := app.requestContext(cmd.Context())
				workspaces, err := app.Services.Workspaces.List(ctx)
				cancel()
				if err != nil {
					return err
				}
				if len(workspaces) == 0 {
					return errors.New("no workspaces were returned for this account")
				}
				option, err := prompts.RunSelect(cmd.Context(),
					prompts.NewSelect(app.Styles, "Select a workspace", prompts.WorkspaceOptions(workspaces)))
				if err != nil {
					return err
				}
				workspaceID = option.Value
			}

			if app.Services.Ready() {
				if err := app.requireReady("Workspaces"); err != nil {
					return err
				}
				ctx, cancel := app.requestContext(cmd.Context())
				workspaces, err := app.Services.Workspaces.List(ctx)
				cancel()
				if err != nil {
					return err
				}
				found := false
				for _, workspace := range workspaces {
					if workspace.ID == workspaceID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("workspace %q is not available to this account", workspaceID)
				}
			} else {
				app.warn("the backend contract is pending; the workspace ID was saved without server verification")
			}

			app.Config.WorkspaceID = workspaceID
			if err := app.saveConfig(); err != nil {
				return err
			}
			app.success("Workspace set to %s.", workspaceID)
			return nil
		},
	}
}
