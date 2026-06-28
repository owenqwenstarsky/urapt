// Package main is the urapt-server entrypoint.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"urapt/server/app"
	"urapt/shared/version"
)

func main() {
	a := app.New(os.Args[1:], version.Version)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := a.Run(ctx); err != nil {
		os.Exit(1)
	}
}
