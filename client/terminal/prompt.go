package terminal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func RunPrompt(execute func(string) error) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println(Muted.Render("  Type 'help' to see available commands."))
	fmt.Println()

	for {
		fmt.Print(Prompt.Render("douztach") + Muted.Render("> "))

		if !scanner.Scan() {
			fmt.Println()
			return
		}

		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		if input == "exit" {
			fmt.Println(Success.Render("Goodbye! See you soon."))
			return
		}

		if err := execute(input); err != nil {
			fmt.Println(Error.Render("Error: " + err.Error()))
		}

		fmt.Println()
	}

}
