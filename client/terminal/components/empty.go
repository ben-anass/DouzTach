package components

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// EmptyState describes an intentional empty or unavailable screen area. It is
// used for "no data yet" and "backend not connected" states alike, with an
// explicit message so the UI never implies that missing data is real data.
type EmptyState struct {
	Icon    string
	Title   string
	Message string
	Hint    string
	Tone    EmptyTone
}

// EmptyTone selects the accent used by an empty state.
type EmptyTone int

const (
	// ToneNeutral is used for ordinary empty collections.
	ToneNeutral EmptyTone = iota
	// TonePending is used when a backend feature is not connected yet.
	TonePending
	// ToneError is used for failed loads.
	ToneError
)

// RenderEmptyState renders a centred empty state inside the available width.
func RenderEmptyState(s styles.Styles, state EmptyState, width int) string {
	if width <= 0 {
		return ""
	}
	icon := state.Icon
	if strings.TrimSpace(icon) == "" {
		icon = "◇"
	}
	iconStyle := s.Muted
	switch state.Tone {
	case TonePending:
		iconStyle = s.Warning
	case ToneError:
		iconStyle = s.Error
	}

	title := state.Title
	if strings.TrimSpace(title) == "" {
		title = "Nothing here yet"
	}
	lines := []string{
		iconStyle.Render(icon) + "  " + s.SectionTitle.Render(title),
	}
	if msg := strings.TrimSpace(state.Message); msg != "" {
		lines = append(lines, "", s.Muted.Render(Wrap(msg, max(10, width-4))))
	}
	if hint := strings.TrimSpace(state.Hint); hint != "" {
		lines = append(lines, "", s.Help.Render(Wrap(hint, max(10, width-4))))
	}
	content := lipgloss.JoinVertical(lipgloss.Center, lines...)
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(content)
}

// PendingState is the standard "feature not connected" state. It is deliberately
// explicit: no placeholder data is shown.
func PendingState(s styles.Styles, feature, requirement string) EmptyState {
	message := "This feature needs the backend API contract from Member 2 (nani)."
	if strings.TrimSpace(requirement) != "" {
		message = requirement
	}
	return EmptyState{
		Icon:    "◌",
		Title:   feature + " is not connected yet",
		Message: message,
		Hint:    "No sample data is shown because none was loaded from a backend.",
		Tone:    TonePending,
	}
}
