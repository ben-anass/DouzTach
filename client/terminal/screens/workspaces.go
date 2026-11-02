package screens

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/components"
)

// Workspaces renders the workspace list and marks the selected workspace.
func Workspaces(c Context) string {
	header, handled := Gate(c, "Workspaces", "Choose the workspace you are working in.", "Workspaces", "")
	if handled {
		return header
	}
	if len(c.Data.Workspaces) == 0 {
		return header + "\n\n" + Empty(c,
			"No workspaces yet",
			"The backend returned no workspaces for your account.",
			"Create one with 'douztech workspace create' once the backend is connected.",
		)
	}
	selectedID := c.WorkspaceID()
	lines := make([]string, 0, len(c.Data.Workspaces)*2)
	for i, workspace := range c.Data.Workspaces {
		cursor := "  "
		if i == c.Cursor {
			cursor = c.Styles.Info.Render("› ")
		}
		marker := "  "
		nameStyle := c.Styles.Text
		if workspace.ID == selectedID {
			marker = c.Styles.Success.Render("● ")
			nameStyle = c.Styles.Selected
		}
		line := cursor + marker + nameStyle.Render(workspace.Name)
		meta := make([]string, 0, 3)
		if workspace.Role != "" {
			meta = append(meta, "role: "+workspace.Role)
		}
		if workspace.MemberCount > 0 {
			meta = append(meta, components.IntString(workspace.MemberCount)+" "+components.Plural(workspace.MemberCount, "member", "members"))
		}
		if len(meta) > 0 {
			line += c.Styles.Muted.Render("   " + strings.Join(meta, " · "))
		}
		lines = append(lines, line)
		if workspace.Description != "" {
			lines = append(lines, c.Styles.Muted.Render("      "+components.Truncate(workspace.Description, max(10, c.Width-8))))
		}
	}
	lines = append(lines, "", c.Styles.Help.Render("enter switch workspace  ·  c create workspace (requires backend)"))
	return header + "\n\n" + strings.Join(lines, "\n")
}

// WorkspaceCursor returns the workspace at the given index, if any.
func WorkspaceCursor(c Context, index int) (api.Workspace, bool) {
	if index < 0 || index >= len(c.Data.Workspaces) {
		return api.Workspace{}, false
	}
	return c.Data.Workspaces[index], true
}
