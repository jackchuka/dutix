package main

import (
	"fmt"
	"os"

	"github.com/jackchuka/dutix/internal/cli"
	"github.com/jackchuka/dutix/internal/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	defer logger.Close()

	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return cli.ExitCode(err)
	}
	return 0
}
