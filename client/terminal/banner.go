package terminal

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	purple = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A855F7")).
		Bold(true)

	cyan = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22D3EE")).
		Bold(true)

	green = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E"))
)

func PrintBanner() {
	banner := `
 ____                  _____           _
|  _ \  ___  _   _ ___|_   _|_ _  ___| |__
| | | |/ _ \| | | |_  / | |/ _` + "`" + ` |/ __| '_ \
| |_| | (_) | |_| |/ /  | | (_| | (__| | | |
|____/ \___/ \__, /___| |_|\__,_|\___|_| |_|
             |___/
`

	fmt.Println(purple.Render(banner))
	fmt.Println(cyan.Render("          PROJECT MANAGEMENT CLI"))
	fmt.Println("                 " + green.Render("v0.1.0"))
	fmt.Println()
	fmt.Println("  Welcome to DouzTach!")
	fmt.Println("  Manage your work. Build together.")
	fmt.Println()
}
