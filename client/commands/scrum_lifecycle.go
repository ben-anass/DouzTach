package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

var epicStatuses = []string{"planned", "active", "completed", "archived"}
var epicPriorities = []string{"low", "medium", "high", "critical"}

func newEpicCreateCommand(app *App) *cobra.Command {
	var title, description, status, priority, color, startDate, endDate string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an epic in the selected workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Epic creation"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			req, err := epicRequestFromFlags(title, description, status, priority, color, startDate, endDate)
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			epic, err := app.Services.Epics.Create(ctx, workspaceID, req)
			if err != nil {
				return err
			}
			if epic == nil || epic.ID == "" {
				return errors.New("the backend returned an epic without an ID")
			}
			app.success("Created epic %s (%s).", epic.Title, epic.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "epic title")
	cmd.Flags().StringVar(&description, "description", "", "epic description")
	cmd.Flags().StringVar(&status, "status", "planned", "planned, active, completed, or archived")
	cmd.Flags().StringVar(&priority, "priority", "medium", "low, medium, high, or critical")
	cmd.Flags().StringVar(&color, "color", "#6D7CFF", "epic color")
	cmd.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	return cmd
}

func newEpicUpdateCommand(app *App) *cobra.Command {
	var title, description, status, priority, color, startDate, endDate string
	cmd := &cobra.Command{
		Use:   "update <epic-id>",
		Short: "Update an epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Epic updates"); err != nil {
				return err
			}
			epicID := strings.TrimSpace(args[0])
			if err := validation.ID("epic ID", epicID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			flags := cmd.Flags()
			changed := false
			for _, name := range []string{"title", "description", "status", "priority", "color", "start-date", "end-date"} {
				changed = changed || flags.Changed(name)
			}
			if !changed {
				return errors.New("at least one epic field must be provided")
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			current, err := app.Services.Epics.Get(ctx, workspaceID, epicID)
			if err != nil {
				return err
			}
			if current == nil {
				return fmt.Errorf("epic %q was not returned by the backend", epicID)
			}
			req := api.EpicMutationRequest{
				Title: current.Title, Description: current.Description,
				Status: current.Status, Priority: current.Priority, Color: current.Color,
				StartDate: dateRequestValue(current.StartDate), EndDate: dateRequestValue(current.DueDate),
			}
			if flags.Changed("title") {
				req.Title = title
			}
			if flags.Changed("description") {
				req.Description = description
			}
			if flags.Changed("status") {
				req.Status = status
			}
			if flags.Changed("priority") {
				req.Priority = priority
			}
			if flags.Changed("color") {
				req.Color = color
			}
			if flags.Changed("start-date") {
				req.StartDate = startDate
			}
			if flags.Changed("end-date") {
				req.EndDate = endDate
			}
			req, err = validateEpicRequest(req)
			if err != nil {
				return err
			}
			updated, err := app.Services.Epics.Update(ctx, workspaceID, epicID, req)
			if err != nil {
				return err
			}
			if updated == nil {
				return errors.New("the backend did not return the updated epic")
			}
			app.success("Updated epic %s.", updated.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&status, "status", "", "new status")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority")
	cmd.Flags().StringVar(&color, "color", "", "new color")
	cmd.Flags().StringVar(&startDate, "start-date", "", "new start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "new end date (YYYY-MM-DD)")
	return cmd
}

func newEpicDeleteCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <epic-id>",
		Short: "Delete an epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Epic deletion"); err != nil {
				return err
			}
			epicID := strings.TrimSpace(args[0])
			if err := validation.ID("epic ID", epicID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			if !yes {
				if !app.Interactive {
					return errors.New("refusing to delete an epic without confirmation; re-run with --yes")
				}
				confirm := prompts.NewConfirm(app.Styles, "Delete epic", fmt.Sprintf("Delete epic %s?", epicID))
				confirm.Destructive = true
				confirmed, err := prompts.RunConfirm(cmd.Context(), confirm)
				if err != nil {
					return err
				}
				if !confirmed {
					app.muted("Deletion cancelled.")
					return nil
				}
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			if err := app.Services.Epics.Delete(ctx, workspaceID, epicID); err != nil {
				return err
			}
			app.success("Deleted epic %s.", epicID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func epicRequestFromFlags(title, description, status, priority, color, startDate, endDate string) (api.EpicMutationRequest, error) {
	req := api.EpicMutationRequest{
		Title: strings.TrimSpace(title), Description: strings.TrimSpace(description),
		Status: status, Priority: priority, Color: color, StartDate: startDate, EndDate: endDate,
	}
	return validateEpicRequest(req)
}

func validateEpicRequest(req api.EpicMutationRequest) (api.EpicMutationRequest, error) {
	if err := validation.TaskTitle("title", req.Title); err != nil {
		return req, err
	}
	var err error
	if req.Status, err = validation.Enum("status", req.Status, epicStatuses); err != nil {
		return req, err
	}
	if req.Priority, err = validation.Enum("priority", req.Priority, epicPriorities); err != nil {
		return req, err
	}
	if len(req.Color) > 30 {
		return req, errors.New("epic color cannot exceed 30 characters")
	}
	if req.StartDate, err = validatedDate("start date", req.StartDate); err != nil {
		return req, err
	}
	if req.EndDate, err = validatedDate("end date", req.EndDate); err != nil {
		return req, err
	}
	return req, nil
}

func validatedDate(field, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	parsed, err := validation.Date(field, value)
	if err != nil {
		return "", err
	}
	return parsed.Format("2006-01-02"), nil
}

func dateRequestValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func newSprintCreateCommand(app *App) *cobra.Command {
	var name, goal, startDate, endDate string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a planned sprint",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint creation"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			req, err := sprintRequestFromFlags(name, goal, startDate, endDate)
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			sprint, err := app.Services.Sprints.Create(ctx, workspaceID, req)
			if err != nil {
				return err
			}
			if sprint == nil || sprint.ID == "" {
				return errors.New("the backend returned a sprint without an ID")
			}
			app.success("Created sprint %s (%s).", sprint.Name, sprint.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "sprint name")
	cmd.Flags().StringVar(&goal, "goal", "", "sprint goal")
	cmd.Flags().StringVar(&startDate, "start-date", "", "start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "end date (YYYY-MM-DD)")
	return cmd
}

func newSprintUpdateCommand(app *App) *cobra.Command {
	var name, goal, startDate, endDate string
	cmd := &cobra.Command{
		Use:   "update <sprint-id>",
		Short: "Update a planned sprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint updates"); err != nil {
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
			flags := cmd.Flags()
			if !flags.Changed("name") && !flags.Changed("goal") && !flags.Changed("start-date") && !flags.Changed("end-date") {
				return errors.New("at least one sprint field must be provided")
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			current, err := app.Services.Sprints.Get(ctx, workspaceID, sprintID)
			if err != nil {
				return err
			}
			if current == nil {
				return fmt.Errorf("sprint %q was not returned by the backend", sprintID)
			}
			req := api.SprintMutationRequest{
				Name: current.Name, Goal: current.Goal,
				StartDate: dateRequestValue(current.StartDate), EndDate: dateRequestValue(current.EndDate),
			}
			if flags.Changed("name") {
				req.Name = name
			}
			if flags.Changed("goal") {
				req.Goal = goal
			}
			if flags.Changed("start-date") {
				req.StartDate = startDate
			}
			if flags.Changed("end-date") {
				req.EndDate = endDate
			}
			req, err = validateSprintRequest(req)
			if err != nil {
				return err
			}
			updated, err := app.Services.Sprints.Update(ctx, workspaceID, sprintID, req)
			if err != nil {
				return err
			}
			if updated == nil {
				return errors.New("the backend did not return the updated sprint")
			}
			app.success("Updated sprint %s.", updated.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new sprint name")
	cmd.Flags().StringVar(&goal, "goal", "", "new sprint goal")
	cmd.Flags().StringVar(&startDate, "start-date", "", "new start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "new end date (YYYY-MM-DD)")
	return cmd
}

func newSprintStartCommand(app *App) *cobra.Command {
	return sprintTransitionCommand(app, "start", "Start a planned sprint", func(ctx context.Context, ws, id string) (*api.Sprint, error) {
		return app.Services.Sprints.Start(ctx, ws, id)
	})
}

func newSprintCompleteCommand(app *App) *cobra.Command {
	return sprintTransitionCommand(app, "complete", "Complete an active sprint", func(ctx context.Context, ws, id string) (*api.Sprint, error) {
		return app.Services.Sprints.Complete(ctx, ws, id)
	})
}

func newSprintReopenCommand(app *App) *cobra.Command {
	return sprintTransitionCommand(app, "reopen", "Reopen a completed sprint", func(ctx context.Context, ws, id string) (*api.Sprint, error) {
		return app.Services.Sprints.Reopen(ctx, ws, id)
	})
}

func sprintTransitionCommand(app *App, verb, short string, action func(context.Context, string, string) (*api.Sprint, error)) *cobra.Command {
	return &cobra.Command{
		Use:   verb + " <sprint-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint " + verb); err != nil {
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
			sprint, err := action(ctx, workspaceID, sprintID)
			if err != nil {
				return err
			}
			if sprint == nil {
				return errors.New("the backend did not return the transitioned sprint")
			}
			app.success("%s sprint %s; status is %s.", capitalize(verb), sprint.ID, sprint.Status)
			return nil
		},
	}
}

func newSprintDeleteCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <sprint-id>",
		Short: "Delete a planned sprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint deletion"); err != nil {
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
			if !yes {
				if !app.Interactive {
					return errors.New("refusing to delete a sprint without confirmation; re-run with --yes")
				}
				confirm := prompts.NewConfirm(app.Styles, "Delete sprint", fmt.Sprintf("Delete sprint %s?", sprintID))
				confirm.Destructive = true
				confirmed, err := prompts.RunConfirm(cmd.Context(), confirm)
				if err != nil {
					return err
				}
				if !confirmed {
					app.muted("Deletion cancelled.")
					return nil
				}
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			if err := app.Services.Sprints.Delete(ctx, workspaceID, sprintID); err != nil {
				return err
			}
			app.success("Deleted sprint %s.", sprintID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func sprintRequestFromFlags(name, goal, startDate, endDate string) (api.SprintMutationRequest, error) {
	req := api.SprintMutationRequest{
		Name: strings.TrimSpace(name), Goal: strings.TrimSpace(goal),
		StartDate: startDate, EndDate: endDate,
	}
	return validateSprintRequest(req)
}

func validateSprintRequest(req api.SprintMutationRequest) (api.SprintMutationRequest, error) {
	if err := validation.TaskTitle("sprint name", req.Name); err != nil {
		return req, err
	}
	var err error
	if req.StartDate, err = validatedDate("start date", req.StartDate); err != nil {
		return req, err
	}
	if req.EndDate, err = validatedDate("end date", req.EndDate); err != nil {
		return req, err
	}
	if req.StartDate != "" && req.EndDate != "" && req.StartDate > req.EndDate {
		return req, errors.New("sprint start date must not be after end date")
	}
	return req, nil
}

func newSprintBacklogCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "backlog",
		Short: "Display unplanned user stories in the selected workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint backlog"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			stories, err := app.Services.Sprints.ListBacklog(ctx, workspaceID)
			if err != nil {
				return err
			}
			if len(stories) == 0 {
				app.empty("The sprint backlog is empty.")
				return nil
			}
			rows := make([][]string, 0, len(stories))
			for _, story := range stories {
				rows = append(rows, storyRow(story))
			}
			app.table([]string{"ID", "TITLE", "STATUS", "PRIORITY", "POINTS", "POSITION"}, rows)
			return nil
		},
	}
}

func newSprintAddStoryCommand(app *App) *cobra.Command {
	return sprintStoryCommand(app, "add-story <sprint-id> <story-id>", "Add a user story to a planned sprint", func(ctx context.Context, ws, sprintID, storyID string) error {
		return app.Services.Sprints.AddStory(ctx, ws, sprintID, storyID)
	})
}

func newSprintRemoveStoryCommand(app *App) *cobra.Command {
	return sprintStoryCommand(app, "remove-story <sprint-id> <story-id>", "Remove a user story from a planned sprint", func(ctx context.Context, ws, sprintID, storyID string) error {
		return app.Services.Sprints.RemoveStory(ctx, ws, sprintID, storyID)
	})
}

func sprintStoryCommand(app *App, use, short string, action func(context.Context, string, string, string) error) *cobra.Command {
	verb := strings.Fields(use)[0]
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint backlog"); err != nil {
				return err
			}
			sprintID, storyID := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
			if err := validation.ID("sprint ID", sprintID); err != nil {
				return err
			}
			if err := validation.ID("story ID", storyID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			if err := action(ctx, workspaceID, sprintID, storyID); err != nil {
				return err
			}
			if verb == "add-story" {
				app.success("Added user story %s to sprint %s.", storyID, sprintID)
			} else {
				app.success("Removed user story %s from sprint %s.", storyID, sprintID)
			}
			return nil
		},
	}
}

func capitalize(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func newSprintReorderStoryCommand(app *App) *cobra.Command {
	var position int
	cmd := &cobra.Command{
		Use:   "reorder-story <story-id>",
		Short: "Set a user story's sprint-backlog position",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint backlog ordering"); err != nil {
				return err
			}
			storyID := strings.TrimSpace(args[0])
			if err := validation.ID("story ID", storyID); err != nil {
				return err
			}
			if position < 0 {
				return errors.New("backlog position cannot be negative")
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			if err := app.Services.Sprints.ReorderBacklogStory(ctx, workspaceID, storyID, position); err != nil {
				return err
			}
			app.success("Moved user story %s to backlog position %d.", storyID, position)
			return nil
		},
	}
	cmd.Flags().IntVar(&position, "position", 0, "zero-based backlog position")
	return cmd
}

func newSprintBoardCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "board <sprint-id>",
		Short: "Display the user-story board for a sprint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Sprint board"); err != nil {
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
			stories, report, err := app.Services.Sprints.Board(ctx, workspaceID, sprintID)
			if err != nil {
				return err
			}
			app.title(fmt.Sprintf("Sprint board · %s", sprintID))
			app.keyValues([][2]string{
				{"Stories", fmt.Sprintf("%d", report.TotalStories)},
				{"Completed", fmt.Sprintf("%d", report.CompletedStories)},
				{"Points", fmt.Sprintf("%d", report.CommittedStoryPoints)},
				{"Completion", fmt.Sprintf("%.1f%%", report.CompletionRate)},
			}, 12)
			if len(stories) == 0 {
				app.empty("This sprint has no user stories.")
				return nil
			}
			rows := make([][]string, 0, len(stories))
			for _, story := range stories {
				rows = append(rows, storyRow(story))
			}
			app.table([]string{"ID", "TITLE", "STATUS", "PRIORITY", "POINTS", "POSITION"}, rows)
			return nil
		},
	}
}
