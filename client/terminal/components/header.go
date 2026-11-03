package components

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// HeaderData is the information shown in the application header.
type HeaderData struct {
	Version   string
	Status    api.ServiceStatus
	SignedIn  bool
	UserName  string
	Workspace string
}

// StatusIndicator renders a coloured connection indicator.
func StatusIndicator(s styles.Styles, status api.ServiceStatus) string {
	dot := "○"
	style := s.Muted
	switch {
	case status.ContractReady && status.Reachable:
		dot = "●"
		style = s.Success
	case status.Reachable:
		dot = "◐"
		style = s.Warning
	case status.Configured:
		dot = "○"
		style = s.Error
	}
	return style.Render(dot + " " + status.Summary())
}

// RenderHeader renders the top application bar and a divider.
func RenderHeader(s styles.Styles, data HeaderData, width int) string {
	if width <= 0 {
		return ""
	}
	version := strings.TrimSpace(data.Version)
	app := s.Logo.Render("DouzTach")
	if version != "" {
		app += " " + s.AppSubtitle.Render("v"+strings.TrimPrefix(version, "v"))
	}
	status := StatusIndicator(s, data.Status)
	lineOne := RightAlign(app, status, width)

	var userPart string
	if data.SignedIn {
		name := strings.TrimSpace(data.UserName)
		if name == "" {
			name = "session active"
		}
		userPart = s.Muted.Render("Signed in as ") + s.Text.Render(name)
	} else {
		userPart = s.Muted.Render("Not signed in")
	}

	workspace := strings.TrimSpace(data.Workspace)
	if workspace == "" {
		workspace = "no workspace selected"
	}
	workspacePart := s.Muted.Render("Workspace: ") + s.Text.Render(workspace)

	var lineTwo string
	if width >= 78 {
		lineTwo = userPart + s.Muted.Render("   ·   ") + workspacePart
	} else {
		lineTwo = userPart
	}
	lineTwo = Truncate(lineTwo, width)

	divider := s.DividerString(width)
	return lipgloss.JoinVertical(lipgloss.Left, lineOne, lineTwo, divider)
}
