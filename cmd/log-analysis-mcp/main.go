package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/trickyearlobe/log-analysis-mcp/internal/server"
)

var version = "0.1.0"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := server.New(version)
	slog.Info("starting log-analysis-mcp", "version", version)
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "log-analysis-mcp: %v\n", err)
		os.Exit(1)
	}
}
