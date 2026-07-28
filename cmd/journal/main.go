// Command journal is daily-journal: a minimal journaling CLI.
//
//	journal                       open today's entry
//	journal "a thought"           quick capture
//	journal help                  everything else
//
// Exit status follows the POSIX convention: 0 on success, 2 when the command
// line is wrong, 1 for anything that failed while running.
package main

import (
	"errors"
	"os"

	"daily-journal/internal/cli"
	"daily-journal/internal/config"
	"daily-journal/internal/ui"
)

// Exit statuses.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

// Build information, injected at link time by GoReleaser:
//
//	-X main.version=1.2.3 -X main.commit=abc123 -X main.date=2026-07-28T...
//
// The defaults are what a `go build` from a working tree reports.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	app, err := cli.NewApp(config.OSEnv())
	if err != nil {
		ui.NewPrinter().Error(err)
		os.Exit(exitFailure)
	}
	app.Build = cli.BuildInfo{Version: version, Commit: commit, Date: date}
	if err := app.Run(os.Args[1:]); err != nil {
		app.Printer.Error(err)
		var usageErr *cli.UsageError
		if errors.As(err, &usageErr) {
			os.Exit(exitUsage)
		}
		os.Exit(exitFailure)
	}
	os.Exit(exitOK)
}
