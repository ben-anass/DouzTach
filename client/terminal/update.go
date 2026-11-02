package terminal

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/screens"
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.Width = msg.Width
		a.Height = msg.Height
		if a.Width < 20 {
			a.Width = 20
		}
		if a.Height < 10 {
			a.Height = 10
		}
		a.clampCursors()
		return a, nil
	case dataMsg:
		a.Loading = false
		a.Data = msg.Data
		a.Err = msg.Err
		a.LoadedAt = msg.LoadedAt
		a.Now = a.clock()
		a.clampCursors()
		return a, nil
	case actionMsg:
		if msg.Err != nil {
			a.setMessage("Action failed: "+msg.Err.Error(), true)
			return a, nil
		}
		message := msg.Message
		if strings.TrimSpace(message) == "" {
			message = "Done."
		}
		a.setMessage(message, false)
		if msg.Reload && a.Services.Ready() {
			a.Loading = true
			return a, a.loadCmd()
		}
		return a, nil
	case tea.KeyMsg:
		return a.handleKey(msg)
	}
	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "ctrl+c" {
		return a, tea.Quit
	}
	if a.ShowHelp {
		if key == "esc" || key == "?" || key == "enter" || key == "q" {
			a.ShowHelp = false
		}
		return a, nil
	}
	if a.ConfirmLogout {
		switch key {
		case "y":
			return a.logout()
		case "n", "esc":
			a.ConfirmLogout = false
		}
		return a, nil
	}
	if a.SelectedTask != nil {
		if key == "esc" || key == "enter" {
			a.SelectedTask = nil
		}
		return a, nil
	}

	switch key {
	case "q":
		return a, tea.Quit
	case "?":
		a.ShowHelp = true
		return a, nil
	case "r":
		return a.refresh()
	case "tab":
		if a.Focus == FocusNav {
			a.Focus = FocusContent
		} else {
			a.Focus = FocusNav
		}
		return a, nil
	case "[":
		a.setScreen(previousScreen(a.Screen))
		return a, nil
	case "]":
		a.setScreen(nextScreen(a.Screen))
		return a, nil
	case "esc":
		if a.Focus == FocusContent {
			a.Focus = FocusNav
		}
		return a, nil
	}

	if screen, ok := screenByShortcut(key); ok {
		if screen == ScreenLogout {
			a.ConfirmLogout = true
			return a, nil
		}
		a.setScreen(screen)
		return a, nil
	}

	if a.Focus == FocusNav {
		return a.updateNav(key)
	}
	return a.updateContent(key)
}

func (a *App) setScreen(screen Screen) {
	a.Screen = screen
	a.NavCursor = screen.Index()
	a.Focus = FocusContent
	a.ContentScroll = 0
	a.TaskCursor = 0
	a.WorkspaceCursor = 0
	a.NotificationCursor = 0
}

func (a *App) updateNav(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		a.moveNav(-1)
	case "down", "j":
		a.moveNav(1)
	case "home", "g":
		a.NavCursor = 0
	case "end", "G":
		a.NavCursor = len(NavItems) - 1
	case "enter", " ", "right", "l":
		return a, a.navigate(a.NavCursor)
	}
	return a, nil
}

func (a *App) moveNav(delta int) {
	a.NavCursor += delta
	if a.NavCursor < 0 {
		a.NavCursor = len(NavItems) - 1
	}
	if a.NavCursor >= len(NavItems) {
		a.NavCursor = 0
	}
}

func (a *App) updateContent(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "pgup":
		a.scroll(-5)
	case "pgdown":
		a.scroll(5)
	case "up", "k":
		a.moveContentCursor(-1)
	case "down", "j":
		a.moveContentCursor(1)
	case "home", "g":
		a.setContentCursor(0)
	case "end", "G":
		a.setContentCursor(1 << 30)
	case "enter", " ":
		return a.activateContent()
	case "left", "h":
		a.Focus = FocusNav
	}
	return a, nil
}

func (a *App) scroll(delta int) {
	a.ContentScroll += delta
	if a.ContentScroll < 0 {
		a.ContentScroll = 0
	}
}

func (a *App) moveContentCursor(delta int) {
	switch a.Screen {
	case ScreenMyTasks, ScreenBacklog:
		a.TaskCursor += delta
		a.clampTaskCursor()
	case ScreenWorkspaces:
		a.WorkspaceCursor += delta
		if a.WorkspaceCursor < 0 {
			a.WorkspaceCursor = max(0, len(a.Data.Workspaces)-1)
		}
		if a.WorkspaceCursor >= len(a.Data.Workspaces) {
			a.WorkspaceCursor = 0
		}
	case ScreenNotifications:
		a.NotificationCursor += delta
		if a.NotificationCursor < 0 {
			a.NotificationCursor = max(0, len(a.Data.Notifications)-1)
		}
		if a.NotificationCursor >= len(a.Data.Notifications) {
			a.NotificationCursor = 0
		}
	}
}

