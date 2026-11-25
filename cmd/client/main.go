// Command client is the DouzTach terminal client.
//
// Running it without a subcommand opens the interactive terminal UI; every
// dashboard action is also available as a scriptable subcommand.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/ben-anass/DouzTach/client/commands"
	"github.com/ben-anass/DouzTach/client/terminal/styles"
)

// version is the client version. Release builds override it with
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd/client
var version = "0.1.0"

func main() {
	// Ctrl+C interrupts a running request or prompt instead of killing the
	// process, so the terminal is always restored cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := commands.Execute(ctx, version); err != nil {
		commands.ReportError(os.Stderr, styles.New(styles.DetectNoColor()), err)
		os.Exit(commands.ExitCode(err))
	}
}
