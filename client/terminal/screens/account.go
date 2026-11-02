package screens

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/components"
)

// Profile renders the signed-in user's cached identity. It never displays the
// access token.
func Profile(c Context) string {
	header := Header(c, "Profile", "Your account and session on this device.")
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	if c.Session == nil {
		b.WriteString(Empty(c,
			"Not signed in",
			"No session token is stored on this device.",
			"Run 'douztech login' once the backend authentication contract is available.",
		))
		return b.String()
	}

	user := c.Session.User
	name := user.DisplayName()
	email := user.Email
	username := user.Username
	id := user.ID
	if c.Data.Profile != nil {
		name = c.Data.Profile.DisplayName()
		email = c.Data.Profile.Email
		username = c.Data.Profile.Username
		id = c.Data.Profile.ID
	}
	if name == "Unknown user" {
		name = "Not reported by the backend"
	}
	rows := [][2]string{
		{"Name", name},
		{"Email", valueOrDash(email)},
		{"Username", valueOrDash(username)},
		{"User ID", valueOrDash(id)},
		{"Workspace", c.WorkspaceName()},
	}
	for _, row := range rows {
		b.WriteString(c.Styles.Muted.Render(components.PadRight(row[0], 12)))
		b.WriteString(c.Styles.Text.Render(row[1]))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(SectionLabel(c, "session"))
	b.WriteString("\n")
	source := "saved session file"
	if c.Session.Ephemeral {
		source = "DOUZTACH_TOKEN environment variable (never written to disk)"
	}
	b.WriteString(detailRow(c, "Source", c.Styles.Text.Render(source)))
	b.WriteString(detailRow(c, "Expires", c.Styles.Text.Render(c.Session.ExpiresIn(c.Now))))
	b.WriteString(detailRow(c, "Token", c.Styles.Muted.Render("[REDACTED — tokens are never printed]")))
	b.WriteString("\n")
	b.WriteString(c.Styles.Help.Render("Sign out from the Logout item in the sidebar or with 'douztech logout'."))
	return b.String()
}

// Settings renders the local client configuration and backend status.
func Settings(c Context) string {
	header := Header(c, "Settings", "Client configuration for this installation.")
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	apiURL := c.Config.APIBaseURL
	if apiURL == "" {
		apiURL = "not configured"
	}
	workspace := c.Config.WorkspaceID
	if workspace == "" {
		workspace = "not selected"
	}
	rows := [][2]string{
		{"API URL", apiURL},
		{"Timeout", c.Config.Timeout.String()},
		{"Workspace", workspace},
		{"Config dir", valueOrDash(c.ConfigDir)},
	}
	for _, row := range rows {
		b.WriteString(c.Styles.Muted.Render(components.PadRight(row[0], 12)))
		b.WriteString(c.Styles.Text.Render(row[1]))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(SectionLabel(c, "backend"))
	b.WriteString("\n")
	status := c.Services.Status
	b.WriteString(detailRow(c, "Status", components.StatusIndicator(c.Styles, status)))
	b.WriteString(detailRow(c, "Detail", c.Styles.Muted.Render(valueOrDash(status.Detail))))
	if !status.CheckedAt.IsZero() {
		b.WriteString(detailRow(c, "Checked", c.Styles.Muted.Render(components.RelativeTime(status.CheckedAt, c.Now))))
	}

	b.WriteString("\n")
	b.WriteString(SectionLabel(c, "change settings"))
	b.WriteString("\n")
	commands := []string{
		"douztech settings set api-url <url>",
		"douztech settings set timeout 30s",
		"douztech settings set workspace <id>",
		"douztech setup",
	}
	for _, command := range commands {
		b.WriteString(c.Styles.Key.Render("  " + command))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(c.Styles.Help.Render("config.json is written with owner-only permissions. Tokens are stored separately in session.json and are never shown."))
	return b.String()
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
