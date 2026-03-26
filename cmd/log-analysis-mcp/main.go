package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/trickyearlobe/log-analysis-mcp/internal/install"
	"github.com/trickyearlobe/log-analysis-mcp/internal/remote"
	"github.com/trickyearlobe/log-analysis-mcp/internal/server"
	"github.com/trickyearlobe/log-analysis-mcp/internal/tools"
)

var version = "0.1.0"

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	doInstall := flag.Bool("install", false, "Register as MCP server in supported IDEs and exit")
	doUninstall := flag.Bool("uninstall", false, "Remove from supported IDE MCP configs and exit")
	flag.Parse()

	if *doInstall && *doUninstall {
		fmt.Fprintf(os.Stderr, "log-analysis-mcp: cannot use --install and --uninstall together\n")
		os.Exit(1)
	}

	if *doInstall {
		fmt.Fprintf(os.Stderr, "Installing log-analysis-mcp into IDE configs...\n")
		results := install.Install()
		install.PrintResults(results)
		os.Exit(exitCodeFromResults(results))
	}

	if *doUninstall {
		fmt.Fprintf(os.Stderr, "Removing log-analysis-mcp from IDE configs...\n")
		results := install.Uninstall()
		install.PrintResults(results)
		os.Exit(exitCodeFromResults(results))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	defer tools.CleanupTempFiles()
	defer remote.CloseDefaultPool()

	srv := server.New(version)
	slog.Info("starting log-analysis-mcp", "version", version)
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "log-analysis-mcp: %v\n", err)
		os.Exit(1)
	}
}

// exitCodeFromResults returns 1 if any result has an error, 0 otherwise.
func exitCodeFromResults(results []install.Result) int {
	for _, r := range results {
		if r.Error != nil {
			return 1
		}
	}
	return 0
}
