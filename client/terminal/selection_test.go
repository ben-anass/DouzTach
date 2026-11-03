package terminal

import (
	"strings"
	"testing"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
)

func TestWorkspaceCursorIsVisibleAndMoves(t *testing.T) {
	app := testApp()
	app.Services.Status.ContractReady = true
	app.Data = screens.Data{Workspaces: []api.Workspace{
		{ID: "w1", Name: "Alpha"},
		{ID: "w2", Name: "Beta"},
	}}
	app.setScreen(ScreenWorkspaces)

	view := app.View()
	if !strings.Contains(view, "› ● Alpha") {
		t.Fatalf("the selected workspace should show the cursor marker:\n%s", view)
	}

	app, _ = updateKey(t, app, "down")
	view = app.View()
	if !strings.Contains(view, "›   Beta") {
		t.Fatalf("the cursor should move to the second workspace:\n%s", view)
	}
	if strings.Contains(view, "› ● Alpha") {
		t.Fatalf("the first row should no longer hold the cursor:\n%s", view)
	}
	if !strings.Contains(view, "● Alpha") {
		t.Fatalf("the current workspace marker must stay on Alpha:\n%s", view)
	}
}

func TestNotificationCursorIsVisibleAndMoves(t *testing.T) {
	app := testApp()
	app.Services.Status.ContractReady = true
	app.Data = screens.Data{Notifications: []api.Notification{
		{ID: "n1", Title: "First"},
		{ID: "n2", Title: "Second", Read: true},
	}}
	app.setScreen(ScreenNotifications)

	if view := app.View(); !strings.Contains(view, "› ● First") {
		t.Fatalf("the first notification should hold the cursor:\n%s", view)
	}
	app, _ = updateKey(t, app, "down")
	if view := app.View(); !strings.Contains(view, "›   Second") {
		t.Fatalf("the cursor should move to the second notification:\n%s", view)
	}
}

func TestTaskTableHighlightsTheSelectedRow(t *testing.T) {
	app := testApp()
	app.Services.Status.ContractReady = true
	app.Session = &config.Session{AccessToken: "token", User: config.User{ID: "u1", Name: "Jane"}}
	assignee := &api.User{ID: "u1", Name: "Jane"}
	app.Data = screens.Data{Tasks: []api.Task{
		{ID: "t1", Key: "DT-1", Title: "First task", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask, Assignee: assignee},
		{ID: "t2", Key: "DT-2", Title: "Second task", Status: api.TaskStatusTodo, Priority: api.PriorityLow, Type: api.TaskTypeTask, Assignee: assignee},
	}}
	app.setScreen(ScreenMyTasks)

	view := app.View()
	if !strings.Contains(view, "▸ DT-1") {
		t.Fatalf("the selected task row should carry the cursor marker:\n%s", view)
	}
	if strings.Contains(view, "▸ DT-2") {
		t.Fatalf("only one row may hold the cursor:\n%s", view)
	}

	app, _ = updateKey(t, app, "down")
	if app.TaskCursor != 1 {
		t.Fatalf("task cursor = %d, want 1", app.TaskCursor)
	}
	view = app.View()
	if !strings.Contains(view, "▸ DT-2") {
		t.Fatalf("the cursor should move to the second task:\n%s", view)
	}
}

func TestMarkReadRequestsAReload(t *testing.T) {
	app := testApp()
	app.Services.Status.ContractReady = true

	model, cmd := app.Update(actionMsg{Reload: true, Message: "Notification marked as read."})
	result, ok := model.(*App)
	if !ok {
		t.Fatalf("unexpected model type %T", model)
	}
	if result.Message != "Notification marked as read." {
		t.Fatalf("message = %q", result.Message)
	}
	if result.MessageIsError {
		t.Fatal("a successful action must not be reported as an error")
	}
	if cmd == nil {
		t.Fatal("marking a notification read should reload the snapshot")
	}
	if !result.Loading {
		t.Fatal("a reload should set the loading state")
	}
}

func TestActionFailureIsReportedAsError(t *testing.T) {
	app := testApp()
	model, _ := app.Update(actionMsg{Err: api.ErrContractPending})
	result := model.(*App)
	if !result.MessageIsError {
		t.Fatal("a failed action should be reported as an error")
	}
	if !strings.Contains(result.Message, "failed") {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestScreenCursorMatchesActiveScreen(t *testing.T) {
	app := testApp()
	app.WorkspaceCursor = 2
	app.NotificationCursor = 1
	app.TaskCursor = 3

	app.Screen = ScreenWorkspaces
	if got := app.context(80).Cursor; got != 2 {
		t.Fatalf("workspace cursor = %d, want 2", got)
	}
	app.Screen = ScreenNotifications
	if got := app.context(80).Cursor; got != 1 {
		t.Fatalf("notification cursor = %d, want 1", got)
	}
	app.Screen = ScreenBacklog
	if got := app.context(80).Cursor; got != 3 {
		t.Fatalf("task cursor = %d, want 3", got)
	}
	app.Screen = ScreenDashboard
	if got := app.context(80).Cursor; got != 0 {
		t.Fatalf("dashboard cursor = %d, want 0", got)
	}
}
