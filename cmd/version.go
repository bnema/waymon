package cmd

import (
	"github.com/bnema/waymon/internal/logger"
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
		logger.Infof("waymon %s", Version)
		logger.Infof("commit: %s", Commit)
		logger.Infof("built: %s", Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
