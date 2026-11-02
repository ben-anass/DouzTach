package terminal

import (
	"context"
	"errors"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/screens"
	tea "github.com/charmbracelet/bubbletea"
)

// dataMsg carries a dashboard snapshot or the errors encountered while loading.
type dataMsg struct {
	Data     screens.Data
	Err      error
	LoadedAt time.Time
}

// loadData fetches the data needed by the dashboard and screens. Every service
// is optional so a partially implemented contract can still return useful data.
func loadData(parent context.Context, services api.Services, cfg config.Config) dataMsg {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()

	var data screens.Data
	var errs []error
	addErr := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if services.Auth != nil {
		profile, err := services.Auth.Profile(ctx)
		if err != nil {
			addErr(err)
		} else {
			data.Profile = profile
		}
	}

	workspaceID := cfg.WorkspaceID
	if services.Workspaces != nil {
		workspaces, err := services.Workspaces.List(ctx)
		if err != nil {
			addErr(err)
		} else {
			data.Workspaces = workspaces
			if workspaceID == "" && len(workspaces) > 0 {
				workspaceID = workspaces[0].ID
			}
		}
	}

	if workspaceID != "" {
		if services.Tasks != nil {
			tasks, err := services.Tasks.List(ctx, api.TaskFilter{WorkspaceID: workspaceID})
			if err != nil {
				addErr(err)
			} else {
				data.Tasks = tasks
			}
		}
		if services.Sprints != nil {
			sprints, err := services.Sprints.List(ctx, workspaceID)
			if err != nil {
				addErr(err)
			} else {
				data.Sprints = sprints
			}
		}
		if services.Epics != nil {
			epics, err := services.Epics.List(ctx, workspaceID)
			if err != nil {
				addErr(err)
			} else {
				data.Epics = epics
			}
		}
		if services.Activity != nil {
			activities, err := services.Activity.List(ctx, workspaceID, 30)
			if err != nil {
				addErr(err)
			} else {
				data.Activities = activities
			}
		}
	}

	if services.Notifications != nil {
		notifications, err := services.Notifications.List(ctx)
		if err != nil {
			addErr(err)
		} else {
			data.Notifications = notifications
		}
	}
	if services.Chat != nil {
		channels, err := services.Chat.Channels(ctx)
		if err != nil {
			addErr(err)
		} else {
			data.Channels = channels
		}
	}

	return dataMsg{Data: data, Err: errors.Join(errs...), LoadedAt: time.Now()}
}

// markReadCmd marks one notification as read and reloads the snapshot so the
// unread marker disappears immediately.
func (a *App) markReadCmd(notificationID string) tea.Cmd {
	services := a.Services
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if services.Notifications == nil {
			return actionMsg{Err: api.ErrContractPending}
		}
		if err := services.Notifications.MarkRead(ctx, notificationID); err != nil {
			return actionMsg{Err: err}
		}
		return actionMsg{Reload: true, Message: "Notification marked as read."}
	}
}

// actionMsg reports the result of a side effect such as marking a notification.
type actionMsg struct {
	Err     error
	Reload  bool
	Message string
}
