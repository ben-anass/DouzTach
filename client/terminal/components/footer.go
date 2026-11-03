package components

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// KeyHint is a single keyboard shortcut shown in the footer.
type KeyHint struct {
	Key         string
	Description string
}

// RenderFooter renders keyboard hints, wrapping them on narrow terminals.
func RenderFooter(s styles.Styles, hints []KeyHint, width int) string {
	if width <= 0 || len(hints) == 0 {
		return ""
	}
	rendered := make([]string, 0, len(hints))
	for _, hint := range hints {
		if strings.TrimSpace(hint.Key) == "" {
			continue
		}
		rendered = append(rendered, s.KeyHint(hint.Key, hint.Description))
	}
	if len(rendered) == 0 {
		return ""
	}
	separator := s.Divider.Render("  ·  ")
	lines := []string{}
	current := ""
	for _, item := range rendered {
		candidate := item
		if current != "" {
			candidate = current + separator + item
		}
		if current != "" && lipgloss.Width(candidate) > width {
			lines = append(lines, current)
			current = item
			continue
		}
		current = candidate
	}
	if current != "" {
		lines = append(lines, current)
	}
	return s.Footer.Render(strings.Join(lines, "\n"))
}

// DefaultFooterHints returns the navigation hints shared by most screens.
func DefaultFooterHints() []KeyHint {
	return []KeyHint{
		{Key: "↑/↓", Description: "navigate"},
		{Key: "enter", Description: "select"},
		{Key: "tab", Description: "next pane"},
		{Key: "?", Description: "help"},
		{Key: "q", Description: "quit"},
	}
}
