package components

import (
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// Panel renders a titled bordered panel. Width includes the border; pass 0 for
// natural width.
func Panel(s styles.Styles, title, body string, width int) string {
	if strings.TrimSpace(title) == "" && strings.TrimSpace(body) == "" {
		return ""
	}
	content := body
	if title != "" {
		content = s.PanelTitle.Render(title) + "\n" + body
	}
	panel := s.Panel
	if width > 0 {
		inner := max(1, width-4) // border (2) + horizontal padding (2)
		panel = panel.Width(inner)
		content = Wrap(content, inner)
	}
	return panel.Render(content)
}

// PanelRow renders two panels side by side. When width is too small for two
// readable columns, the panels are stacked vertically.
func PanelRow(s styles.Styles, leftTitle, leftBody, rightTitle, rightBody string, width int) string {
	if width < 76 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			Panel(s, leftTitle, leftBody, width),
			Panel(s, rightTitle, rightBody, width),
		)
	}
	const gap = 1
	columnWidth := (width - gap) / 2
	left := Panel(s, leftTitle, leftBody, columnWidth)
	right := Panel(s, rightTitle, rightBody, width-columnWidth-gap)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
}
