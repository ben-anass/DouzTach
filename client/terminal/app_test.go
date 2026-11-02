package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func testApp() *App {
	return New(Options{
		Version: "test",
		Styles:  styles.New(true),
		Width:   100,
		Height:  30,
		Now:     func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) },
	})
}

func key(value string) tea.KeyMsg {
	switch value {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
	}
}

func updateKey(t *testing.T, app *App, value string) (*App, tea.Cmd) {
	t.Helper()
	model, cmd := app.Update(key(value))
	result, ok := model.(*App)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}
	return result, cmd
}

func TestNewDefaultsToDashboard(t *testing.T) {
	t.Parallel()
	app := testApp()
	if app.Screen != ScreenDashboard {
		t.Fatalf("screen = %v", app.Screen)
	}
	if app.Focus != FocusNav {
		t.Fatalf("focus = %v", app.Focus)
	}
	if app.NavCursor != 0 {
		t.Fatalf("nav cursor = %d", app.NavCursor)
	}
}

func TestNavigationWithArrowAndEnter(t *testing.T) {
	t.Parallel()
	app := testApp()
	app, _ = updateKey(t, app, "down")
	if app.NavCursor != 1 {
		t.Fatalf("nav cursor = %d, want 1", app.NavCursor)
	}
	app, _ = updateKey(t, app, "enter")
	if app.Screen != ScreenMyTasks {
		t.Fatalf("screen = %v, want My Tasks", app.Screen)
	}
	if app.Focus != FocusContent {
		t.Fatalf("focus = %v, want content", app.Focus)
	}
}

func TestShortcutNavigation(t *testing.T) {
	t.Parallel()
	app := testApp()
	app, _ = updateKey(t, app, "w")
	if app.Screen != ScreenWorkspaces {
		t.Fatalf("screen = %v, want workspaces", app.Screen)
	}
	app, _ = updateKey(t, app, "p")
	if app.Screen != ScreenProfile {
		t.Fatalf("screen = %v, want profile", app.Screen)
	}
}

func TestBracketNavigationSkipsLogout(t *testing.T) {
	t.Parallel()
	app := testApp()
	app.Screen = ScreenSettings
	app.NavCursor = ScreenSettings.Index()
	app, _ = updateKey(t, app, "]")
	if app.Screen != ScreenDashboard {
		t.Fatalf("next after settings = %v, want dashboard", app.Screen)
	}
	app, _ = updateKey(t, app, "[")
	if app.Screen == ScreenLogout {
		t.Fatal("bracket navigation must skip logout")
	}
}

func TestTabTogglesFocus(t *testing.T) {
	t.Parallel()
	app := testApp()
	app, _ = updateKey(t, app, "tab")
	if app.Focus != FocusContent {
		t.Fatalf("focus = %v", app.Focus)
	}
	app, _ = updateKey(t, app, "tab")
	if app.Focus != FocusNav {
		t.Fatalf("focus = %v", app.Focus)
	}
}

func TestHelpModalOpensAndCloses(t *testing.T) {
	t.Parallel()
	app := testApp()
	app, _ = updateKey(t, app, "?")
	if !app.ShowHelp {
		t.Fatal("help should open")
	}
	if !strings.Contains(app.View(), "Keyboard help") {
		t.Fatal("help modal missing from view")
	}
	app, _ = updateKey(t, app, "esc")
	if app.ShowHelp {
		t.Fatal("help should close on escape")
	}
}

func TestLogoutConfirmationCancels(t *testing.T) {
	t.Parallel()
	app := testApp()
	app, _ = updateKey(t, app, "x")
	if !app.ConfirmLogout {
		t.Fatal("logout confirmation should open")
	}
	app, _ = updateKey(t, app, "n")
	if app.ConfirmLogout {
		t.Fatal("logout confirmation should cancel")
	}
}

func TestLogoutClearsSessionFile(t *testing.T) {
	t.Parallel()
	manager, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveSession(&config.Session{AccessToken: "secret-token", User: config.User{ID: "u1"}}); err != nil {
		t.Fatal(err)
	}
	app := New(Options{
		Version: "test",
		Styles:  styles.New(true),
		Manager: manager,
		Session: &config.Session{AccessToken: "secret-token", User: config.User{ID: "u1"}},
		Width:   100,
		Height:  30,
	})
	app, _ = updateKey(t, app, "x")
	app, cmd := updateKey(t, app, "y")
	if app.Session != nil {
		t.Fatal("session should be cleared")
	}
	if manager.SessionExists() {
		t.Fatal("session file should be removed")
	}
	if cmd == nil {
		t.Fatal("logout should quit the program")
	}
}

