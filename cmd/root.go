// Package cmd provides the composition root for waymon CLI.
// It creates dependencies and wires them into the CLI adapter.
package cmd

import (
	"context"
	"os"

	"github.com/bnema/waymon/internal/adapters/in/cli"
	"github.com/bnema/waymon/internal/adapters/in/ipc"
	"github.com/bnema/waymon/internal/adapters/out/config"
	"github.com/bnema/waymon/internal/adapters/out/display"
	"github.com/rs/zerolog"
)

var (
	// Version is set at build time via ldflags.
	Version = "dev"

	// Commit is the git commit hash set at build time.
	Commit = "unknown"

	// Date is the build date set at build time.
	Date = "unknown"
)

// Execute runs the waymon CLI.
// This is the composition root that creates all dependencies and wires them together.
func Execute() error {
	// Create context with logger
	ctx := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).
		With().
		Timestamp().
		Logger().
		WithContext(context.Background())

	// Set version
	cli.SetVersion(Version)

	// Create dependencies
	var ipcClient cli.IPCClient
	ipcClientRaw, err := ipc.NewClient()
	if err == nil {
		ipcClient = cli.NewIPCClientAdapter(ipcClientRaw)
	}
	// If IPC client fails, that's ok - it means server isn't running
	// Commands that need it will handle the nil case

	// Create config repository
	configRepo := config.NewViperRepository()

	// Create display port (may fail on some systems)
	displayPort, _ := display.New(ctx)
	// If display detection fails, that's ok - monitors command will handle it

	// Create CLI with dependencies
	cliInstance := cli.New(
		cli.WithIPCClient(ipcClient),
		cli.WithConfigRepository(configRepo),
		cli.WithDisplayPort(displayPort),
	)

	return cliInstance.ExecuteContext(ctx)
}
