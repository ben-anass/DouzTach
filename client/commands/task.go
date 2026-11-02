package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/prompts"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
	"github.com/ben-anass/DouzTach/client/validation"
	"github.com/spf13/cobra"
)

func newTaskCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "List, create, show, update, and delete tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTaskListCommand(app),
		newTaskCreateCommand(app),
		newTaskShowCommand(app),
		newTaskUpdateCommand(app),
		newTaskDeleteCommand(app),
	)
	return cmd
}

func newTaskListCommand(app *App) *cobra.Command {
	var (
		statusFilter   string
		priorityFilter string
		typeFilter     string
		search         string
		sprintID       string
		epicID         string
		assigneeID     string
		mine           bool
		backlog        bool
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks in the selected workspace",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Tasks"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}
			filter := api.TaskFilter{
				WorkspaceID: workspaceID,
				SprintID:    strings.TrimSpace(sprintID),
				EpicID:      strings.TrimSpace(epicID),
				AssigneeID:  strings.TrimSpace(assigneeID),
				Search:      strings.TrimSpace(search),
				BacklogOnly: backlog,
				Mine:        mine,
			}
			if statusFilter != "" {
				status, err := api.ParseTaskStatus(statusFilter)
				if err != nil {
					return err
				}
				filter.Status = &status
			}
			if priorityFilter != "" {
				priority, err := api.ParsePriority(priorityFilter)
				if err != nil {
					return err
				}
				filter.Priority = &priority
			}
			if typeFilter != "" {
				taskType, err := api.ParseTaskType(typeFilter)
				if err != nil {
					return err
				}
				filter.Type = &taskType
			}
			if filter.SprintID != "" {
				if err := validation.ID("sprint ID", filter.SprintID); err != nil {
					return err
				}
			}
			if filter.EpicID != "" {
				if err := validation.ID("epic ID", filter.EpicID); err != nil {
					return err
				}
			}
			if filter.AssigneeID != "" {
				if err := validation.ID("assignee ID", filter.AssigneeID); err != nil {
					return err
				}
			}
			if mine {
				if app.Session == nil || app.Session.User.ID == "" {
					return errors.New("--mine needs a signed-in user with an ID reported by the backend")
				}
				filter.CurrentUserID = app.Session.User.ID
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			tasks, err := app.Services.Tasks.List(ctx, filter)
			if err != nil {
				return err
			}
			matched := make([]api.Task, 0, len(tasks))
			for _, task := range tasks {
				if filter.Matches(task) {
					matched = append(matched, task)
				}
			}
			if len(matched) == 0 {
				app.empty("No tasks match these filters.")
				return nil
			}
			sortTasks(matched)
			rows := make([][]string, 0, len(matched))
			for _, task := range matched {
				rows = append(rows, taskRow(task))
			}
			app.table([]string{"KEY", "TITLE", "STATUS", "PRIORITY", "TYPE", "ASSIGNEE", "DUE"}, rows)
			return nil
		},
	}
	cmd.Flags().StringVar(&statusFilter, "status", "", "filter by TODO, IN_PROGRESS, REVIEW, or DONE")
	cmd.Flags().StringVar(&priorityFilter, "priority", "", "filter by LOW, MEDIUM, HIGH, or URGENT")
	cmd.Flags().StringVar(&typeFilter, "type", "", "filter by TASK, BUG, STORY, EPIC, or SUBTASK")
	cmd.Flags().StringVar(&search, "search", "", "search keys, titles, and descriptions")
	cmd.Flags().StringVar(&sprintID, "sprint", "", "filter by sprint ID")
	cmd.Flags().StringVar(&epicID, "epic", "", "filter by epic ID")
	cmd.Flags().StringVar(&assigneeID, "assignee", "", "filter by assignee ID")
	cmd.Flags().BoolVar(&mine, "mine", false, "only tasks assigned to you")
	cmd.Flags().BoolVar(&backlog, "backlog", false, "only tasks without a sprint")
	return cmd
}

