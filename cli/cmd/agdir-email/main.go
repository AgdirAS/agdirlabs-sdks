// Command agdir-email is the official command-line interface for Agdir Email
// (https://agdir.email): send email, set up sending domains, check their DNS and
// manage webhooks.
//
//	go install github.com/AgdirAS/agdirlabs-sdks/cli/cmd/agdir-email@latest
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/AgdirAS/agdirlabs-sdks/cli/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := app.New().RunContext(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}
