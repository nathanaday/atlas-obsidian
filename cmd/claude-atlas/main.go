package main

import (
	"os"

	"github.com/nathanaday/claude-atlas/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
