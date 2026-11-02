package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

func newBoardCommand(app *App) *cobra.Command {
	var sprintID string
	cmd := &cobra.Command{
		Use:   "board",
		Short: "Show the active sprint as a kanban board",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint board"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()

			sprints, err := app.Services.Sprints.List(ctx, workspaceID)
			if err != nil {
				return err
			}
			var sprint api.Sprint
			if strings.TrimSpace(sprintID) != "" {
				id := strings.TrimSpace(sprintID)
				if err := validation.ID("sprint ID", id); err != nil {
					return err
				}
				for _, candidate := range sprints {
					if candidate.ID == id {
						sprint = candidate
						break
					}
				}
				if sprint.ID == "" {
					fetched, err := app.Services.Sprints.Get(ctx, id)
					if err != nil {
						return err
					}
					if fetched == nil {
						return fmt.Errorf("sprint %q was not returned by the backend", id)
					}
					sprint = *fetched
				}
			} else {
				var ok bool
				sprint, ok = findActiveSprint(sprints)
				if !ok {
					return errors.New("there is no active sprint; pass --sprint <id> to view another sprint")
				}
			}

			tasks, err := app.Services.Tasks.List(ctx, api.TaskFilter{
				WorkspaceID: workspaceID,
				SprintID:    sprint.ID,
			})
			if err != nil {
				return err
			}
			filtered := make([]api.Task, 0, len(tasks))
			for _, task := range tasks {
				if task.SprintID == sprint.ID {
					filtered = append(filtered, task)
				}
			}

			app.title("Sprint board · " + sprint.Name)
			app.keyValues([][2]string{
				{"Goal", dash(sprint.Goal)},
				{"Progress", progressText(sprint.DoneCount, sprint.TaskCount)},
				{"Ends", formatDate(sprint.EndDate)},
			}, 10)
			fmt.Fprintln(app.Out)

			if len(filtered) == 0 {
				app.empty("This sprint has no tasks yet.")
				return nil
			}
			for _, status := range api.TaskStatuses {
				var matching []api.Task
				for _, task := range filtered {
					if task.Status == status {
						matching = append(matching, task)
					}
				}
				heading := app.Styles.StatusStyle(status).Render(status.Title())
				fmt.Fprintf(app.Out, "%s %s\n", heading,
					app.Styles.Muted.Render(fmt.Sprintf("(%d)", len(matching))))
				if len(matching) == 0 {
					app.muted("  no tasks")
				}
				for _, task := range matching {
					line := fmt.Sprintf("  %-10s %-48s %-7s %s",
						dash(task.Key),
						components.Truncate(task.Title, 48),
						task.Priority.Label(),
						components.Truncate(task.AssigneeName(), 20),
					)
					fmt.Fprintln(app.Out, strings.TrimRight(line, " "))
				}
				fmt.Fprintln(app.Out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sprintID, "sprint", "", "sprint ID (defaults to the active sprint)")
	return cmd
}
