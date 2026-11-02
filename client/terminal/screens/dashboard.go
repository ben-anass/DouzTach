package screens

import (
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
)

// Dashboard renders the main overview.
func Dashboard(c Context) string {
	var b strings.Builder
	b.WriteString(Header(c, "Dashboard", "A clear view of "+c.WorkspaceName()+"."))
	b.WriteString("\n")

	b.WriteString(c.Styles.Muted.Render("Signed in as "))
	b.WriteString(c.Styles.Text.Render(c.CurrentUserName()))
	b.WriteString(c.Styles.Muted.Render("   ·   Workspace: "))
	b.WriteString(c.Styles.Text.Render(c.WorkspaceName()))
	b.WriteString("\n")

	switch {
	case !c.Ready():
		b.WriteString("\n")
		b.WriteString(Pending(c, "Dashboard data",
			"The dashboard will show live task counts, the active sprint, and recent activity once Member 2 publishes the API contract."))
		b.WriteString("\n")
		b.WriteString(RenderMetrics(c, api.TaskCounts{}, false))
		return b.String()
	case c.Loading:
		b.WriteString("\n")
		b.WriteString(c.Styles.Muted.Render("Loading dashboard data…"))
		return b.String()
	case c.Err != nil:
		b.WriteString("\n")
		b.WriteString(ErrorState(c, c.Err))
		return b.String()
	}

	counts := api.CountTasks(c.Data.Tasks, c.Now)
	b.WriteString("\n")
	b.WriteString(RenderMetrics(c, counts, true))
	b.WriteString("\n")

	activeSprint, _ := activeSprint(c.Data.Sprints)
	sprintBody := renderActiveSprint(c, activeSprint)
	activityBody := renderRecentActivity(c, c.Data.Activities, 5)
	b.WriteString(components.PanelRow(c.Styles,
		"Active sprint", sprintBody,
		"Recent activity", activityBody,
		c.Width,
	))
	if hint := FooterHint(c, c.LoadedAt); hint != "" {
		b.WriteString("\n")
		b.WriteString(hint)
	}
	return b.String()
}

// RenderMetrics renders the six dashboard counters. When connected is false,
// values are shown as em dashes because no data was loaded.
func RenderMetrics(c Context, counts api.TaskCounts, connected bool) string {
	value := func(n int) string {
		if !connected {
			return "—"
		}
		return components.IntString(n)
	}
	hint := ""
	if !connected {
		hint = "not connected"
	}
	cards := []components.StatCard{
		{Label: "Total tasks", Value: value(counts.Total), Hint: hint, Color: c.Styles.Theme.Primary},
		{Label: "To do", Value: value(counts.Todo), Hint: hint, Color: c.Styles.Theme.Muted},
		{Label: "In progress", Value: value(counts.InProgress), Hint: hint, Color: c.Styles.Theme.Secondary},
		{Label: "Review", Value: value(counts.Review), Hint: hint, Color: c.Styles.Theme.Warning},
		{Label: "Completed", Value: value(counts.Done), Hint: hint, Color: c.Styles.Theme.Success},
		{Label: "Overdue", Value: value(counts.Overdue), Hint: hint, Color: c.Styles.Theme.Danger},
	}
	return components.RenderStatCards(c.Styles, cards, c.Width)
}

func activeSprint(sprints []api.Sprint) (api.Sprint, bool) {
	for _, sprint := range sprints {
		if sprint.Status == api.SprintStatusActive {
			return sprint, true
		}
	}
	return api.Sprint{}, false
}

func renderActiveSprint(c Context, sprint api.Sprint) string {
	if sprint.ID == "" {
		return c.Styles.Muted.Render("No active sprint.")
	}
	var b strings.Builder
	b.WriteString(c.Styles.Value.Render(sprint.Name))
	b.WriteString("\n")
	if sprint.Goal != "" {
		b.WriteString(c.Styles.Muted.Render(components.Wrap(sprint.Goal, max(10, c.Width/2-6))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	progress := sprint.Progress()
	b.WriteString(components.RenderProgress(c.Styles, progress, max(8, min(30, c.Width/2-10))))
	b.WriteString("\n")
	b.WriteString(c.Styles.Muted.Render(
		components.IntString(sprint.DoneCount) + " of " + components.IntString(sprint.TaskCount) + " tasks done",
	))
	if sprint.EndDate != nil {
		b.WriteString(c.Styles.Muted.Render(" · ends " + components.FormatDate(sprint.EndDate)))
	}
	return b.String()
}

func renderRecentActivity(c Context, activities []api.Activity, limit int) string {
	if len(activities) == 0 {
		return c.Styles.Muted.Render("No activity recorded yet.")
	}
	if limit > 0 && len(activities) > limit {
		activities = activities[:limit]
	}
	lines := make([]string, 0, len(activities))
	for _, activity := range activities {
		actor := activity.ActorName
		if actor == "" {
			actor = "Someone"
		}
		line := c.Styles.Text.Render(actor)
		if activity.Action != "" {
			line += " " + c.Styles.Muted.Render(activity.Action)
		}
		if activity.Subject != "" {
			line += " " + c.Styles.Text.Render(components.Truncate(activity.Subject, max(8, c.Width/2-18)))
		}
		line += " " + c.Styles.Help.Render(components.RelativeTime(activity.CreatedAt, c.Now))
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// FormatUpdated renders a timestamp for screen footers.
func FormatUpdated(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("15:04:05")
}
