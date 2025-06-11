package main

import (
	"fmt"
	"os"

	"github.com/bnema/waymon/cmd"
)

const version = "0.1.0-dev"

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}