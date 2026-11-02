// Package styles defines the visual design system for the DouzTach terminal
// client. Every screen and component uses these tokens so colours, spacing,
// borders, and status indicators stay consistent.
package styles

import (
	"os"
	"strings"

	"github.com/ben-anass/DouzTach/client/api"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds the raw colour tokens. Colours are adaptive so the interface
// remains readable on both light and dark terminals.
type Theme struct {
	Primary    lipgloss.TerminalColor
	Secondary  lipgloss.TerminalColor
	Success    lipgloss.TerminalColor
	Warning    lipgloss.TerminalColor
	Danger     lipgloss.TerminalColor
	Muted      lipgloss.TerminalColor
	Text       lipgloss.TerminalColor
	Subtle     lipgloss.TerminalColor
	Border     lipgloss.TerminalColor
	Surface    lipgloss.TerminalColor
	SelectedBg lipgloss.TerminalColor
	SelectedFg lipgloss.TerminalColor
	OnPrimary  lipgloss.TerminalColor
}

// Styles is the complete set of reusable text styles.
type Styles struct {
	Theme Theme

	AppTitle       lipgloss.Style
	AppSubtitle    lipgloss.Style
	Header         lipgloss.Style
	HeaderMuted    lipgloss.Style
	SectionTitle   lipgloss.Style
	Text           lipgloss.Style
	Muted          lipgloss.Style
	Subtle         lipgloss.Style
	Success        lipgloss.Style
	Warning        lipgloss.Style
	Error          lipgloss.Style
	Info           lipgloss.Style
	Label          lipgloss.Style
	Value          lipgloss.Style
	Card           lipgloss.Style
	CardTitle      lipgloss.Style
	CardValue      lipgloss.Style
	Panel          lipgloss.Style
	PanelTitle     lipgloss.Style
	Badge          lipgloss.Style
	Selected       lipgloss.Style
	MenuItem       lipgloss.Style
	MenuItemActive lipgloss.Style
	Sidebar        lipgloss.Style
	Footer         lipgloss.Style
	Key            lipgloss.Style
	Help           lipgloss.Style
	Input          lipgloss.Style
	InputLabel     lipgloss.Style
	ErrorText      lipgloss.Style
	Divider        lipgloss.Style
	Logo           lipgloss.Style
	LogoAccent     lipgloss.Style
}

// New builds the design system. When noColor is true, foreground and
// background colours are suppressed while bold, borders, and layout remain.
func New(noColor bool) Styles {
	theme := defaultTheme()
	if noColor {
		theme = noColorTheme()
	}
	return Styles{
		Theme: theme,

		AppTitle: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),
		AppSubtitle: lipgloss.NewStyle().
			Foreground(theme.Muted),
		Header: lipgloss.NewStyle().
			Foreground(theme.Text).
			Bold(true),
		HeaderMuted: lipgloss.NewStyle().
			Foreground(theme.Muted),
		SectionTitle: lipgloss.NewStyle().
			Foreground(theme.Text).
			Bold(true),
		Text: lipgloss.NewStyle().
			Foreground(theme.Text),
		Muted: lipgloss.NewStyle().
			Foreground(theme.Muted),
		Subtle: lipgloss.NewStyle().
			Foreground(theme.Subtle),
		Success: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true),
		Error: lipgloss.NewStyle().
			Foreground(theme.Danger).
			Bold(true),
		Info: lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Bold(true),
		Label: lipgloss.NewStyle().
			Foreground(theme.Muted),
		Value: lipgloss.NewStyle().
			Foreground(theme.Text).
			Bold(true),
		Card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		CardTitle: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Bold(true),
		CardValue: lipgloss.NewStyle().
			Foreground(theme.Text).
			Bold(true),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2),
		PanelTitle: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),
		Badge: lipgloss.NewStyle().
			Foreground(theme.Text).
			Background(theme.Surface).
			Padding(0, 1).
			Bold(true),
		Selected: lipgloss.NewStyle().
			Foreground(theme.SelectedFg).
			Background(theme.SelectedBg).
			Bold(true),
		MenuItem: lipgloss.NewStyle().
			Foreground(theme.Subtle).
			Padding(0, 1),
		MenuItemActive: lipgloss.NewStyle().
			Foreground(theme.SelectedFg).
			Background(theme.SelectedBg).
			Bold(true).
			Padding(0, 1),
		Sidebar: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(theme.Border).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(theme.Muted),
		Key: lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(theme.Subtle),
		Input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Primary).
			Padding(0, 1),
		InputLabel: lipgloss.NewStyle().
			Foreground(theme.Text).
			Bold(true),
		ErrorText: lipgloss.NewStyle().
			Foreground(theme.Danger),
		Divider: lipgloss.NewStyle().
			Foreground(theme.Border),
		Logo: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),
		LogoAccent: lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Bold(true),
	}
}

