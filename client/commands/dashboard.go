package commands

import (
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/spf13/cobra"
)

func newDashboardCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Show task counts, the active sprint, and recent activity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !app.Services.Ready() {
				app.printLocalSummary()
				return pending("Dashboard data")
			}
			if app.Session == nil || !app.Session.Valid() {
				return errNotSignedIn()
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()

			workspaces, err := app.Services.Workspaces.List(ctx)
			if err != nil {
				return fmt.Errorf("load workspaces: %w", err)
			}
			workspaceID := app.Config.WorkspaceID
			if workspaceID == "" && len(workspaces) > 0 {
				workspaceID = workspaces[0].ID
			}
			if workspaceID == "" {
				app.title("Dashboard")
				app.empty("No workspace is available for this account.")
				return nil
			}

			tasks, err := app.Services.Tasks.List(ctx, api.TaskFilter{WorkspaceID: workspaceID})
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}
			sprints, err := app.Services.Sprints.List(ctx, workspaceID)
			if err != nil {
				return fmt.Errorf("load sprints: %w", err)
			}
			activities, err := app.Services.Activity.List(ctx, workspaceID, 5)
			if err != nil {
				return fmt.Errorf("load activity: %w", err)
			}

			app.title("Dashboard")
			app.keyValues([][2]string{
				{"Signed in", app.Session.User.DisplayName()},
				{"Workspace", workspaceName(workspaces, workspaceID)},
				{"API", app.Config.APIBaseURL},
			}, 12)
			fmt.Fprintln(app.Out)

			counts := api.CountTasks(tasks, app.Now())
			app.keyValues([][2]string{
				{"Total tasks", fmt.Sprintf("%d", counts.Total)},
				{"To do", fmt.Sprintf("%d", counts.Todo)},
				{"In progress", fmt.Sprintf("%d", counts.InProgress)},
				{"Review", fmt.Sprintf("%d", counts.Review)},
				{"Completed", fmt.Sprintf("%d", counts.Done)},
				{"Overdue", fmt.Sprintf("%d", counts.Overdue)},
			}, 12)
			fmt.Fprintln(app.Out)

			if sprint, ok := findActiveSprint(sprints); ok {
				app.title("Active sprint")
				app.keyValues([][2]string{
					{"Name", sprint.Name},
					{"Goal", dash(sprint.Goal)},
					{"Progress", progressText(sprint.DoneCount, sprint.TaskCount)},
					{"Ends", formatDate(sprint.EndDate)},
				}, 10)
			} else {
				app.empty("No active sprint.")
			}
			fmt.Fprintln(app.Out)

			app.title("Recent activity")
			if len(activities) == 0 {
				app.empty("No activity recorded yet.")
			} else {
				for _, entry := range activities {
					actor := entry.ActorName
					if actor == "" {
						actor = "Someone"
					}
					line := fmt.Sprintf("%s  %s %s %s",
						formatTimestamp(entry.CreatedAt), actor, entry.Action, entry.Subject)
					fmt.Fprintln(app.Out, components.Truncate(strings.Join(strings.Fields(line), " "), 100))
				}
			}
			return nil
		},
	}
}

// printLocalSummary shows what is known without contacting the backend.
func (a *App) printLocalSummary() {
	a.title("DouzTach")
	apiURL := a.Config.APIBaseURL
	if apiURL == "" {
		apiURL = "not configured"
	}
	session := "not signed in"
	if a.Session != nil {
		session = a.Session.User.DisplayName()
	}
	workspace := a.Config.WorkspaceID
	if workspace == "" {
		workspace = "not selected"
	}
	a.keyValues([][2]string{
		{"Version", a.Version},
		{"API URL", apiURL},
		{"Session", session},
		{"Workspace", workspace},
		{"Contract", a.Services.Status.Summary()},
	}, 12)
}

func workspaceName(workspaces []api.Workspace, id string) string {
	for _, workspace := range workspaces {
		if workspace.ID == id {
			return workspace.Name
		}
	}
	return id
}

func findActiveSprint(sprints []api.Sprint) (api.Sprint, bool) {
	for _, sprint := range sprints {
		if sprint.Status == api.SprintStatusActive {
			return sprint, true
		}
	}
	return api.Sprint{}, false
}
