package terminal

import (
	"context"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// FocusArea identifies which pane receives navigation keys.
type FocusArea int

const (
	// FocusNav is the sidebar.
	FocusNav FocusArea = iota
	// FocusContent is the active screen.
	FocusContent
)

// Options configures the terminal application.
type Options struct {
	Version  string
	Styles   styles.Styles
	Services api.Services
	Manager  *config.Manager
	Config   config.Config
	Session  *config.Session
	Width    int
	Height   int
	Now      func() time.Time
}

// App is the root Bubble Tea model.
type App struct {
	Version  string
	Styles   styles.Styles
	Services api.Services
	Manager  *config.Manager
	Config   config.Config
	Session  *config.Session

	Width  int
	Height int

	Screen        Screen
	NavCursor     int
	Focus         FocusArea
	ShowHelp      bool
	ConfirmLogout bool
	SelectedTask  *api.Task

	Data     screens.Data
	Loading  bool
	Err      error
	LoadedAt time.Time
	Now      time.Time
	clock    func() time.Time

	TaskCursor         int
	WorkspaceCursor    int
	NotificationCursor int
	ContentScroll      int

	Message        string
	MessageIsError bool
}

// New builds the application model. It does not perform I/O.
func New(opts Options) *App {
	s := opts.Styles
	if s.Theme.Primary == nil {
		s = styles.New(false)
	}
	clock := opts.Now
	if clock == nil {
		clock = time.Now
	}
	width := opts.Width
	if width <= 0 {
		width = 100
	}
	height := opts.Height
	if height <= 0 {
		height = 30
	}
	app := &App{
		Version:  opts.Version,
		Styles:   s,
		Services: opts.Services,
		Manager:  opts.Manager,
		Config:   opts.Config,
		Session:  opts.Session,
		Width:    width,
		Height:   height,
		Screen:   ScreenDashboard,
		Focus:    FocusNav,
		Now:      clock(),
		clock:    clock,
		Loading:  opts.Services.Ready(),
	}
	app.NavCursor = app.Screen.Index()
	return app
}

// Run launches the interactive terminal application.
func Run(ctx context.Context, opts Options) error {
	app := New(opts)
	program := tea.NewProgram(app, tea.WithContext(ctx), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

// Init implements tea.Model. Data is only requested when a real backend
// contract is registered; otherwise the UI shows an explicit pending state.
func (a *App) Init() tea.Cmd {
	if !a.Services.Ready() {
		return nil
	}
	return a.loadCmd()
}

// loadCmd fetches the dashboard snapshot.
func (a *App) loadCmd() tea.Cmd {
	services := a.Services
	cfg := a.Config
	return func() tea.Msg {
		return loadData(context.Background(), services, cfg)
	}
}

// context builds the immutable screen context for the current size.
func (a *App) context(width int) screens.Context {
	configDir := ""
	if a.Manager != nil {
		configDir = a.Manager.Dir()
	}
	return screens.Context{
		Styles:    a.Styles,
		Width:     width,
		Height:    a.Height,
		Services:  a.Services,
		Config:    a.Config,
		Session:   a.Session,
		ConfigDir: configDir,
		Data:      a.Data,
		Loading:   a.Loading,
		Err:       a.Err,
		Now:       a.Now,
		LoadedAt:  a.LoadedAt,
		Cursor:    a.cursorForScreen(),
	}
}

// cursorForScreen returns the highlighted row for the active list screen so
// screens can render the keyboard cursor.
func (a *App) cursorForScreen() int {
	switch a.Screen {
	case ScreenWorkspaces:
		return a.WorkspaceCursor
	case ScreenNotifications:
		return a.NotificationCursor
	case ScreenMyTasks, ScreenBacklog:
		return a.TaskCursor
	default:
		return 0
	}
}

// setMessage records a short status line shown above the footer.
func (a *App) setMessage(message string, isError bool) {
	a.Message = message
	a.MessageIsError = isError
}

// navigate moves to a screen or opens the logout confirmation.
func (a *App) navigate(index int) tea.Cmd {
	if index < 0 || index >= len(NavItems) {
		return nil
	}
	item := NavItems[index]
	a.NavCursor = index
	a.ContentScroll = 0
	a.TaskCursor = 0
	a.WorkspaceCursor = 0
	a.NotificationCursor = 0
	if item.Action {
		a.ConfirmLogout = true
		return nil
	}
	a.Screen = item.Screen
	a.Focus = FocusContent
	return nil
}

// clampCursors keeps selection indexes valid after a data refresh.
func (a *App) clampCursors() {
	if a.NavCursor < 0 || a.NavCursor >= len(NavItems) {
		a.NavCursor = a.Screen.Index()
	}
	if n := len(a.Data.Workspaces); a.WorkspaceCursor >= n {
		a.WorkspaceCursor = max(0, n-1)
	}
	if n := len(a.Data.Notifications); a.NotificationCursor >= n {
		a.NotificationCursor = max(0, n-1)
	}
	if n := len(a.visibleTasks()); a.TaskCursor >= n {
		a.TaskCursor = max(0, n-1)
	}
}

// visibleTasks returns the tasks listed by the current screen.
func (a *App) visibleTasks() []api.Task {
	c := a.context(a.contentWidth())
	switch a.Screen {
	case ScreenMyTasks:
		return screens.MineTasks(c)
	case ScreenBacklog:
		return screens.BacklogTasks(c)
	default:
		return nil
	}
}

// selectedTask returns the highlighted task for the current screen.
func (a *App) selectedTask() (api.Task, bool) {
	tasks := a.visibleTasks()
	if a.TaskCursor < 0 || a.TaskCursor >= len(tasks) {
		return api.Task{}, false
	}
	return tasks[a.TaskCursor], true
}
