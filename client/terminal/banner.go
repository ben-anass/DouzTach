package terminal

import "fmt"

func PrintBanner() {
	banner := `
 ____                  _____           _
|  _ \  ___  _   _ ___|_   _|_ _  ___| |__
| | | |/ _ \| | | |_  / | |/ _` + "`" + ` |/ __| '_ \
| |_| | (_) | |_| |/ /  | | (_| | (__| | | |
|____/ \___/ \__, /___| |_|\__,_|\___|_| |_|
             |___/
`

	fmt.Println(Primary.Render(banner))
	fmt.Println(Secondary.Render("          PROJECT MANAGEMENT CLI"))
	fmt.Println("                 " + Success.Render("v0.2.0"))
	fmt.Println()
	fmt.Println("  Welcome to DouzTach!")
	fmt.Println("  Manage your work. Build together.")
	fmt.Println()
}
