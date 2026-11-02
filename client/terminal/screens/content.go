package screens

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
)

// Epics renders the epic list with progress bars.
func Epics(c Context) string {
	header, handled := Gate(c, "Epics", "Larger bodies of work that group related tasks.", "Epics", "")
	if handled {
		return header
	}
	if len(c.Data.Epics) == 0 {
		return header + "\n\n" + Empty(c,
			"No epics yet",
			"The backend returned no epics for this workspace.",
			"Epics will appear here once they are created.",
		)
	}
	barWidth := max(8, min(24, c.Width/3))
	lines := make([]string, 0, len(c.Data.Epics)*3)
	for _, epic := range c.Data.Epics {
		title := epic.Title
		if epic.Key != "" {
			title = epic.Key + " · " + title
		}
		line := c.Styles.Text.Render(components.Truncate(title, max(10, c.Width-20)))
		if epic.Status != "" {
			line += c.Styles.Muted.Render("  " + epic.Status)
		}
		lines = append(lines, line)
		progress := components.RenderProgress(c.Styles, epic.Progress(), barWidth)
		counts := components.IntString(epic.DoneCount) + "/" + components.IntString(epic.TaskCount) + " tasks"
		lines = append(lines, "  "+progress+c.Styles.Muted.Render("  "+counts))
		if epic.DueDate != nil {
			lines = append(lines, "  "+c.Styles.Muted.Render("due "+components.FormatDate(epic.DueDate)))
		}
		lines = append(lines, "")
	}
	return header + "\n\n" + strings.Join(lines, "\n")
}

// Notifications renders the notification inbox.
func Notifications(c Context) string {
	header, handled := Gate(c, "Notifications", "Inbox, mentions, and assignment updates.", "Notifications", "")
	if handled {
		return header
	}
	if len(c.Data.Notifications) == 0 {
		return header + "\n\n" + Empty(c,
			"You are all caught up",
			"The backend returned no notifications for your account.",
			"New notifications will appear here.",
		)
	}
	unread := 0
	lines := make([]string, 0, len(c.Data.Notifications)*3)
	for i, notification := range c.Data.Notifications {
		cursor := "  "
		if i == c.Cursor {
			cursor = c.Styles.Info.Render("› ")
		}
		marker := "  "
		titleStyle := c.Styles.Text
		if !notification.Read {
			unread++
			marker = c.Styles.Info.Render("● ")
			titleStyle = c.Styles.Selected
		}
		line := cursor + marker + titleStyle.Render(notification.Title)
		if notification.Type != "" {
			line += c.Styles.Muted.Render("  " + notification.Type)
		}
		line += c.Styles.Help.Render("  " + components.RelativeTime(notification.CreatedAt, c.Now))
		lines = append(lines, line)
		if notification.Body != "" {
			lines = append(lines, c.Styles.Muted.Render("      "+components.Truncate(notification.Body, max(10, c.Width-8))))
		}
	}
	summary := c.Styles.Muted.Render(components.IntString(unread) + " unread · " +
		components.IntString(len(c.Data.Notifications)) + " total")
	return header + "\n\n" + summary + "\n\n" + strings.Join(lines, "\n")
}

// NotificationCursor returns the notification at the given index, if any.
func NotificationCursor(c Context, index int) (api.Notification, bool) {
	if index < 0 || index >= len(c.Data.Notifications) {
		return api.Notification{}, false
	}
	return c.Data.Notifications[index], true
}

// Activity renders the workspace activity feed.
func Activity(c Context) string {
	header, handled := Gate(c, "Activity", "A chronological history of workspace changes.", "Activity history", "")
	if handled {
		return header
	}
	if len(c.Data.Activities) == 0 {
		return header + "\n\n" + Empty(c,
			"No activity yet",
			"The backend returned no activity entries for this workspace.",
			"Task, sprint, and workspace changes will appear here.",
		)
	}
	lines := make([]string, 0, len(c.Data.Activities))
	for _, entry := range c.Data.Activities {
		actor := entry.ActorName
		if actor == "" {
			actor = "Someone"
		}
		line := c.Styles.Help.Render(components.RelativeTime(entry.CreatedAt, c.Now))
		line += "  " + c.Styles.Text.Render(actor)
		if entry.Action != "" {
			line += " " + c.Styles.Muted.Render(entry.Action)
		}
		if entry.Subject != "" {
			line += " " + c.Styles.Text.Render(entry.Subject)
		}
		lines = append(lines, line)
	}
	return header + "\n\n" + strings.Join(lines, "\n")
}

// Chat renders the available channels and states the realtime limitation.
func Chat(c Context) string {
	header, handled := Gate(c, "Chat", "Team conversations, channels, and direct messages.", "Chat", "")
	if handled {
		return header
	}
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	if len(c.Data.Channels) == 0 {
		b.WriteString(Empty(c,
			"No chat channels",
			"The backend returned no channels for this workspace.",
			"Channel creation and messaging require the chat API contract.",
		))
		return b.String()
	}
	lines := make([]string, 0, len(c.Data.Channels))
	for _, channel := range c.Data.Channels {
		line := c.Styles.Text.Render("# " + channel.Name)
		if channel.MemberCount > 0 {
			line += c.Styles.Muted.Render("  " + components.IntString(channel.MemberCount) + " members")
		}
		lines = append(lines, line)
		if channel.Description != "" {
			lines = append(lines, c.Styles.Muted.Render("    "+components.Truncate(channel.Description, max(10, c.Width-6))))
		}
	}
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n\n")
	b.WriteString(components.Panel(c.Styles, "Messages",
		c.Styles.Muted.Render("Message history and realtime delivery are not connected yet. The websocket contract is still pending, so no messages are simulated."),
		c.Width,
	))
	return b.String()
}