func newTaskCreateCommand(app *App) *cobra.Command {
	var (
		title       string
		description string
		taskType    string
		priority    string
		assigneeID  string
		sprintID    string
		epicID      string
		due         string
		points      string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a task in the selected workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Task creation"); err != nil {
				return err
			}
			workspaceID, err := app.workspaceID()
			if err != nil {
				return err
			}

			values := map[string]string{
				"title":        title,
				"description":  description,
				"type":         taskType,
				"priority":     priority,
				"due_date":     due,
				"story_points": points,
			}
			if strings.TrimSpace(title) == "" {
				if !app.Interactive {
					return errors.New("task title is required; pass --title")
				}
				prompted, err := prompts.RunForm(cmd.Context(), prompts.TaskForm(app.Styles))
				if err != nil {
					return err
				}
				for key, value := range prompted {
					values[key] = value
				}
				if err := chooseTaskAttributes(cmd.Context(), app, cmd, values, workspaceID); err != nil {
					return err
				}
			}
			if err := validation.TaskTitle("title", values["title"]); err != nil {
				return err
			}
			if err := validation.Description("description", values["description"]); err != nil {
				return err
			}

			taskTypeValue, err := parseTaskTypeValue(values["type"])
			if err != nil {
				return err
			}
			priorityValue, err := parsePriorityValue(values["priority"])
			if err != nil {
				return err
			}
			dueDate, err := parseDueDateValue(values["due_date"])
			if err != nil {
				return err
			}
			storyPoints, err := parseStoryPointsValue(values["story_points"])
			if err != nil {
				return err
			}
			if candidate := strings.TrimSpace(values["sprint"]); candidate != "" {
				sprintID = candidate
			}
			if err := validateOptionalID("assignee ID", assigneeID); err != nil {
				return err
			}
			if err := validateOptionalID("sprint ID", sprintID); err != nil {
				return err
			}
			if err := validateOptionalID("epic ID", epicID); err != nil {
				return err
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			task, err := app.Services.Tasks.Create(ctx, api.CreateTaskRequest{
				WorkspaceID: workspaceID,
				Title:       strings.TrimSpace(values["title"]),
				Description: strings.TrimSpace(values["description"]),
				Type:        taskTypeValue,
				Priority:    priorityValue,
				AssigneeID:  strings.TrimSpace(assigneeID),
				SprintID:    strings.TrimSpace(sprintID),
				EpicID:      strings.TrimSpace(epicID),
				DueDate:     dueDate,
				StoryPoints: storyPoints,
			})
			if err != nil {
				return err
			}
			if task == nil || strings.TrimSpace(task.ID) == "" {
				return errors.New("the backend returned a task without an ID")
			}
			app.success("Created %s %s.", dash(task.Key), task.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "task title")
	cmd.Flags().StringVar(&description, "description", "", "task description")
	cmd.Flags().StringVar(&taskType, "type", string(api.TaskTypeTask), "TASK, BUG, STORY, EPIC, or SUBTASK")
	cmd.Flags().StringVar(&priority, "priority", string(api.PriorityMedium), "LOW, MEDIUM, HIGH, or URGENT")
	cmd.Flags().StringVar(&assigneeID, "assignee", "", "assignee user ID")
	cmd.Flags().StringVar(&sprintID, "sprint", "", "sprint ID")
	cmd.Flags().StringVar(&epicID, "epic", "", "epic ID")
	cmd.Flags().StringVar(&due, "due", "", "due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&points, "points", "", "story points (0-100)")
	return cmd
}

func newTaskShowCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show a task's details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Tasks"); err != nil {
				return err
			}
			taskID := strings.TrimSpace(args[0])
			if err := validation.ID("task ID", taskID); err != nil {
				return err
			}
			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			task, err := app.Services.Tasks.Get(ctx, taskID)
			if err != nil {
				return err
			}
			if task == nil {
				return fmt.Errorf("task %q was not returned by the backend", taskID)
			}
			fmt.Fprintln(app.Out, screens.TaskDetail(app.screenContext(88), *task))
			return nil
		},
	}
}

