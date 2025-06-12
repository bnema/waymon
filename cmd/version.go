package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version info set by main package
	Version string
	Commit  string
	Date    string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("waymon %s\n", Version)
		fmt.Printf("commit: %s\n", Commit)
		fmt.Printf("built: %s\n", Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
