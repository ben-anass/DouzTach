package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

var storyStatuses = []string{"to_do", "in_progress", "in_review", "done", "blocked"}
var storyPriorities = []string{"low", "medium", "high", "critical"}

func newStoryCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "story",
		Short: "Create and manage user stories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newStoryCreateCommand(app), newStoryListCommand(app), newStoryShowCommand(app), newStoryUpdateCommand(app), newStoryDeleteCommand(app))
	return cmd
}

func newStoryListCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List user stories in the selected workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("User stories"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			stories, err := app.Services.UserStories.List(ctx, workspaceID)
			if err != nil {
				return err
			}
			if len(stories) == 0 {
				app.empty("No user stories were returned for this workspace.")
				return nil
			}
			rows := make([][]string, 0, len(stories))
			for _, story := range stories {
				rows = append(rows, storyRow(story))
			}
			app.table([]string{"ID", "TITLE", "STATUS", "PRIORITY", "POINTS", "BACKLOG"}, rows)
			return nil
		},
	}
}

func newStoryCreateCommand(app *App) *cobra.Command {
	var title, description, acceptance, epicID, status, priority, assigneeID string
	var points int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user story in the selected workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("User story creation"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			title = strings.TrimSpace(title)
			if err := validation.TaskTitle("title", title); err != nil {
				return err
			}
			if err := validation.Description("description", description); err != nil {
				return err
			}
			if status == "" {
				status = "to_do"
			}
			if status, err = validation.Enum("status", status, storyStatuses); err != nil {
				return err
			}
			if priority == "" {
				priority = "medium"
			}
			if priority, err = validation.Enum("priority", priority, storyPriorities); err != nil {
				return err
			}
			if epicID != "" {
				if err := validation.ID("epic ID", epicID); err != nil {
					return err
				}
			}
			if assigneeID != "" {
				if err := validation.ID("assignee ID", assigneeID); err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("points") && points < 0 {
				return errors.New("story points must be between 0 and 100")
			}
			pointsValue, err := storyPoints(points)
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			story, err := app.Services.UserStories.Create(ctx, workspaceID, api.CreateUserStoryRequest{
				EpicID: epicID, Title: title, Description: description,
				AcceptanceCriteria: acceptance, Status: status, Priority: priority,
				AssigneeID: assigneeID, StoryPoints: pointsValue,
			})
			if err != nil {
				return err
			}
			if story == nil || story.ID == "" {
				return errors.New("the backend returned a user story without an ID")
			}
			app.success("Created user story %s (%s).", story.Title, story.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "user-story title")
	cmd.Flags().StringVar(&description, "description", "", "user-story description")
	cmd.Flags().StringVar(&acceptance, "acceptance-criteria", "", "acceptance criteria")
	cmd.Flags().StringVar(&epicID, "epic", "", "parent epic ID")
	cmd.Flags().StringVar(&status, "status", "", "to_do, in_progress, in_review, done, or blocked")
	cmd.Flags().StringVar(&priority, "priority", "", "low, medium, high, or critical")
	cmd.Flags().StringVar(&assigneeID, "assignee", "", "assignee user ID")
	cmd.Flags().IntVar(&points, "points", -1, "story points (0-100)")
	return cmd
}

func newStoryShowCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <story-id>",
		Short: "Show a user story",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("User stories"); err != nil {
				return err
			}
			storyID := strings.TrimSpace(args[0])
			if err := validation.ID("story ID", storyID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			story, err := app.Services.UserStories.Get(ctx, workspaceID, storyID)
			if err != nil {
				return err
			}
			if story == nil {
				return fmt.Errorf("user story %q was not returned by the backend", storyID)
			}
			app.title(story.Title)
			app.keyValues([][2]string{
				{"ID", story.ID},
				{"Epic", dash(story.EpicID)},
				{"Status", story.Status},
				{"Priority", story.Priority},
				{"Story points", storyPointText(story.StoryPoints)},
				{"Backlog position", strconv.Itoa(story.BacklogPosition)},
				{"Description", dash(story.Description)},
				{"Acceptance criteria", dash(story.AcceptanceCriteria)},
			}, 22)
			return nil
		},
	}
}

