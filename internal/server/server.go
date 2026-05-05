// Package server provides the MCP server configuration and lifecycle management.
package server

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe/log-analysis-mcp/internal/metrics"
	"github.com/trickyearlobe/log-analysis-mcp/internal/prompts"
	"github.com/trickyearlobe/log-analysis-mcp/internal/resources"
	"github.com/trickyearlobe/log-analysis-mcp/internal/tools"
)

// Server wraps the MCP SDK server with log-analysis-mcp configuration.
type Server struct {
	srv     *mcp.Server
	metrics *metrics.Writer
}

// MetricsDir returns the default metrics storage directory.
func MetricsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".log-analysis-mcp", "metrics")
}

// New creates a configured MCP server with all tools, resources, and prompts registered.
func New(version string) *Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "log-analysis-mcp",
		Version: version,
	}, nil)

	metricsDir := MetricsDir()
	mw, err := metrics.NewWriter(metricsDir)
	if err != nil {
		slog.Error("metrics writer init failed, continuing without metrics", "error", err)
	}

	if mw != nil {
		srv.AddReceivingMiddleware(metrics.NewMiddleware(mw))
	}

	tools.Register(srv, metricsDir)
	resources.Register(srv)
	prompts.Register(srv)

	return &Server{srv: srv, metrics: mw}
}

// Run starts the MCP server on the stdio transport, blocking until the context
// is cancelled or the transport closes.
func (s *Server) Run(ctx context.Context) error {
	defer s.shutdown()
	return s.srv.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) shutdown() {
	if s.metrics != nil {
		if err := s.metrics.Close(); err != nil {
			slog.Error("metrics flush failed", "error", err)
		}
		slog.Info("metrics flushed")
	}
}
