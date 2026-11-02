package screens

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/terminal/tables"
)

// MyTasks renders tasks assigned to the signed-in user.
func MyTasks(c Context) string {
	header, handled := Gate(c, "My Tasks", "Work assigned to you in "+c.WorkspaceName()+".", "My Tasks", "")
	if handled {
		return header
	}
	tasks := MineTasks(c)
	if len(tasks) == 0 {
		return header + "\n\n" + Empty(c,
			"No tasks assigned to you",
			"The backend returned no tasks for your account in this workspace.",
			"Create a task with 'douztech task create' or assign one to yourself.",
		)
	}
	return header + "\n\n" + tables.RenderSelectableTasks(c.Styles, tasks, c.Width, c.Now, c.Cursor)
}

// MineTasks returns tasks assigned to the signed-in user.
func MineTasks(c Context) []api.Task {
	filter := api.TaskFilter{Mine: true, CurrentUserID: c.CurrentUserID()}
	var tasks []api.Task
	for _, task := range c.Data.Tasks {
		if filter.Matches(task) {
			tasks = append(tasks, task)
		}
	}
	if len(tasks) > 0 {
		return tasks
	}
	// Some auth contracts may not return a user ID. Fall back to email matching
	// only when an email is actually known; never guess.
	email := ""
	if c.Session != nil {
		email = strings.TrimSpace(c.Session.User.Email)
	}
	if email == "" {
		return nil
	}
	for _, task := range c.Data.Tasks {
		if task.Assignee != nil && strings.EqualFold(task.Assignee.Email, email) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// BacklogTasks returns tasks that are not assigned to a sprint.
func BacklogTasks(c Context) []api.Task {
	filter := api.TaskFilter{BacklogOnly: true}
	tasks := make([]api.Task, 0, len(c.Data.Tasks))
	for _, task := range c.Data.Tasks {
		if filter.Matches(task) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// Backlog renders tasks that are not assigned to a sprint.
func Backlog(c Context) string {
	header, handled := Gate(c, "Backlog", "Unscheduled work waiting to be planned.", "Backlog", "")
	if handled {
		return header
	}
	tasks := BacklogTasks(c)
	if len(tasks) == 0 {
		return header + "\n\n" + Empty(c,
			"Backlog is empty",
			"Every task in this workspace is assigned to a sprint, or no tasks exist yet.",
			"Tasks without a sprint appear here.",
		)
	}
	return header + "\n\n" + tables.RenderSelectableTasks(c.Styles, tasks, c.Width, c.Now, c.Cursor)
}

// SprintBoard renders the active sprint as a kanban board.
func SprintBoard(c Context) string {
	header, handled := Gate(c, "Sprint Board", "Kanban view of the active sprint.", "Sprint Board", "")
	if handled {
		return header
	}
	sprint, ok := activeSprint(c.Data.Sprints)
	if !ok {
		return header + "\n\n" + Empty(c,
			"No active sprint",
			"There is no sprint with status ACTIVE in this workspace.",
			"Start a sprint with 'douztech sprint list' once the backend is connected.",
		)
	}
	tasks := make([]api.Task, 0, len(c.Data.Tasks))
	for _, task := range c.Data.Tasks {
		if task.SprintID == sprint.ID {
			tasks = append(tasks, task)
		}
	}
	subtitle := sprint.Name
	if sprint.Goal != "" {
		subtitle += " · " + sprint.Goal
	}
	header = Header(c, "Sprint Board", subtitle)
	if len(tasks) == 0 {
		return header + "\n\n" + Empty(c,
			"No tasks in this sprint",
			"The active sprint has no tasks yet.",
			"Move backlog tasks into the sprint once the backend is connected.",
		)
	}
	return header + "\n\n" + renderBoard(c, tasks)
}

func renderBoard(c Context, tasks []api.Task) string {
	columns := make([]string, 0, len(api.TaskStatuses))
	for _, status := range api.TaskStatuses {
		columns = append(columns, renderBoardColumn(c, status, tasks))
	}
	if c.Width < 96 {
		return strings.Join(columns, "\n\n")
	}
	const gap = 1
	columnWidth := (c.Width - gap*(len(columns)-1)) / len(columns)
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, components.Panel(c.Styles, "", column, columnWidth))
	}
	return strings.Join(parts, strings.Repeat(" ", gap))
}

func renderBoardColumn(c Context, status api.TaskStatus, tasks []api.Task) string {
	var matching []api.Task
	for _, task := range tasks {
		if task.Status == status {
			matching = append(matching, task)
		}
	}
	var b strings.Builder
	b.WriteString(c.Styles.StatusStyle(status).Render(status.Label()))
	b.WriteString(c.Styles.Muted.Render("  " + components.IntString(len(matching))))
	b.WriteString("\n")
	b.WriteString(c.Styles.DividerString(max(6, min(28, c.Width/4-2))))
	if len(matching) == 0 {
		b.WriteString("\n")
		b.WriteString(c.Styles.Muted.Render("No tasks"))
		return b.String()
	}
	for _, task := range matching {
		b.WriteString("\n")
		title := task.Title
		if task.Key != "" {
			title = task.Key + " · " + title
		}
		b.WriteString(c.Styles.Text.Render(components.Truncate(title, max(8, min(28, c.Width/4-2)))))
		b.WriteString("\n")
		b.WriteString(c.Styles.PriorityStyle(task.Priority).Render(task.Priority.Label()))
		b.WriteString(c.Styles.Muted.Render(" · " + components.Truncate(task.AssigneeName(), 12)))
	}
	return b.String()
}

// TaskDetail renders the full detail body for a task modal.
func TaskDetail(c Context, task api.Task) string {
	var b strings.Builder
	title := task.Title
	if task.Key != "" {
		title = task.Key + " · " + title
	}
	b.WriteString(c.Styles.SectionTitle.Render(title))
	b.WriteString("\n\n")
	b.WriteString(detailRow(c, "Status", c.Styles.StatusStyle(task.Status).Render(task.Status.Label())))
	b.WriteString(detailRow(c, "Priority", c.Styles.PriorityStyle(task.Priority).Render(task.Priority.Label())))
	b.WriteString(detailRow(c, "Type", c.Styles.TypeStyle(task.Type).Render(task.Type.Label())))
	b.WriteString(detailRow(c, "Assignee", c.Styles.Text.Render(task.AssigneeName())))
	b.WriteString(detailRow(c, "Due date", c.Styles.Text.Render(components.FormatDate(task.DueDate))))
	if task.StoryPoints != nil {
		b.WriteString(detailRow(c, "Story points", c.Styles.Text.Render(components.IntString(*task.StoryPoints))))
	}
	if task.SprintID != "" {
		b.WriteString(detailRow(c, "Sprint ID", c.Styles.Muted.Render(task.SprintID)))
	}
	if task.EpicID != "" {
		b.WriteString(detailRow(c, "Epic ID", c.Styles.Muted.Render(task.EpicID)))
	}
	if task.Description != "" {
		b.WriteString("\n")
		b.WriteString(c.Styles.CardTitle.Render("DESCRIPTION"))
		b.WriteString("\n")
		b.WriteString(c.Styles.Text.Render(components.Wrap(task.Description, max(20, min(66, c.Width-8)))))
	}
	b.WriteString("\n\n")
	b.WriteString(c.Styles.Help.Render("esc close · enter close"))
	return b.String()
}

func detailRow(c Context, label, value string) string {
	return c.Styles.Muted.Render(components.PadRight(label, 13)) + value + "\n"
}
