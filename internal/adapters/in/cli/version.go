package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (c *CLI) newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Long:  `Print the version number of waymon.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("waymon version %s\n", Version)
		},
	}
}
