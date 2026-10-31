package terminal

import "github.com/charmbracelet/lipgloss"

var (
	Primary = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A855F7")).
		Bold(true)

	Secondary = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22D3EE")).
			Bold(true)

	Success = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E")).
		Bold(true)

	Error = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	Muted = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#71717A"))

	Prompt = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A855F7")).
		Bold(true)
)
