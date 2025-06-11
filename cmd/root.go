package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set during build
	Version = "0.1.0-dev"

	rootCmd = &cobra.Command{
		Use:   "waymon",
		Short: "Waymon - Wayland mouse sharing",
		Long: `Waymon is a client/server mouse sharing application for Wayland systems.
It allows seamless mouse movement between two computers on a local network,
working around Wayland's security restrictions by using the uinput kernel module.`,
		SilenceUsage: true,
	}
)

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s\n" .Version}}`)

	// Add commands
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(clientCmd)
	rootCmd.AddCommand(testCmd)
}

// Exit with error message
func exitError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}