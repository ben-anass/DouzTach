package commands

import (
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

func newSprintCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sprint",
		Short: "List and inspect sprints",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newSprintListCommand(app), newSprintShowCommand(app))
	return cmd
}

func newSprintListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List sprints in the selected workspace",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprints"); err != nil {
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
			if len(sprints) == 0 {
				app.empty("No sprints were returned for this workspace.")
				return nil
			}
			rows := make([][]string, 0, len(sprints))
			for _, sprint := range sprints {
				rows = append(rows, []string{
					cell(sprint.ID, 24),
					cell(sprint.Name, 30),
					string(sprint.Status),
					formatDate(sprint.StartDate),
					formatDate(sprint.EndDate),
					progressText(sprint.DoneCount, sprint.TaskCount),
				})
			}
			app.table([]string{"ID", "NAME", "STATUS", "START", "END", "PROGRESS"}, rows)
			return nil
		},
	}
}

func newSprintShowCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <sprint-id>",
		Short: "Show a sprint and its tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprints"); err != nil {
				return err
			}
			sprintID := strings.TrimSpace(args[0])
			if err := validation.ID("sprint ID", sprintID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			sprint, err := app.Services.Sprints.Get(ctx, sprintID)
			if err != nil {
				return err
			}
			if sprint == nil {
				return fmt.Errorf("sprint %q was not returned by the backend", sprintID)
			}
			app.title(sprint.Name)
			app.keyValues([][2]string{
				{"ID", sprint.ID},
				{"Status", string(sprint.Status)},
				{"Goal", dash(sprint.Goal)},
				{"Start", formatDate(sprint.StartDate)},
				{"End", formatDate(sprint.EndDate)},
				{"Progress", progressText(sprint.DoneCount, sprint.TaskCount)},
			}, 10)
			fmt.Fprintln(app.Out)

			tasks, err := app.Services.Tasks.List(ctx, api.TaskFilter{
				WorkspaceID: workspaceID,
				SprintID:    sprintID,
			})
			if err != nil {
				return err
			}
			filtered := make([]api.Task, 0, len(tasks))
			for _, task := range tasks {
				if task.SprintID == sprintID {
					filtered = append(filtered, task)
				}
			}
			if len(filtered) == 0 {
				app.empty("This sprint has no tasks.")
				return nil
			}
			sortTasks(filtered)
			rows := make([][]string, 0, len(filtered))
			for _, task := range filtered {
				rows = append(rows, taskRow(task))
			}
			app.table([]string{"KEY", "TITLE", "STATUS", "PRIORITY", "TYPE", "ASSIGNEE", "DUE"}, rows)
			return nil
		},
	}
}

func newEpicCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "List and inspect epics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newEpicListCommand(app), newEpicShowCommand(app))
	return cmd
}

func newEpicListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List epics in the selected workspace",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Epics"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			epics, err := app.Services.Epics.List(ctx, workspaceID)
			if err != nil {
				return err
			}
			if len(epics) == 0 {
				app.empty("No epics were returned for this workspace.")
				return nil
			}
			rows := make([][]string, 0, len(epics))
			for _, epic := range epics {
				rows = append(rows, []string{
					dash(epic.Key),
					cell(epic.Title, 40),
					dash(epic.Status),
					progressText(epic.DoneCount, epic.TaskCount),
					formatDate(epic.DueDate),
				})
			}
			app.table([]string{"KEY", "TITLE", "STATUS", "PROGRESS", "DUE"}, rows)
			return nil
		},
	}
}

func newEpicShowCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <epic-id>",
		Short: "Show an epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Epics"); err != nil {
				return err
			}
			epicID := strings.TrimSpace(args[0])
			if err := validation.ID("epic ID", epicID); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			epic, err := app.Services.Epics.Get(ctx, epicID)
			if err != nil {
				return err
			}
			if epic == nil {
				return fmt.Errorf("epic %q was not returned by the backend", epicID)
			}
			app.title(epic.Title)
			app.keyValues([][2]string{
				{"ID", epic.ID},
				{"Key", dash(epic.Key)},
				{"Status", dash(epic.Status)},
				{"Owner", epicOwner(epic)},
				{"Due", formatDate(epic.DueDate)},
				{"Progress", progressText(epic.DoneCount, epic.TaskCount)},
				{"Description", dash(epic.Description)},
			}, 12)
			return nil
		},
	}
}

func epicOwner(epic *api.Epic) string {
	if epic == nil || epic.Owner == nil {
		return "Unassigned"
	}
	return epic.Owner.DisplayName()
}
