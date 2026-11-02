package components

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// Modal describes a centred dialog. It is a rendering helper; the owning
// Bubble Tea model decides when it is visible and how input is handled.
type Modal struct {
	Title   string
	Body    string
	Actions []ModalAction
	Width   int
	Tone    EmptyTone
}

// ModalAction is a labelled choice shown at the bottom of a modal.
type ModalAction struct {
	Label       string
	Key         string
	Primary     bool
	Destructive bool
}

// RenderModal renders the dialog centred within the available area. When the
// terminal is smaller than the dialog, it shrinks to fit rather than overflowing.
func RenderModal(s styles.Styles, modal Modal, width, height int) string {
	if width <= 0 {
		return ""
	}
	dialogWidth := modal.Width
	if dialogWidth <= 0 {
		dialogWidth = min(64, width-4)
	}
	if dialogWidth > width-2 {
		dialogWidth = max(20, width-2)
	}
	inner := max(10, dialogWidth-6) // border (2) + padding (4)

	titleStyle := s.PanelTitle
	switch modal.Tone {
	case TonePending:
		titleStyle = s.Warning
	case ToneError:
		titleStyle = s.Error
	}
	title := modal.Title
	if strings.TrimSpace(title) == "" {
		title = "Confirm"
	}
	body := Wrap(strings.TrimSpace(modal.Body), inner)
	content := titleStyle.Render(Truncate(title, inner)) + "\n\n" + s.Text.Render(body)

	if len(modal.Actions) > 0 {
		parts := make([]string, 0, len(modal.Actions))
		for _, action := range modal.Actions {
			parts = append(parts, renderModalAction(s, action))
		}
		content += "\n\n" + strings.Join(parts, "   ")
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(s.Theme.Border).
		Padding(1, 2).
		Width(inner).
		Render(content)

	if height > 0 && lipgloss.Height(dialog) > height {
		dialog = Clamp(dialog, width, height)
	}
	return lipgloss.Place(width, max(0, height), lipgloss.Center, lipgloss.Center, dialog)
}

func renderModalAction(s styles.Styles, action ModalAction) string {
	label := action.Label
	if action.Key != "" {
		label = "[" + action.Key + "] " + label
	}
	switch {
	case action.Destructive:
		return s.Error.Render(label)
	case action.Primary:
		return s.Selected.Render(label)
	default:
		return s.Muted.Render(label)
	}
}
