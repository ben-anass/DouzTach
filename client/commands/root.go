package commands

import (
	"fmt"
	"strings"

	"github.com/ben-anass/DouzTach/client/terminal"
	"github.com/spf13/cobra"
)

func newRootCommand(interactive bool) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "douztach",
		Short:         "DouzTach - Project Management CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newClearCommand())

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !interactive {
			return cmd.Help()
		}

		terminal.PrintBanner()

		terminal.RunPrompt(func(input string) error {
			if input == "help" {
				return cmd.Help()
			}

			parts := strings.Fields(input)

			if len(parts) == 0 {
				return nil
			}

			command := newRootCommand(false)
			command.SetArgs(parts)

			return command.Execute()
		})

		return nil
	}

	return rootCmd
}

func Execute() {
	if err := newRootCommand(true).Execute(); err != nil {
		fmt.Println(terminal.Error.Render(err.Error()))
	}
}
