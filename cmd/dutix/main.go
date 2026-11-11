package main

import (
	"os"

	"github.com/jackchuka/dutix/internal/cli"
	"github.com/jackchuka/dutix/internal/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	defer logger.Close()
	return cli.Run(os.Args[1:], os.Stdout, os.Stderr)
}
