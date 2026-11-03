package components

import (
	"strconv"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal/styles"
	"github.com/charmbracelet/lipgloss"
)

// StatCard is a single dashboard metric.
type StatCard struct {
	Label string
	Value string
	Hint  string
	Color lipgloss.TerminalColor
}

// RenderStatCards renders a responsive grid of metric cards.
func RenderStatCards(s styles.Styles, cards []StatCard, width int) string {
	if len(cards) == 0 || width <= 0 {
		return ""
	}
	columns := 6
	switch {
	case width < 46:
		columns = 1
	case width < 66:
		columns = 2
	case width < 110:
		columns = 3
	}
	if columns > len(cards) {
		columns = len(cards)
	}
	const gap = 1
	cardWidth := (width - gap*(columns-1)) / columns
	if cardWidth < 14 {
		columns = 1
		cardWidth = width
	}

	rows := make([]string, 0, (len(cards)+columns-1)/columns)
	gapString := strings.Repeat(" ", gap)
	for start := 0; start < len(cards); start += columns {
		end := start + columns
		if end > len(cards) {
			end = len(cards)
		}
		parts := make([]string, 0, (end-start)*2-1)
		for i, card := range cards[start:end] {
			if i > 0 {
				parts = append(parts, gapString)
			}
			parts = append(parts, renderStatCard(s, card, cardWidth))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, parts...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func renderStatCard(s styles.Styles, card StatCard, width int) string {
	innerWidth := max(1, width-2)
	value := strings.TrimSpace(card.Value)
	if value == "" {
		value = "—"
	}
	color := card.Color
	if color == nil {
		color = s.Theme.Text
	}
	title := Truncate(card.Label, innerWidth)
	valueLine := lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(Truncate(value, innerWidth))
	content := s.CardTitle.Render(title) + "\n" + valueLine
	if hint := strings.TrimSpace(card.Hint); hint != "" {
		content += "\n" + s.Muted.Render(Truncate(hint, innerWidth))
	}
	return s.Card.Width(innerWidth).Render(content)
}

// RenderProgress renders a compact horizontal progress bar.
func RenderProgress(s styles.Styles, progress float64, width int) string {
	if width <= 0 {
		return ""
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(float64(width) * progress)
	if filled > width {
		filled = width
	}
	bar := s.Success.Render(strings.Repeat("█", filled)) +
		s.Divider.Render(strings.Repeat("░", width-filled))
	percent := s.Muted.Render(" " + formatPercent(progress))
	return bar + percent
}

func formatPercent(progress float64) string {
	percent := int(progress*100 + 0.5)
	return strconv.Itoa(percent) + "%"
}
