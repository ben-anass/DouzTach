// Package screens renders the individual terminal screens. Each screen is a
// pure function of its Context so it can be tested without a live backend.
package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/ben-anass/DouzTach/client/config"
	"github.com/ben-anass/DouzTach/client/terminal/components"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
)

// Data is the snapshot loaded from the backend. Empty slices mean the backend
// returned no records, not that placeholder data exists.
type Data struct {
	Profile       *api.User
	Workspaces    []api.Workspace
	Tasks         []api.Task
	Sprints       []api.Sprint
	Epics         []api.Epic
	Notifications []api.Notification
	Activities    []api.Activity
	Channels      []api.ChatChannel
}

// Context is everything a screen needs to render.
type Context struct {
	Styles    styles.Styles
	Width     int
	Height    int
	Services  api.Services
	Config    config.Config
	Session   *config.Session
	ConfigDir string
	Data      Data
	Loading   bool
	Err       error
	Now       time.Time
	LoadedAt  time.Time

	// Cursor is the highlighted row on list screens so keyboard navigation is
	// visible. It is ignored by screens that are not lists.
	Cursor int
}

// Ready reports whether a real backend contract is implemented.
func (c Context) Ready() bool { return c.Services.Ready() }

// CurrentUserID returns the signed-in user's ID when known.
func (c Context) CurrentUserID() string {
	if c.Session != nil && c.Session.User.ID != "" {
		return c.Session.User.ID
	}
	if c.Data.Profile != nil {
		return c.Data.Profile.ID
	}
	return ""
}

// CurrentUserName returns a safe display name for the signed-in user.
func (c Context) CurrentUserName() string {
	if c.Session != nil {
		if name := c.Session.User.DisplayName(); name != "" && name != "Unknown user" {
			return name
		}
	}
	if c.Data.Profile != nil {
		return c.Data.Profile.DisplayName()
	}
	return "Not signed in"
}

// WorkspaceID returns the selected workspace ID.
func (c Context) WorkspaceID() string {
	if c.Config.WorkspaceID != "" {
		return c.Config.WorkspaceID
	}
	if len(c.Data.Workspaces) > 0 {
		return c.Data.Workspaces[0].ID
	}
	return ""
}

// WorkspaceName returns the selected workspace's name when it is known.
func (c Context) WorkspaceName() string {
	id := c.WorkspaceID()
	for _, workspace := range c.Data.Workspaces {
		if workspace.ID == id {
			return workspace.Name
		}
	}
	if id != "" {
		return id
	}
	return "No workspace selected"
}

// Workspace returns the selected workspace when it is loaded.
func (c Context) Workspace() (api.Workspace, bool) {
	id := c.WorkspaceID()
	for _, workspace := range c.Data.Workspaces {
		if workspace.ID == id {
			return workspace, true
		}
	}
	return api.Workspace{}, false
}

// HasData reports whether at least one collection or profile was loaded.
func (c Context) HasData() bool {
	return c.Data.Profile != nil ||
		len(c.Data.Workspaces) > 0 ||
		len(c.Data.Tasks) > 0 ||
		len(c.Data.Sprints) > 0 ||
		len(c.Data.Epics) > 0 ||
		len(c.Data.Notifications) > 0 ||
		len(c.Data.Activities) > 0 ||
		len(c.Data.Channels) > 0
}

// Header renders a screen title, optional subtitle, and divider.
func Header(c Context, title, subtitle string) string {
	if c.Width <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(c.Styles.SectionTitle.Render(title))
	if strings.TrimSpace(subtitle) != "" {
		b.WriteString("\n")
		b.WriteString(c.Styles.Muted.Render(components.Wrap(subtitle, c.Width)))
	}
	b.WriteString("\n")
	b.WriteString(c.Styles.DividerString(c.Width))
	return b.String()
}

// Gate renders the common not-connected/loading/error states. The bool result
// reports whether the screen has been fully handled and should not render data.
func Gate(c Context, title, subtitle, feature, requirement string) (string, bool) {
	header := Header(c, title, subtitle)
	switch {
	case !c.Ready():
		return header + "\n\n" + Pending(c, feature, requirement), true
	case c.Loading:
		return header + "\n\n" + c.Styles.Muted.Render("Loading data from the backend…"), true
	case c.Err != nil && !c.HasData():
		return header + "\n\n" + ErrorState(c, c.Err), true
	case c.Err != nil:
		header += "\n" + c.Styles.Warning.Render("Partial data: "+c.Err.Error())
	}
	return header, false
}

// Pending renders the explicit "backend not connected" state.
func Pending(c Context, feature, requirement string) string {
	return components.RenderEmptyState(c.Styles, components.PendingState(c.Styles, feature, requirement), c.Width)
}

// ErrorState renders a load failure without pretending data is empty.
func ErrorState(c Context, err error) string {
	message := "The request failed. Check the API URL and try again."
	if err != nil {
		message = err.Error()
	}
	return components.RenderEmptyState(c.Styles, components.EmptyState{
		Icon:    "!",
		Title:   "Could not load data",
		Message: message,
		Hint:    "Press r to retry, or run 'douztech doctor'.",
		Tone:    components.ToneError,
	}, c.Width)
}

// Empty renders a neutral empty collection state.
func Empty(c Context, title, message, hint string) string {
	return components.RenderEmptyState(c.Styles, components.EmptyState{
		Icon:    "◇",
		Title:   title,
		Message: message,
		Hint:    hint,
		Tone:    components.ToneNeutral,
	}, c.Width)
}

// SectionLabel renders a small muted label above a block.
func SectionLabel(c Context, label string) string {
	return c.Styles.CardTitle.Render(strings.ToUpper(label))
}

// FooterHint returns the refresh/updated line shown under data screens.
func FooterHint(c Context, loadedAt time.Time) string {
	if !c.Ready() {
		return c.Styles.Help.Render("Backend contract pending · no sample data is shown")
	}
	if loadedAt.IsZero() {
		return ""
	}
	return c.Styles.Help.Render(fmt.Sprintf("Updated %s · press r to refresh", components.RelativeTime(loadedAt, c.Now)))
}
