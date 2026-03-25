package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var version = "0.1.0"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// TODO: instantiate server and run on stdio transport
	_ = ctx

	fmt.Fprintf(os.Stderr, "log-analysis-mcp %s: server not yet implemented\n", version)
	os.Exit(0)
}
