package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/saurlax/autodl-cli/internal/cli"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	os.Exit(cli.Execute(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version))
}
