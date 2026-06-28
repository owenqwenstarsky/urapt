// Package main is the urapt CLI entrypoint.
package main

import (
	"os"

	"urapt/cli/commands"
	"urapt/shared/version"
)

func main() {
	if err := commands.New(version.Version).Execute(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
