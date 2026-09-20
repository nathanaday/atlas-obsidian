package main

import (
	"os"

	"github.com/nathanaday/atlas-obsidian/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
