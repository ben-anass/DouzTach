package terminal

// Screen identifies a section of the terminal application.
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenMyTasks
	ScreenWorkspaces
	ScreenBacklog
	ScreenSprintBoard
	ScreenEpics
	ScreenNotifications
	ScreenActivity
	ScreenChat
	ScreenProfile
	ScreenSettings
	ScreenLogout
)

// NavItem is one entry in the sidebar.
type NavItem struct {
	Screen      Screen
	Title       string
	Description string
	Shortcut    string
	Action      bool
}

// NavItems is the fixed navigation order required by the product brief.
var NavItems = []NavItem{
	{Screen: ScreenDashboard, Title: "Dashboard", Description: "Overview of your work", Shortcut: "d"},
	{Screen: ScreenMyTasks, Title: "My Tasks", Description: "Tasks assigned to you", Shortcut: "t"},
	{Screen: ScreenWorkspaces, Title: "Workspaces", Description: "Switch or create a workspace", Shortcut: "w"},
	{Screen: ScreenBacklog, Title: "Backlog", Description: "Unscheduled work", Shortcut: "b"},
	{Screen: ScreenSprintBoard, Title: "Sprint Board", Description: "Kanban for the active sprint", Shortcut: "s"},
	{Screen: ScreenEpics, Title: "Epics", Description: "Larger bodies of work", Shortcut: "e"},
	{Screen: ScreenNotifications, Title: "Notifications", Description: "Inbox and mentions", Shortcut: "n"},
	{Screen: ScreenActivity, Title: "Activity", Description: "Workspace history", Shortcut: "a"},
	{Screen: ScreenChat, Title: "Chat", Description: "Team conversations", Shortcut: "c"},
	{Screen: ScreenProfile, Title: "Profile", Description: "Your account", Shortcut: "p"},
	{Screen: ScreenSettings, Title: "Settings", Description: "Client configuration", Shortcut: ","},
	{Screen: ScreenLogout, Title: "Logout", Description: "Sign out of DouzTach", Shortcut: "x", Action: true},
}

// Title returns the display title for a screen.
func (s Screen) Title() string {
	for _, item := range NavItems {
		if item.Screen == s {
			return item.Title
		}
	}
	return "DouzTach"
}

// Index returns the position of a screen in the navigation list.
func (s Screen) Index() int {
	for i, item := range NavItems {
		if item.Screen == s {
			return i
		}
	}
	return 0
}

// ScreenAt returns the screen for a navigation index, clamped to the list.
func ScreenAt(index int) Screen {
	if index < 0 {
		index = 0
	}
	if index >= len(NavItems) {
		index = len(NavItems) - 1
	}
	return NavItems[index].Screen
}

// nextScreen returns the screen after the current one, skipping the logout
// action so bracket navigation stays on content screens.
func nextScreen(current Screen) Screen {
	index := current.Index()
	for i := 1; i <= len(NavItems); i++ {
		candidate := NavItems[(index+i)%len(NavItems)]
		if !candidate.Action {
			return candidate.Screen
		}
	}
	return ScreenDashboard
}

// previousScreen returns the screen before the current one, skipping logout.
func previousScreen(current Screen) Screen {
	index := current.Index()
	for i := 1; i <= len(NavItems); i++ {
		position := (index - i + len(NavItems)) % len(NavItems)
		candidate := NavItems[position]
		if !candidate.Action {
			return candidate.Screen
		}
	}
	return ScreenDashboard
}

// screenByShortcut resolves a single-key navigation shortcut.
func screenByShortcut(key string) (Screen, bool) {
	for _, item := range NavItems {
		if item.Shortcut == key {
			return item.Screen, true
		}
	}
	return 0, false
}