func newTaskUpdateCommand(app *App) *cobra.Command {
	var (
		title       string
		description string
		status      string
		priority    string
		assigneeID  string
		sprintID    string
		epicID      string
		due         string
		points      string
	)
	cmd := &cobra.Command{
		Use:   "update <task-id>",
		Short: "Update a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Task updates"); err != nil {
				return err
			}
			taskID := strings.TrimSpace(args[0])
			if err := validation.ID("task ID", taskID); err != nil {
				return err
			}
			values := map[string]string{
				"title":       title,
				"description": description,
				"status":      status,
				"priority":    priority,
				"due_date":    due,
			}
			changed := map[string]bool{
				"title":       cmd.Flags().Changed("title"),
				"description": cmd.Flags().Changed("description"),
				"status":      cmd.Flags().Changed("status"),
				"priority":    cmd.Flags().Changed("priority"),
				"due_date":    cmd.Flags().Changed("due"),
				"assignee":    cmd.Flags().Changed("assignee"),
				"sprint":      cmd.Flags().Changed("sprint"),
				"epic":        cmd.Flags().Changed("epic"),
				"points":      cmd.Flags().Changed("points"),
			}
			anyChange := false
			for _, isChanged := range changed {
				if isChanged {
					anyChange = true
					break
				}
			}
			if !anyChange {
				if !app.Interactive {
					return errors.New("no updates were provided; pass at least one flag")
				}
				prompted, err := prompts.RunForm(cmd.Context(), prompts.TaskUpdateForm(app.Styles))
				if err != nil {
					return err
				}
				for key, value := range prompted {
					if strings.TrimSpace(value) != "" {
						values[key] = value
						changed[key] = true
						anyChange = true
					}
				}
				if err := chooseTaskUpdate(cmd.Context(), app, cmd, values, changed); err != nil {
					return err
				}
				if !anyChange {
					anyChange = changed["status"] || changed["priority"]
				}
				if !anyChange {
					return errors.New("no updates were provided")
				}
			}

			var request api.UpdateTaskRequest
			if changed["title"] {
				if err := validation.TaskTitle("title", values["title"]); err != nil {
					return err
				}
				value := strings.TrimSpace(values["title"])
				request.Title = &value
			}
			if changed["description"] {
				if err := validation.Description("description", values["description"]); err != nil {
					return err
				}
				value := strings.TrimSpace(values["description"])
				request.Description = &value
			}
			if changed["status"] {
				value, err := api.ParseTaskStatus(values["status"])
				if err != nil {
					return err
				}
				request.Status = &value
			}
			if changed["priority"] {
				value, err := api.ParsePriority(values["priority"])
				if err != nil {
					return err
				}
				request.Priority = &value
			}
			if changed["due_date"] {
				value, err := parseDueDateValue(values["due_date"])
				if err != nil {
					return err
				}
				request.DueDate = value
			}
			if changed["assignee"] {
				if err := validateOptionalID("assignee ID", assigneeID); err != nil {
					return err
				}
				value := strings.TrimSpace(assigneeID)
				request.AssigneeID = &value
			}
			if changed["sprint"] {
				if err := validateOptionalID("sprint ID", sprintID); err != nil {
					return err
				}
				value := strings.TrimSpace(sprintID)
				request.SprintID = &value
			}
			if changed["epic"] {
				if err := validateOptionalID("epic ID", epicID); err != nil {
					return err
				}
				value := strings.TrimSpace(epicID)
				request.EpicID = &value
			}
			if changed["points"] {
				value, err := parseStoryPointsValue(points)
				if err != nil {
					return err
				}
				request.StoryPoints = value
			}

			ctx, cancel := app.requestContext(cmd.Context())
			defer cancel()
			task, err := app.Services.Tasks.Update(ctx, taskID, request)
			if err != nil {
				return err
			}
			if task == nil {
				return errors.New("the backend returned no task after the update")
			}
			app.success("Updated %s %s.", dash(task.Key), task.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&status, "status", "", "new status: TODO, IN_PROGRESS, REVIEW, or DONE")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority: LOW, MEDIUM, HIGH, or URGENT")
	cmd.Flags().StringVar(&assigneeID, "assignee", "", "new assignee ID (empty clears the assignee)")
	cmd.Flags().StringVar(&sprintID, "sprint", "", "new sprint ID (empty clears the sprint)")
	cmd.Flags().StringVar(&epicID, "epic", "", "new epic ID (empty clears the epic)")
	cmd.Flags().StringVar(&due, "due", "", "new due date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&points, "points", "", "new story points (0-100)")
	return cmd
}

func newTaskDeleteCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <task-id>",
		Short: "Delete a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.requireReady("Task deletion"); err != nil {
				return err
			}
			taskID := strings.TrimSpace(args[0])
			if err := validation.ID("task ID", taskID); err != nil {
				return err
			}
			if !yes {
				if !app.Interactive {
					return errors.New("refusing to delete a task without confirmation; re-run with --yes")
				}
				confirm := prompts.NewConfirm(app.Styles, "Delete task",
					fmt.Sprintf("Delete task %s? This cannot be undone.", taskID))
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
			if err := app.Services.Tasks.Delete(ctx, taskID); err != nil {
				return err
			}
			app.success("Deleted task %s.", taskID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func taskRow(task api.Task) []string {
	return []string{
		dash(task.Key),
		cell(task.Title, 44),
		task.Status.Title(),
		task.Priority.Label(),
		task.Type.Label(),
		cell(task.AssigneeName(), 20),
		formatDate(task.DueDate),
	}
}

func sortTasks(tasks []api.Task) {
	statusRank := map[api.TaskStatus]int{
		api.TaskStatusTodo:       0,
		api.TaskStatusInProgress: 1,
		api.TaskStatusReview:     2,
		api.TaskStatusDone:       3,
	}
	priorityRank := map[api.Priority]int{
		api.PriorityUrgent: 0,
		api.PriorityHigh:   1,
		api.PriorityMedium: 2,
		api.PriorityLow:    3,
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left, right := tasks[i], tasks[j]
		if statusRank[left.Status] != statusRank[right.Status] {
			return statusRank[left.Status] < statusRank[right.Status]
		}
		if priorityRank[left.Priority] != priorityRank[right.Priority] {
			return priorityRank[left.Priority] < priorityRank[right.Priority]
		}
		return strings.ToLower(left.Title) < strings.ToLower(right.Title)
	})
}

func parseTaskTypeValue(value string) (api.TaskType, error) {
	if strings.TrimSpace(value) == "" {
		return api.TaskTypeTask, nil
	}
	return api.ParseTaskType(value)
}

func parsePriorityValue(value string) (api.Priority, error) {
	if strings.TrimSpace(value) == "" {
		return api.PriorityMedium, nil
	}
	return api.ParsePriority(value)
}

func parseDueDateValue(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := validation.Date("due date", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseStoryPointsValue(value string) (*int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	points, err := strconv.Atoi(value)
	if err != nil {
		return nil, errors.New("story points must be a whole number")
	}
	if points < 0 || points > 100 {
		return nil, errors.New("story points must be between 0 and 100")
	}
	return &points, nil
}

// chooseTaskUpdate offers keyboard pickers for the enum fields of an edit. Each
// picker starts on "leave unchanged" so an edit only sends what the user chose.
func chooseTaskUpdate(ctx context.Context, app *App, cmd *cobra.Command, values map[string]string, changed map[string]bool) error {
	if !cmd.Flags().Changed("status") && strings.TrimSpace(values["status"]) == "" {
		option, err := prompts.RunSelect(ctx, prompts.NewSelectSelected(app.Styles,
			"Status", keepOption("Leave the status unchanged", prompts.TaskStatusOptions()), ""))
		if err != nil {
			return err
		}
		if option.Value != "" {
			values["status"] = option.Value
			changed["status"] = true
		}
	}
	if !cmd.Flags().Changed("priority") && strings.TrimSpace(values["priority"]) == "" {
		option, err := prompts.RunSelect(ctx, prompts.NewSelectSelected(app.Styles,
			"Priority", keepOption("Leave the priority unchanged", prompts.PriorityOptions()), ""))
		if err != nil {
			return err
		}
		if option.Value != "" {
			values["priority"] = option.Value
			changed["priority"] = true
		}
	}
	return nil
}

// keepOption prepends a "no change" choice so an edit picker can be skipped
// without cancelling the whole flow.
func keepOption(label string, options []prompts.Option) []prompts.Option {
	return append([]prompts.Option{{Label: label, Value: ""}}, options...)
}

// chooseTaskAttributes lets an interactive user pick the task type, priority,
// and sprint with keyboard prompts. Values supplied as flags are never
// overridden, so scripted invocations stay predictable.
func chooseTaskAttributes(ctx context.Context, app *App, cmd *cobra.Command, values map[string]string, workspaceID string) error {
	if !cmd.Flags().Changed("type") {
		option, err := prompts.RunSelect(ctx,
			prompts.NewSelectSelected(app.Styles, "Task type", prompts.TaskTypeOptions(), values["type"]))
		if err != nil {
			return err
		}
		values["type"] = option.Value
	}
	if !cmd.Flags().Changed("priority") {
		option, err := prompts.RunSelect(ctx,
			prompts.NewSelectSelected(app.Styles, "Priority", prompts.PriorityOptions(), values["priority"]))
		if err != nil {
			return err
		}
		values["priority"] = option.Value
	}
	if cmd.Flags().Changed("sprint") || !app.Services.Ready() {
		return nil
	}
	sprints, err := app.Services.Sprints.List(ctx, workspaceID)
	if err != nil {
		app.warn("could not load sprints; the task will be created without a sprint: %v", err)
		return nil
	}
	if len(sprints) == 0 {
		return nil
	}
	options := append([]prompts.Option{{Label: "No sprint (backlog)", Value: ""}}, prompts.SprintOptions(sprints)...)
	option, err := prompts.RunSelect(ctx,
		prompts.NewSelectSelected(app.Styles, "Sprint", options, ""))
	if err != nil {
		return err
	}
	if option.Value != "" {
		values["sprint"] = option.Value
	}
	return nil
}

func validateOptionalID(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return validation.ID(field, value)
}
