package main

import (
	"fmt"
	"os"

	"github.com/jackchuka/dutix/internal/cli"
	"github.com/jackchuka/dutix/internal/logger"
)

func main() {
	// Ensure logger is always closed, even on errors
	defer logger.Close()

	rootCmd := cli.NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
