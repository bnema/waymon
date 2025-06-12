package main

import (
	"os"

	"github.com/bnema/waymon/cmd"
)

// Build variables set by goreleaser
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Set version info
	cmd.Version = version
	cmd.Commit = commit
	cmd.Date = date

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
