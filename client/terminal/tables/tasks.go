package tables

import (
	"strconv"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// TaskColumns returns the responsive column set for the given width. Columns
// are dropped as the terminal narrows, with the title always remaining.
func TaskColumns(width int) []Column {
	id := Column{Title: "ID", MinWidth: 7, MaxWidth: 14, Weight: 0}
	title := Column{Title: "Title", MinWidth: 14, Weight: 6}
	taskType := Column{Title: "Type", MinWidth: 7, MaxWidth: 10, Weight: 1}
	status := Column{Title: "Status", MinWidth: 11, MaxWidth: 13, Weight: 0}
	priority := Column{Title: "Priority", MinWidth: 8, MaxWidth: 10, Weight: 0}
	assignee := Column{Title: "Assignee", MinWidth: 10, MaxWidth: 20, Weight: 2}
	due := Column{Title: "Due", MinWidth: 11, MaxWidth: 13, Weight: 0}

	switch {
	case width >= 104:
		return []Column{id, title, taskType, status, priority, assignee, due}
	case width >= 88:
		return []Column{id, title, status, priority, assignee, due}
	case width >= 70:
		return []Column{id, title, status, priority, due}
	case width >= 52:
		return []Column{id, title, status, due}
	case width >= 36:
		return []Column{title, status}
	default:
		return []Column{title}
	}
}

// TaskTable builds a styled task table for the available width.
func TaskTable(s styles.Styles, tasks []api.Task, width int, now time.Time) Table {
	columns := TaskColumns(width)
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		cells := make([]string, 0, len(columns))
		for _, column := range columns {
			cells = append(cells, taskCell(s, task, column.Title, now))
		}
		rows = append(rows, cells)
	}
	return Table{Columns: columns, Rows: rows, PreStyled: true}
}

// RenderTasks renders the task table with a header. Callers are responsible
// for showing an empty state when tasks is empty.
func RenderTasks(s styles.Styles, tasks []api.Task, width int, now time.Time) string {
	if len(tasks) == 0 {
		return ""
	}
	return TaskTable(s, tasks, width, now).Render(s, width)
}

// RenderSelectableTasks renders the task table with one highlighted row so the
// keyboard cursor is visible in the terminal UI.
func RenderSelectableTasks(s styles.Styles, tasks []api.Task, width int, now time.Time, selected int) string {
	if len(tasks) == 0 {
		return ""
	}
	return TaskTable(s, tasks, width, now).WithSelection(selected).Render(s, width)
}

func taskCell(s styles.Styles, task api.Task, column string, now time.Time) string {
	switch column {
	case "ID":
		return s.Muted.Render(taskIdentifier(task))
	case "Title":
		return s.Text.Render(task.Title)
	case "Type":
		return s.TypeStyle(task.Type).Render(task.Type.Label())
	case "Status":
		return s.StatusStyle(task.Status).Render(task.Status.Label())
	case "Priority":
		return s.PriorityStyle(task.Priority).Render(task.Priority.Label())
	case "Assignee":
		return s.Text.Render(task.AssigneeName())
	case "Due":
		due := components.FormatDate(task.DueDate)
		if task.IsOverdue(now) {
			return s.Error.Render("! " + due)
		}
		return s.Muted.Render(due)
	default:
		return ""
	}
}

func taskIdentifier(task api.Task) string {
	if task.Key != "" {
		return task.Key
	}
	if task.ID != "" {
		return task.ID
	}
	return "—"
}

// BacklogColumns returns columns for the backlog view.
func BacklogColumns(width int) []Column {
	return TaskColumns(width)
}

// TaskCountLabel renders "n tasks" with correct pluralisation.
func TaskCountLabel(count int) string {
	return strconv.Itoa(count) + " " + components.Plural(count, "task", "tasks")
}

// AlignRight is a convenience alias used by callers building custom tables.
func AlignRight() lipgloss.Position { return lipgloss.Right }
