package commands

import (
	"fmt"

	"github.com/ben-anass/DouzTach/client/terminal"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(terminal.Secondary.Render("DouzTach v0.2.0"))
		},
	}
}