func newStoryUpdateCommand(app *App) *cobra.Command {
	var title, description, acceptance, epicID, status, priority, assigneeID string
	var points int
	cmd := &cobra.Command{
		Use:   "update <story-id>",
		Short: "Update a user story",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("User story updates"); err != nil {
				return err
			}
			storyID := strings.TrimSpace(args[0])
			if err := validation.ID("story ID", storyID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			flags := cmd.Flags()
			changed := flags.Changed("title") || flags.Changed("description") ||
				flags.Changed("acceptance-criteria") || flags.Changed("epic") ||
				flags.Changed("status") || flags.Changed("priority") ||
				flags.Changed("assignee") || flags.Changed("points")
			if !changed {
				return errors.New("at least one user-story field must be provided")
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			current, err := app.Services.UserStories.Get(ctx, workspaceID, storyID)
			if err != nil {
				return err
			}
			if current == nil {
				return fmt.Errorf("user story %q was not returned by the backend", storyID)
			}
			req := api.CreateUserStoryRequest{
				EpicID: current.EpicID, Title: current.Title, Description: current.Description,
				AcceptanceCriteria: current.AcceptanceCriteria, Status: current.Status,
				Priority: current.Priority, AssigneeID: current.AssigneeID,
				StoryPoints: current.StoryPoints,
			}
			if flags.Changed("title") {
				req.Title = strings.TrimSpace(title)
			}
			if flags.Changed("description") {
				req.Description = description
			}
			if flags.Changed("acceptance-criteria") {
				req.AcceptanceCriteria = acceptance
			}
			if flags.Changed("epic") {
				req.EpicID = epicID
			}
			if flags.Changed("status") {
				req.Status = status
			}
			if flags.Changed("priority") {
				req.Priority = priority
			}
			if flags.Changed("assignee") {
				req.AssigneeID = assigneeID
			}
			if flags.Changed("points") {
				req.StoryPoints, err = storyPoints(points)
				if err != nil {
					return err
				}
			}
			if err := validation.TaskTitle("title", req.Title); err != nil {
				return err
			}
			if req.Status, err = validation.Enum("status", req.Status, storyStatuses); err != nil {
				return err
			}
			if req.Priority, err = validation.Enum("priority", req.Priority, storyPriorities); err != nil {
				return err
			}
			if req.EpicID != "" {
				if err := validation.ID("epic ID", req.EpicID); err != nil {
					return err
				}
			}
			if req.AssigneeID != "" {
				if err := validation.ID("assignee ID", req.AssigneeID); err != nil {
					return err
				}
			}
			updated, err := app.Services.UserStories.Update(ctx, workspaceID, storyID, req)
			if err != nil {
				return err
			}
			if updated == nil {
				return errors.New("the backend did not return the updated user story")
			}
			app.success("Updated user story %s.", updated.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&acceptance, "acceptance-criteria", "", "new acceptance criteria")
	cmd.Flags().StringVar(&epicID, "epic", "", "new epic ID")
	cmd.Flags().StringVar(&status, "status", "", "new story status")
	cmd.Flags().StringVar(&priority, "priority", "", "new story priority")
	cmd.Flags().StringVar(&assigneeID, "assignee", "", "new assignee ID")
	cmd.Flags().IntVar(&points, "points", -1, "new story points (0-100)")
	return cmd
}

func newStoryDeleteCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <story-id>",
		Short: "Delete a user story",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("User story deletion"); err != nil {
				return err
			}
			storyID := strings.TrimSpace(args[0])
			if err := validation.ID("story ID", storyID); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			if !yes {
				if !app.Interactive {
					return errors.New("refusing to delete a user story without confirmation; re-run with --yes")
				}
				confirm := prompts.NewConfirm(app.Styles, "Delete user story",
					fmt.Sprintf("Delete user story %s? This cannot be undone.", storyID))
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
			if err := app.Services.UserStories.Delete(ctx, workspaceID, storyID); err != nil {
				return err
			}
			app.success("Deleted user story %s.", storyID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func storyRow(story api.UserStory) []string {
	return []string{
		cell(story.ID, 24),
		cell(story.Title, 40),
		story.Status,
		story.Priority,
		storyPointText(story.StoryPoints),
		strconv.Itoa(story.BacklogPosition),
	}
}

func storyPoints(points int) (*int, error) {
	if points == -1 {
		return nil, nil
	}
	if points < 0 || points > 100 {
		return nil, errors.New("story points must be between 0 and 100")
	}
	return &points, nil
}

func storyPointText(points *int) string {
	if points == nil {
		return "—"
	}
	return strconv.Itoa(*points)
}