func (a *App) setContentCursor(index int) {
	switch a.Screen {
	case ScreenMyTasks, ScreenBacklog:
		a.TaskCursor = index
		a.clampTaskCursor()
	case ScreenWorkspaces:
		a.WorkspaceCursor = min(index, max(0, len(a.Data.Workspaces)-1))
	case ScreenNotifications:
		a.NotificationCursor = min(index, max(0, len(a.Data.Notifications)-1))
	}
}

func (a *App) clampTaskCursor() {
	n := len(a.visibleTasks())
	if n == 0 {
		a.TaskCursor = 0
		return
	}
	if a.TaskCursor < 0 {
		a.TaskCursor = n - 1
	}
	if a.TaskCursor >= n {
		a.TaskCursor = 0
	}
}

func (a *App) activateContent() (tea.Model, tea.Cmd) {
	switch a.Screen {
	case ScreenMyTasks, ScreenBacklog:
		task, ok := a.selectedTask()
		if !ok {
			a.setMessage("No task selected.", true)
			return a, nil
		}
		a.SelectedTask = &task
	case ScreenWorkspaces:
		return a, a.switchWorkspace()
	case ScreenNotifications:
		c := a.context(a.contentWidth())
		notification, ok := screens.NotificationCursor(c, a.NotificationCursor)
		if !ok {
			a.setMessage("No notification selected.", true)
			return a, nil
		}
		if notification.Read {
			a.setMessage("That notification is already read.", false)
			return a, nil
		}
		return a, a.markReadCmd(notification.ID)
	}
	return a, nil
}

func (a *App) switchWorkspace() tea.Cmd {
	c := a.context(a.contentWidth())
	workspace, ok := screens.WorkspaceCursor(c, a.WorkspaceCursor)
	if !ok {
		a.setMessage("No workspace selected.", true)
		return nil
	}
	if workspace.ID == a.Config.WorkspaceID {
		a.setMessage("Already using "+workspace.Name+".", false)
		return nil
	}
	previous := a.Config.WorkspaceID
	a.Config.WorkspaceID = workspace.ID
	if a.Manager != nil {
		if err := a.Manager.Save(a.Config); err != nil {
			a.Config.WorkspaceID = previous
			a.setMessage("Could not save workspace: "+err.Error(), true)
			return nil
		}
	}
	a.setMessage("Workspace set to "+workspace.Name+".", false)
	if a.Services.Ready() {
		a.Loading = true
		return a.loadCmd()
	}
	return nil
}

func (a *App) refresh() (tea.Model, tea.Cmd) {
	if !a.Services.Ready() {
		a.setMessage("The backend contract is still pending; there is nothing to refresh.", true)
		return a, nil
	}
	a.Loading = true
	a.Err = nil
	a.ContentScroll = 0
	return a, a.loadCmd()
}

func (a *App) logout() (tea.Model, tea.Cmd) {
	if a.Session != nil && a.Session.Ephemeral {
		a.ConfirmLogout = false
		a.setMessage("DOUZTACH_TOKEN is set in your environment. Unset it to fully sign out.", true)
		return a, nil
	}
	if a.Manager != nil {
		if err := a.Manager.ClearSession(); err != nil {
			a.ConfirmLogout = false
			a.setMessage("Could not clear the session: "+err.Error(), true)
			return a, nil
		}
	}
	a.Session = nil
	a.ConfirmLogout = false
	a.SelectedTask = nil
	a.Data = screens.Data{}
	a.setMessage("Signed out. The local session token was removed.", false)
	return a, tea.Quit
}

// helpText returns the keyboard reference shown by the help modal.
func helpText() string {
	lines := []string{
		"↑/↓ or j/k    move through the sidebar or the active list",
		"enter         open the selected screen, task, or workspace",
		"tab           switch focus between navigation and content",
		"[ / ]         previous / next screen",
		"d t w b s e n a c p ,   jump directly to a screen",
		"r             refresh data from the backend",
		"pgup/pgdown   scroll long content",
		"esc           close a dialog or return focus to navigation",
		"?             toggle this help",
		"q             quit (ctrl+c also works)",
		"",
		"Logout is available from the sidebar or with x.",
	}
	return strings.Join(lines, "\n")
}
