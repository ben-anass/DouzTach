package terminal

import (
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model.
func (a *App) View() string {
	width := max(20, a.Width)
	height := max(10, a.Height)

	header := components.RenderHeader(a.Styles, components.HeaderData{
		Version:   a.Version,
		Status:    a.Services.Status,
		SignedIn:  a.Session != nil,
		UserName:  a.userName(),
		Workspace: a.workspaceName(),
	}, width)
	headerHeight := lipgloss.Height(header)

	footer := components.RenderFooter(a.Styles, a.footerHints(), width)
	footerHeight := lipgloss.Height(footer)

	message := a.renderMessage(width)
	messageHeight := 0
	if message != "" {
		messageHeight = 1
	}

	bodyHeight := height - headerHeight - footerHeight - messageHeight - 2
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	var body string
	switch {
	case a.ShowHelp:
		body = a.renderHelp(width, bodyHeight)
	case a.ConfirmLogout:
		body = a.renderLogoutConfirm(width, bodyHeight)
	case a.SelectedTask != nil:
		body = a.renderTaskModal(width, bodyHeight)
	default:
		body = a.renderMain(width, bodyHeight)
	}
	body = components.Clamp(body, width, bodyHeight)

	parts := []string{header, "", body}
	if message != "" {
		parts = append(parts, message)
	}
	parts = append(parts, "", footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (a *App) renderMain(width, height int) string {
	if width < 58 {
		top := a.renderTopNav(width)
		contentHeight := max(1, height-lipgloss.Height(top)-1)
		return lipgloss.JoinVertical(lipgloss.Left, top, "", a.renderScreen(width, contentHeight))
	}
	sidebarWidth := a.sidebarWidth()
	contentWidth := width - sidebarWidth - 2
	if contentWidth < 20 {
		contentWidth = 20
	}
	sidebar := a.renderSidebar(sidebarWidth, height)
	content := a.renderScreen(contentWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", content)
}

func (a *App) sidebarWidth() int {
	if a.Width >= 110 {
		return 24
	}
	return 21
}

func (a *App) renderSidebar(width, height int) string {
	inner := max(12, width-3) // right border (1) + horizontal padding (2)
	lines := []string{a.Styles.CardTitle.Render("NAVIGATION"), ""}
	activeIndex := a.Screen.Index()
	if a.ConfirmLogout {
		activeIndex = len(NavItems) - 1
	}
	for i, item := range NavItems {
		if item.Action {
			lines = append(lines, "")
		}
		label := item.Title
		shortcut := item.Shortcut
		line := ""
		switch {
		case i == activeIndex:
			line = a.Styles.Selected.Render("▸ " + label)
		case a.Focus == FocusNav && i == a.NavCursor:
			line = a.Styles.Info.Render("› " + label)
		default:
			line = a.Styles.Muted.Render("  " + label)
		}
		gap := inner - lipgloss.Width(line) - lipgloss.Width(shortcut)
		if gap < 1 {
			gap = 1
		}
		line += strings.Repeat(" ", gap) + a.Styles.Help.Render(shortcut)
		lines = append(lines, components.Truncate(line, inner))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	content := strings.Join(lines, "\n")
	return a.Styles.Sidebar.Render(content)
}

func (a *App) renderTopNav(width int) string {
	position := fmt.Sprintf("%d/%d", a.Screen.Index()+1, len(NavItems))
	left := a.Styles.Selected.Render(" " + a.Screen.Title() + " ")
	right := a.Styles.Muted.Render("[" + position + "]  [ ] screens")
	return components.RightAlign(left, right, width)
}

func (a *App) renderScreen(width, height int) string {
	c := a.context(width)
	var content string
	switch a.Screen {
	case ScreenDashboard:
		content = screens.Dashboard(c)
	case ScreenMyTasks:
		content = screens.MyTasks(c)
	case ScreenWorkspaces:
		content = screens.Workspaces(c)
	case ScreenBacklog:
		content = screens.Backlog(c)
	case ScreenSprintBoard:
		content = screens.SprintBoard(c)
	case ScreenEpics:
		content = screens.Epics(c)
	case ScreenNotifications:
		content = screens.Notifications(c)
	case ScreenActivity:
		content = screens.Activity(c)
	case ScreenChat:
		content = screens.Chat(c)
	case ScreenProfile:
		content = screens.Profile(c)
	case ScreenSettings:
		content = screens.Settings(c)
	default:
		content = screens.Dashboard(c)
	}
	return a.scrollAndClamp(content, width, height)
}

func (a *App) scrollAndClamp(content string, width, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	maxScroll := max(0, len(lines)-height)
	offset := min(a.ContentScroll, maxScroll)
	lines = lines[offset:]
	return components.Clamp(strings.Join(lines, "\n"), width, height)
}

func (a *App) renderHelp(width, height int) string {
	return components.RenderModal(a.Styles, components.Modal{
		Title: "Keyboard help",
		Body:  helpText(),
		Width: min(78, width-4),
		Actions: []components.ModalAction{
			{Label: "Close", Key: "esc"},
		},
	}, width, height)
}

func (a *App) renderTaskModal(width, height int) string {
	if a.SelectedTask == nil {
		return ""
	}
	c := a.context(min(76, width-8))
	return components.RenderModal(a.Styles, components.Modal{
		Title: "Task details",
		Body:  screens.TaskDetail(c, *a.SelectedTask),
		Width: min(80, width-4),
		Actions: []components.ModalAction{
			{Label: "Close", Key: "esc"},
		},
	}, width, height)
}

func (a *App) renderLogoutConfirm(width, height int) string {
	return components.RenderConfirm(a.Styles, components.Confirm{
		Title:       "Sign out",
		Question:    "Sign out of DouzTach on this device?",
		Detail:      "The saved session token will be removed. You can sign in again at any time.",
		ConfirmKey:  "y",
		CancelKey:   "n",
		Destructive: true,
	}, width, height)
}

func (a *App) renderMessage(width int) string {
	message := strings.TrimSpace(a.Message)
	if message == "" {
		return ""
	}
	message = components.Truncate(message, width)
	if a.MessageIsError {
		return a.Styles.Error.Render("! " + message)
	}
	return a.Styles.Success.Render("✓ " + message)
}

func (a *App) footerHints() []components.KeyHint {
	if a.ShowHelp {
		return []components.KeyHint{{Key: "esc", Description: "close help"}}
	}
	if a.ConfirmLogout {
		return []components.KeyHint{
			{Key: "y", Description: "confirm sign out"},
			{Key: "n", Description: "cancel"},
		}
	}
	if a.SelectedTask != nil {
		return []components.KeyHint{{Key: "esc", Description: "close task details"}}
	}
	if a.Focus == FocusNav {
		hints := []components.KeyHint{
			{Key: "↑/↓", Description: "navigate"},
			{Key: "enter", Description: "open"},
			{Key: "tab", Description: "content"},
			{Key: "?", Description: "help"},
			{Key: "q", Description: "quit"},
		}
		if a.Width < 58 {
			hints = append(hints, components.KeyHint{Key: "[ ]", Description: "screens"})
		}
		return hints
	}
	hints := []components.KeyHint{
		{Key: "esc", Description: "navigation"},
		{Key: "r", Description: "refresh"},
		{Key: "?", Description: "help"},
		{Key: "q", Description: "quit"},
	}
	switch a.Screen {
	case ScreenMyTasks, ScreenBacklog:
		hints = append([]components.KeyHint{
			{Key: "↑/↓", Description: "select"},
			{Key: "enter", Description: "details"},
		}, hints...)
	case ScreenWorkspaces:
		hints = append([]components.KeyHint{
			{Key: "↑/↓", Description: "select"},
			{Key: "enter", Description: "switch"},
		}, hints...)
	case ScreenNotifications:
		hints = append([]components.KeyHint{
			{Key: "↑/↓", Description: "select"},
			{Key: "enter", Description: "mark read"},
		}, hints...)
	}
	if a.Width < 58 {
		hints = append(hints, components.KeyHint{Key: "[ ]", Description: "screens"})
	}
	return hints
}

func (a *App) userName() string {
	if a.Session != nil {
		if name := a.Session.User.DisplayName(); name != "" && name != "Unknown user" {
			return name
		}
	}
	if a.Data.Profile != nil {
		return a.Data.Profile.DisplayName()
	}
	return ""
}

func (a *App) workspaceName() string {
	c := a.context(a.contentWidth())
	return c.WorkspaceName()
}

func (a *App) contentWidth() int {
	width := max(20, a.Width)
	if width < 58 {
		return width
	}
	content := width - a.sidebarWidth() - 2
	if content < 20 {
		return 20
	}
	return content
}