func TestDashboardPendingStateIsHonest(t *testing.T) {
	t.Parallel()
	app := testApp()
	view := app.View()
	if !strings.Contains(view, "not connected") {
		t.Fatalf("pending dashboard should say not connected: %q", view)
	}
	if strings.Contains(view, "Total tasks\n0") {
		t.Fatal("pending dashboard must not show fabricated zero counts")
	}
	if !strings.Contains(view, "—") {
		t.Fatal("pending metrics should use an em dash")
	}
}

func TestDashboardWithRealData(t *testing.T) {
	t.Parallel()
	app := testApp()
	app.Services.Status.ContractReady = true
	app.Data = screens.Data{
		Tasks: []api.Task{
			{ID: "1", Title: "One", Status: api.TaskStatusTodo, Priority: api.PriorityHigh, Type: api.TaskTypeTask},
			{ID: "2", Title: "Two", Status: api.TaskStatusDone, Priority: api.PriorityLow, Type: api.TaskTypeBug},
		},
		Sprints: []api.Sprint{
			{ID: "s1", Name: "Sprint 1", Status: api.SprintStatusActive, TaskCount: 2, DoneCount: 1},
		},
	}
	view := app.View()
	if !strings.Contains(view, "Sprint 1") {
		t.Fatalf("active sprint missing: %q", view)
	}
	if strings.Contains(view, "not connected") {
		t.Fatal("ready dashboard should not show the pending banner")
	}
}

func TestTaskSelectionOpensDetail(t *testing.T) {
	t.Parallel()
	app := testApp()
	app.Services.Status.ContractReady = true
	app.Session = &config.Session{AccessToken: "token", User: config.User{ID: "u1", Name: "Jane"}}
	app.Data = screens.Data{Tasks: []api.Task{{
		ID: "t1", Key: "DT-1", Title: "Fix login", Status: api.TaskStatusTodo,
		Priority: api.PriorityHigh, Type: api.TaskTypeBug,
		Assignee: &api.User{ID: "u1", Name: "Jane"},
	}}}
	app.Screen = ScreenMyTasks
	app.Focus = FocusContent
	app, _ = updateKey(t, app, "enter")
	if app.SelectedTask == nil {
		t.Fatal("enter should open the selected task")
	}
	if !strings.Contains(app.View(), "Fix login") {
		t.Fatal("task detail should contain the title")
	}
	app, _ = updateKey(t, app, "esc")
	if app.SelectedTask != nil {
		t.Fatal("escape should close task details")
	}
}

func TestViewFitsTerminalWidthAndHeight(t *testing.T) {
	t.Parallel()
	for _, width := range []int{120, 90, 70, 55, 40, 25} {
		for _, height := range []int{32, 20, 12} {
			app := New(Options{
				Version: "test",
				Styles:  styles.New(false),
				Width:   width,
				Height:  height,
			})
			app.Services.Status.ContractReady = false
			view := app.View()
			lines := strings.Split(view, "\n")
			if len(lines) > height {
				t.Errorf("width %d height %d: %d lines exceeds height", width, height, len(lines))
			}
			for _, line := range lines {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("width %d height %d: line width %d exceeds terminal: %q", width, height, w, line)
				}
			}
		}
	}
}

func TestScrollClampsContent(t *testing.T) {
	t.Parallel()
	app := testApp()
	app.Screen = ScreenSettings
	app.Focus = FocusContent
	app, _ = updateKey(t, app, "pgdown")
	if app.ContentScroll != 5 {
		t.Fatalf("scroll = %d", app.ContentScroll)
	}
	app, _ = updateKey(t, app, "pgup")
	app, _ = updateKey(t, app, "pgup")
	if app.ContentScroll != 0 {
		t.Fatalf("scroll should clamp at zero, got %d", app.ContentScroll)
	}
}

func TestRefreshWithoutContractIsRejected(t *testing.T) {
	t.Parallel()
	app := testApp()
	app.Focus = FocusContent
	app, cmd := updateKey(t, app, "r")
	if cmd != nil {
		t.Fatal("refresh should not run without a contract")
	}
	if !app.MessageIsError {
		t.Fatal("refresh should report that the contract is pending")
	}
}
