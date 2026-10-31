package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Clear terminal screen",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("\033[H\033[2J")
		},
	}
}