func noColorTheme() Theme {
	n := lipgloss.NoColor{}
	return Theme{
		Primary: n, Secondary: n, Success: n, Warning: n, Danger: n,
		Muted: n, Text: n, Subtle: n, Border: n, Surface: n,
		SelectedBg: n, SelectedFg: n, OnPrimary: n,
	}
}

func defaultTheme() Theme {
	return Theme{
		Primary:    lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A855F7"},
		Secondary:  lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"},
		Success:    lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#22C55E"},
		Warning:    lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#F59E0B"},
		Danger:     lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#EF4444"},
		Muted:      lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"},
		Text:       lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"},
		Subtle:     lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#D1D5DB"},
		Border:     lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"},
		Surface:    lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"},
		SelectedBg: lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#4C1D95"},
		SelectedFg: lipgloss.AdaptiveColor{Light: "#4C1D95", Dark: "#F5F3FF"},
		OnPrimary:  lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111827"},
	}
}

// DetectNoColor reports whether the environment asks for plain output. It is
// shared by the CLI entry point and the command environment so both honour
// NO_COLOR and TERM=dumb consistently.
func DetectNoColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return strings.EqualFold(os.Getenv("TERM"), "dumb")
}

// StatusColor returns the colour token for a task status.
func (s Styles) StatusColor(status api.TaskStatus) lipgloss.TerminalColor {
	switch status {
	case api.TaskStatusTodo:
		return s.Theme.Muted
	case api.TaskStatusInProgress:
		return s.Theme.Secondary
	case api.TaskStatusReview:
		return s.Theme.Warning
	case api.TaskStatusDone:
		return s.Theme.Success
	default:
		return s.Theme.Text
	}
}

// StatusStyle returns a text style for a task status.
func (s Styles) StatusStyle(status api.TaskStatus) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.StatusColor(status)).Bold(true)
}

// StatusBadge renders a compact status indicator.
func (s Styles) StatusBadge(status api.TaskStatus) string {
	return lipgloss.NewStyle().
		Foreground(s.StatusColor(status)).
		Background(s.Theme.Surface).
		Padding(0, 1).
		Bold(true).
		Render(status.Label())
}

// PriorityColor returns the colour token for a priority.
func (s Styles) PriorityColor(priority api.Priority) lipgloss.TerminalColor {
	switch priority {
	case api.PriorityLow:
		return s.Theme.Muted
	case api.PriorityMedium:
		return s.Theme.Secondary
	case api.PriorityHigh:
		return s.Theme.Warning
	case api.PriorityUrgent:
		return s.Theme.Danger
	default:
		return s.Theme.Text
	}
}

// PriorityStyle returns a text style for a priority.
func (s Styles) PriorityStyle(priority api.Priority) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.PriorityColor(priority)).Bold(true)
}

// TypeStyle returns a text style for a task type.
func (s Styles) TypeStyle(taskType api.TaskType) lipgloss.Style {
	switch taskType {
	case api.TaskTypeBug:
		return lipgloss.NewStyle().Foreground(s.Theme.Danger)
	case api.TaskTypeStory:
		return lipgloss.NewStyle().Foreground(s.Theme.Success)
	case api.TaskTypeEpic:
		return lipgloss.NewStyle().Foreground(s.Theme.Primary)
	default:
		return lipgloss.NewStyle().Foreground(s.Theme.Subtle)
	}
}

// SecondaryStyle returns a style using the secondary accent colour.
func (s Styles) SecondaryStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.Theme.Secondary)
}

// DividerString renders a horizontal rule of the requested width.
func (s Styles) DividerString(width int) string {
	if width <= 0 {
		return ""
	}
	return s.Divider.Render(strings.Repeat("─", width))
}

// KeyHint renders a key and its description for footers and help panels.
func (s Styles) KeyHint(key, description string) string {
	return s.Key.Render(key) + " " + s.Help.Render(description)
}
