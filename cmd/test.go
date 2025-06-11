package cmd

import (
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run various test utilities",
	Long:  `Run test utilities to verify Waymon components are working correctly.`,
}

func init() {
	// Add subcommands
	testCmd.AddCommand(testInputCmd)
	testCmd.AddCommand(testDisplayCmd)
}

var testInputCmd = &cobra.Command{
	Use:   "input",
	Short: "Test uinput functionality",
	Long:  `Test uinput functionality by drawing circles with the mouse.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// This just runs the test-visual command
		return runTestVisual()
	},
}

var testDisplayCmd = &cobra.Command{
	Use:   "display",
	Short: "Test display detection",
	Long:  `Test display detection and show monitor configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// This just runs the test-display command
		return runTestDisplay()
	},
}

func runTestVisual() error {
	// Implementation moved from cmd/test-visual/main.go
	return testVisualMain()
}

func runTestDisplay() error {
	// Implementation moved from cmd/test-display/main.go
	return testDisplayMain()
}